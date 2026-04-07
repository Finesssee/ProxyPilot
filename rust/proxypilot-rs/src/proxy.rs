use std::net::SocketAddr;
use std::path::Path;
use std::sync::Arc;

use anyhow::{Context, Result};
use axum::body::{Body, Bytes};
use axum::extract::{OriginalUri, State};
use axum::http::{HeaderMap, Method, Response, StatusCode};
use axum::response::IntoResponse;
use axum::routing::{get, post};
use axum::{Json, Router};
use reqwest::Client;
use serde_json::json;
use tokio::sync::RwLock;
use tracing::info;

use crate::auth_runtime::{
    self, AuthCredentialSource, RefreshStatusSnapshot, RuntimeRequestCounters, RuntimeStatsSnapshot,
};
use crate::config::AppConfig;
use crate::provider_registry::{ProviderRegistry, resolve_active_provider};
use crate::state::AccountState;

#[derive(Clone)]
pub struct AppState {
    config: Arc<AppConfig>,
    accounts: Arc<RwLock<AccountState>>,
    runtime: Arc<RwLock<RuntimeTelemetry>>,
    client: Client,
    providers: Arc<ProviderRegistry>,
}

#[derive(Debug, Clone, Default)]
struct RuntimeTelemetry {
    active_account_name: Option<String>,
    account_count: usize,
    request_counters: RuntimeRequestCounters,
    last_refresh: RefreshStatusSnapshot,
}

impl AppState {
    pub fn new(config: AppConfig, accounts: AccountState) -> Self {
        let runtime = runtime_telemetry_from_state(&config, &accounts);
        let providers = Arc::new(ProviderRegistry::new(&config));
        Self {
            config: Arc::new(config),
            accounts: Arc::new(RwLock::new(accounts)),
            runtime: Arc::new(RwLock::new(runtime)),
            client: Client::new(),
            providers,
        }
    }

    async fn runtime_stats_snapshot(&self) -> RuntimeStatsSnapshot {
        let runtime = self.runtime.read().await;
        let accounts = self.accounts.read().await;
        let auth_health = runtime_auth_health_from_state(&self.config, &accounts);
        let provider = self.providers.active_provider(&self.config);
        let mut snapshot = RuntimeStatsSnapshot::new(
            self.config.server.bind.clone(),
            provider.upstream_base_url().to_string(),
            runtime.active_account_name.clone(),
            runtime.account_count,
            auth_health,
        );
        snapshot.request_counters = runtime.request_counters.clone();
        snapshot.last_refresh = runtime.last_refresh.clone();
        snapshot
    }

    async fn record_supported_request(&self) {
        let mut runtime = self.runtime.write().await;
        runtime.request_counters.total_proxied_requests += 1;
    }

    async fn record_successful_upstream_response(&self) {
        let mut runtime = self.runtime.write().await;
        runtime.request_counters.successful_upstream_responses += 1;
    }

    async fn record_upstream_401(&self) {
        let mut runtime = self.runtime.write().await;
        runtime.request_counters.upstream_401_count += 1;
    }

    async fn record_refresh_attempt(&self) {
        let mut runtime = self.runtime.write().await;
        runtime.request_counters.auth_refresh_attempts += 1;
    }

    async fn record_refresh_failure(&self, account_name: Option<String>, message: String) {
        let mut runtime = self.runtime.write().await;
        runtime.request_counters.auth_refresh_failures += 1;
        runtime.last_refresh =
            RefreshStatusSnapshot::failure(account_name, auth_runtime::now_unix_secs(), message);
    }

    async fn record_refresh_success(&self, account_name: String) {
        let mut runtime = self.runtime.write().await;
        runtime.last_refresh =
            RefreshStatusSnapshot::success(account_name, auth_runtime::now_unix_secs());
    }
}

fn runtime_telemetry_from_state(config: &AppConfig, accounts: &AccountState) -> RuntimeTelemetry {
    let provider = config.active_provider();
    RuntimeTelemetry {
        active_account_name: accounts.active_account_for_provider(provider).map(|account| account.name),
        account_count: accounts.runtime_usable_account_count_for_provider(provider),
        request_counters: RuntimeRequestCounters::default(),
        last_refresh: RefreshStatusSnapshot::default(),
    }
}

fn runtime_auth_health_from_state(
    config: &AppConfig,
    accounts: &AccountState,
) -> auth_runtime::AuthHealthSnapshot {
    let provider = config.active_provider();
    let active_account = accounts.active_account_for_provider(provider);
    let source = if active_account.is_some() {
        AuthCredentialSource::ActiveAccount
    } else if provider == crate::provider::CODEX_PROVIDER && config.codex.api_key.trim().is_empty() {
        AuthCredentialSource::NoCredential
    } else if provider == crate::provider::CODEX_PROVIDER {
        AuthCredentialSource::ConfigFallbackKey
    } else {
        AuthCredentialSource::NoCredential
    };
    let refresh_token = active_account
        .as_ref()
        .and_then(|account| account.refresh_token.as_deref());
    let expires_at = active_account
        .as_ref()
        .and_then(|account| account.expires_at.as_deref());

    auth_runtime::evaluate_auth_health(
        source,
        refresh_token,
        expires_at,
        auth_runtime::now_unix_secs(),
    )
}

pub fn build_app(config: AppConfig, accounts: AccountState) -> Router {
    let state = AppState::new(config, accounts);

    Router::new()
        .route("/healthz", get(healthz))
        .route("/v0/runtime/stats", get(runtime_stats))
        .route("/v1/models", get(list_models))
        .route("/v1/chat/completions", post(chat_completions))
        .route("/v1/responses", post(responses))
        .route("/v1/{*path}", get(unsupported_v1).post(unsupported_v1))
        .with_state(state)
}

pub async fn run(config: AppConfig, config_path: &Path) -> Result<()> {
    let bind_addr: SocketAddr = config
        .server
        .bind
        .parse()
        .with_context(|| format!("invalid bind address {}", config.server.bind))?;

    let state_path = config.resolve_state_path(config_path);
    let accounts = AccountState::load_or_default(&state_path)?;
    let app = build_app(config.clone(), accounts);
    let listener = tokio::net::TcpListener::bind(bind_addr)
        .await
        .with_context(|| format!("failed to bind {}", bind_addr))?;
    info!(
        bind = %config.server.bind,
        upstream = %ProviderRegistry::new(&config).active_provider(&config).upstream_base_url(),
        provider = %config.active_provider(),
        "proxypilot-rs listening"
    );
    axum::serve(listener, app)
        .await
        .context("proxy server exited unexpectedly")?;
    Ok(())
}

async fn healthz(State(state): State<AppState>) -> impl IntoResponse {
    let provider = state.providers.active_provider(&state.config);
    let account_state = state.accounts.read().await;
    let active_account = account_state
        .active_account_for_provider(provider.provider_tag())
        .map(|account| account.name);
    Json(json!({
        "status": "ok",
        "provider": provider.provider_tag(),
        "listen": state.config.server.bind,
        "upstream": provider.upstream_base_url(),
        "active_account": active_account,
    }))
}

async fn runtime_stats(State(state): State<AppState>) -> impl IntoResponse {
    Json(state.runtime_stats_snapshot().await)
}

async fn list_models(
    State(state): State<AppState>,
    original_uri: OriginalUri,
    headers: HeaderMap,
) -> Result<Response<Body>, ProxyError> {
    forward_upstream(
        &state,
        Method::GET,
build_upstream_url(state.providers.active_provider(&state.config).upstream_base_url(), &original_uri)?,
        headers,
        Bytes::new(),
    )
    .await
}

async fn chat_completions(
    State(state): State<AppState>,
    original_uri: OriginalUri,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response<Body>, ProxyError> {
    forward_upstream(
        &state,
        Method::POST,
build_upstream_url(state.providers.active_provider(&state.config).upstream_base_url(), &original_uri)?,
        headers,
        body,
    )
    .await
}

async fn responses(
    State(state): State<AppState>,
    original_uri: OriginalUri,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response<Body>, ProxyError> {
    forward_upstream(
        &state,
        Method::POST,
build_upstream_url(state.providers.active_provider(&state.config).upstream_base_url(), &original_uri)?,
        headers,
        body,
    )
    .await
}

async fn unsupported_v1(original_uri: OriginalUri) -> impl IntoResponse {
    (
        StatusCode::NOT_FOUND,
        Json(json!({
            "error": format!(
                "unsupported Rust replatform route: {}",
                original_uri.0.path()
            ),
            "supported_routes": [
                "/v1/models",
                "/v1/chat/completions",
                "/v1/responses"
            ]
        })),
    )
}

async fn forward_upstream(
    state: &AppState,
    method: Method,
    target: String,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response<Body>, ProxyError> {
    state.record_supported_request().await;

    if active_account_needs_refresh(state).await {
        let _ = try_refresh_active_account(state).await?;
    }

    let token = resolve_active_provider(&state.config, &state.providers, &state.accounts).await.api_key;
    let upstream = send_upstream_request(state, &method, &target, &headers, &body, token)
        .await
        .context("failed to reach upstream provider endpoint")?;

    let upstream = if upstream.status() == StatusCode::UNAUTHORIZED {
        state.record_upstream_401().await;
        if let Some(refreshed_token) = try_refresh_active_account(state).await? {
            send_upstream_request(
                state,
                &method,
                &target,
                &headers,
                &body,
                Some(refreshed_token),
            )
            .await
            .context("failed to retry upstream provider endpoint after refresh")?
        } else {
            upstream
        }
    } else {
        upstream
    };

    let status = upstream.status();
    if status.is_success() {
        state.record_successful_upstream_response().await;
    }
    let response_headers = upstream.headers().clone();
    let response_body = upstream
        .bytes()
        .await
        .context("failed to read upstream response body")?;

    let mut response = Response::builder().status(status);
    for (name, value) in &response_headers {
        if should_skip_response_header(name.as_str()) {
            continue;
        }
        response = response.header(name, value);
    }

    response
        .body(Body::from(response_body))
        .context("failed to build downstream response")
        .map_err(ProxyError::from)
}

async fn send_upstream_request(
    state: &AppState,
    method: &Method,
    target: &str,
    headers: &HeaderMap,
    body: &Bytes,
    token: Option<String>,
) -> Result<reqwest::Response> {
    let mut request = state.client.request(method.clone(), target);

    for (name, value) in headers {
        if should_skip_request_header(name.as_str()) {
            continue;
        }
        request = request.header(name, value);
    }

    if let Some(api_key) = token
        && !api_key.trim().is_empty()
    {
        request = request.bearer_auth(api_key);
    }

    request
        .body(body.clone())
        .send()
        .await
        .context("upstream request failed")
}

async fn try_refresh_active_account(state: &AppState) -> Result<Option<String>, ProxyError> {
    let resolved = resolve_active_provider(&state.config, &state.providers, &state.accounts).await;

    let Some(account_name) = resolved.account_name.clone() else {
        return Ok(None);
    };
    let Some(refresh_token) = resolved.refresh_token.as_deref() else {
        return Ok(None);
    };

    state.record_refresh_attempt().await;

    let refreshed = match resolved.provider.refresh_token(refresh_token).await {
        Ok(value) => value,
        Err(err) => {
            state
                .record_refresh_failure(Some(account_name.clone()), err.to_string())
                .await;
            return Err(ProxyError::from(err));
        }
    };

    {
        let mut accounts = state.accounts.write().await;
        if let Err(err) = accounts.update_account_tokens_for_provider(
            &account_name,
            resolved.provider.provider_tag(),
            refreshed.access_token,
            refreshed.refresh_token,
            refreshed.id_token,
            refreshed.email,
            refreshed.account_id,
            refreshed.plan_type,
            refreshed.expires_at,
        ) {
            state
                .record_refresh_failure(Some(account_name.clone()), err.to_string())
                .await;
            return Err(ProxyError::from(err));
        }
    }

    state.record_refresh_success(account_name.clone()).await;
    let refreshed_token = {
        let accounts = state.accounts.read().await;
        accounts
            .active_account_for_provider(resolved.provider.provider_tag())
            .map(|account| account.api_key)
    };

    Ok(refreshed_token)
}

async fn active_account_needs_refresh(state: &AppState) -> bool {
    let provider = state.config.active_provider().to_string();
    let active = {
        let accounts = state.accounts.read().await;
        accounts.active_account_for_provider(&provider)
    };

    let Some(active) = active else {
        return false;
    };
    let auth_health = auth_runtime::evaluate_auth_health(
        AuthCredentialSource::ActiveAccount,
        active.refresh_token.as_deref(),
        active.expires_at.as_deref(),
        auth_runtime::now_unix_secs(),
    );

    auth_health.state.is_refresh_worthy()
}

fn build_upstream_url(base_url: &str, original_uri: &OriginalUri) -> Result<String, ProxyError> {
    let cleaned_base = base_url.trim_end_matches('/');
    let suffix = original_uri
        .0
        .path_and_query()
        .map(|value| value.as_str())
        .unwrap_or("/v1");

    if !suffix.starts_with("/v1") {
        return Err(ProxyError::bad_request("only /v1/* proxying is supported"));
    }

    Ok(format!("{cleaned_base}{suffix}"))
}

fn should_skip_request_header(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "host" | "content-length" | "authorization" | "connection"
    )
}

fn should_skip_response_header(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "content-length" | "transfer-encoding" | "connection"
    )
}

#[derive(Debug)]
pub struct ProxyError {
    status: StatusCode,
    message: String,
}

impl ProxyError {
    fn bad_request(message: impl Into<String>) -> Self {
        Self {
            status: StatusCode::BAD_REQUEST,
            message: message.into(),
        }
    }
}

impl From<anyhow::Error> for ProxyError {
    fn from(value: anyhow::Error) -> Self {
        Self {
            status: StatusCode::BAD_GATEWAY,
            message: value.to_string(),
        }
    }
}

impl IntoResponse for ProxyError {
    fn into_response(self) -> Response<Body> {
        let payload = Json(json!({
            "error": self.message,
        }));
        (self.status, payload).into_response()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::auth_runtime::parse_rfc3339_z;
    use axum::routing::{get, post};
    use base64::Engine;
    use serde_json::Value;
    use tokio::sync::oneshot;

    async fn start_listener(app: Router) -> (String, oneshot::Sender<()>) {
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let (tx, rx) = oneshot::channel::<()>();
        tokio::spawn(async move {
            let server = axum::serve(listener, app).with_graceful_shutdown(async {
                let _ = rx.await;
            });
            let _ = server.await;
        });
        (format!("http://{}", addr), tx)
    }

    fn build_test_state_with_endpoints(
        config: AppConfig,
        accounts: AccountState,
        oauth_token_url: String,
    ) -> AppState {
        let runtime = runtime_telemetry_from_state(&config, &accounts);
        let mut provider_config = config.clone();
        provider_config.codex.refresh_token_url = oauth_token_url;
        AppState {
            config: std::sync::Arc::new(config),
            accounts: std::sync::Arc::new(tokio::sync::RwLock::new(accounts)),
            runtime: std::sync::Arc::new(tokio::sync::RwLock::new(runtime)),
            client: Client::new(),
            providers: std::sync::Arc::new(ProviderRegistry::new(&provider_config)),
        }
    }

    #[tokio::test]
    async fn proxies_chat_completions_and_injects_bearer_token() {
        async fn upstream_handler(
            headers: HeaderMap,
            Json(body_json): Json<Value>,
        ) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();
            Json(json!({
                "received_authorization": auth,
                "echo_model": body_json["model"],
                "ok": true
            }))
        }

        let upstream_app = Router::new().route("/v1/chat/completions", post(upstream_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "test-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let proxy_app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let client = Client::new();
        let response = client
            .post(format!("{proxy_url}/v1/chat/completions"))
            .header("authorization", "Bearer wrong-token")
            .json(&json!({
                "model": "gpt-5.2-codex",
                "input": "ping"
            }))
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);
        let payload: Value = response.json().await.unwrap();
        assert_eq!(payload["received_authorization"], "Bearer test-token");
        assert_eq!(payload["echo_model"], "gpt-5.2-codex");

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn proxies_responses_endpoint() {
        async fn upstream_handler(
            headers: HeaderMap,
            Json(body_json): Json<Value>,
        ) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();
            Json(json!({
                "received_authorization": auth,
                "echo_input": body_json["input"],
                "object": "response"
            }))
        }

        let upstream_app = Router::new().route("/v1/responses", post(upstream_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "test-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let proxy_app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let client = Client::new();
        let response = client
            .post(format!("{proxy_url}/v1/responses"))
            .json(&json!({
                "model": "gpt-5.2-codex",
                "input": "ship it"
            }))
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);
        let payload: Value = response.json().await.unwrap();
        assert_eq!(payload["received_authorization"], "Bearer test-token");
        assert_eq!(payload["echo_input"], "ship it");

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn proxies_models_endpoint() {
        async fn upstream_models() -> impl IntoResponse {
            Json(json!({
                "object": "list",
                "data": [
                    {"id": "gpt-5.2-codex", "object": "model"},
                    {"id": "gpt-5.4-mini", "object": "model"}
                ]
            }))
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_models));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "test-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let proxy_app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let client = Client::new();
        let response = client
            .get(format!("{proxy_url}/v1/models"))
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);
        let payload: Value = response.json().await.unwrap();
        assert_eq!(payload["data"][0]["id"], "gpt-5.2-codex");
        assert_eq!(payload["data"][1]["id"], "gpt-5.4-mini");

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn unsupported_v1_routes_return_clear_error() {
        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: "https://api.openai.com".to_string(),
                api_key: "test-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let proxy_app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let client = Client::new();
        let response = client
            .post(format!("{proxy_url}/v1/files"))
            .json(&json!({}))
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::NOT_FOUND);
        let payload: Value = response.json().await.unwrap();
        assert!(
            payload["error"]
                .as_str()
                .unwrap_or_default()
                .contains("/v1/files")
        );

        let _ = proxy_shutdown.send(());
    }

    #[tokio::test]
    async fn active_state_account_beats_config_fallback() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();
            Json(json!({
                "received_authorization": auth
            }))
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url,
                api_key: "fallback-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_or_replace_codex_account("primary".to_string(), "state-token".to_string(), true)
            .unwrap();

        let proxy_app = build_app(config, accounts);
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let payload: Value = Client::new()
            .get(format!("{proxy_url}/v1/models"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();

        assert_eq!(payload["received_authorization"], "Bearer state-token");

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn refreshes_active_account_after_401_and_retries() {
        async fn upstream_handler(
            headers: HeaderMap,
            State(seen): State<std::sync::Arc<std::sync::atomic::AtomicBool>>,
        ) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            if auth == "Bearer stale-token" && !seen.swap(true, std::sync::atomic::Ordering::SeqCst)
            {
                return (
                    StatusCode::UNAUTHORIZED,
                    Json(json!({"error": "expired token"})),
                )
                    .into_response();
            }

            Json(json!({
                "received_authorization": auth
            }))
            .into_response()
        }

        async fn refresh_handler() -> impl IntoResponse {
            Json(json!({
                "access_token": "fresh-token",
                "refresh_token": "fresh-refresh-token",
                "id_token": format!("header.{}.sig", encoded_claims_placeholder()),
                "expires_in": 3600
            }))
        }

        fn encoded_claims_placeholder() -> String {
            let payload = serde_json::json!({
                "email": "refresh@example.com",
                "exp": 1893456000_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": "pro"
                }
            });
            base64::engine::general_purpose::URL_SAFE_NO_PAD
                .encode(serde_json::to_vec(&payload).unwrap())
        }

        let seen = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
        let upstream_app = Router::new()
            .route("/v1/models", get(upstream_handler))
            .with_state(seen);
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url,
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "stale-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("2030-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state =
            build_test_state_with_endpoints(config, accounts, format!("{refresh_url}/oauth/token"));

        let response = forward_upstream(
            &state,
            Method::GET,
            format!("{}/v1/models", state.config.codex.upstream_base_url),
            HeaderMap::new(),
            Bytes::new(),
        )
        .await
        .unwrap();
        let body = axum::body::to_bytes(response.into_body(), usize::MAX)
            .await
            .unwrap();
        let payload: Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(payload["received_authorization"], "Bearer fresh-token");

        let snapshot = state.runtime_stats_snapshot().await;
        assert_eq!(snapshot.request_counters.total_proxied_requests, 1);
        assert_eq!(snapshot.request_counters.successful_upstream_responses, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_attempts, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_failures, 0);
        assert_eq!(snapshot.request_counters.upstream_401_count, 1);
        assert_eq!(
            snapshot.last_refresh.kind,
            crate::auth_runtime::RefreshStatusKind::Success
        );

        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[tokio::test]
    async fn device_endpoints_follow_config_refresh_override() {
        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: "https://example.com".to_string(),
                api_key: "test-token".to_string(),
                refresh_token_url: "http://127.0.0.1:18319/oauth/token".to_string(),
            },
            ..AppConfig::default()
        };

        let endpoints = crate::codex::device_endpoints_from_config(&config);
        assert_eq!(
            endpoints.oauth_token_url,
            "http://127.0.0.1:18319/oauth/token"
        );
    }

    #[tokio::test]
    async fn runtime_refresh_path_uses_configured_refresh_endpoint() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            Json(json!({
                "received_authorization": auth
            }))
        }

        async fn refresh_handler() -> impl IntoResponse {
            Json(json!({
                "access_token": "configured-token",
                "refresh_token": "configured-refresh-token",
                "id_token": format!("header.{}.sig", encoded_claims_placeholder()),
                "expires_in": 3600
            }))
        }

        fn encoded_claims_placeholder() -> String {
            let payload = serde_json::json!({
                "email": "refresh@example.com",
                "exp": 1893456000_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": "pro"
                }
            });
            base64::engine::general_purpose::URL_SAFE_NO_PAD
                .encode(serde_json::to_vec(&payload).unwrap())
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "".to_string(),
                refresh_token_url: format!("{refresh_url}/oauth/token"),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "expired-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let proxy_app = build_app(config, accounts);
        let (proxy_url, proxy_shutdown) = start_listener(proxy_app).await;

        let response = Client::new()
            .get(format!("{proxy_url}/v1/models"))
            .send()
            .await
            .unwrap();
        assert_eq!(response.status(), StatusCode::OK);

        let body = response.json::<Value>().await.unwrap();
        assert_eq!(body["received_authorization"], "Bearer configured-token");

        let stats: Value = Client::new()
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_eq!(stats["request_counters"]["auth_refresh_attempts"], 1);
        assert_eq!(stats["request_counters"]["auth_refresh_failures"], 0);
        assert_eq!(stats["last_refresh"]["kind"], "success");

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[tokio::test]
    async fn refreshes_expired_active_account_before_first_request() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            Json(json!({
                "received_authorization": auth
            }))
        }

        async fn refresh_handler() -> impl IntoResponse {
            Json(json!({
                "access_token": "preemptive-fresh-token",
                "refresh_token": "fresh-refresh-token",
                "id_token": format!("header.{}.sig", encoded_claims_placeholder("team")),
                "expires_in": 3600
            }))
        }

        fn encoded_claims_placeholder(plan: &str) -> String {
            let payload = serde_json::json!({
                "email": "refresh@example.com",
                "exp": 1767225600_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": plan
                }
            });
            base64::engine::general_purpose::URL_SAFE_NO_PAD
                .encode(serde_json::to_vec(&payload).unwrap())
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url,
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "expired-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state =
            build_test_state_with_endpoints(config, accounts, format!("{refresh_url}/oauth/token"));

        let response = forward_upstream(
            &state,
            Method::GET,
            format!("{}/v1/models", state.config.codex.upstream_base_url),
            HeaderMap::new(),
            Bytes::new(),
        )
        .await
        .unwrap();
        let body = axum::body::to_bytes(response.into_body(), usize::MAX)
            .await
            .unwrap();
        let payload: Value = serde_json::from_slice(&body).unwrap();

        assert_eq!(
            payload["received_authorization"],
            "Bearer preemptive-fresh-token"
        );
        let active = state.accounts.read().await.active_codex_account().unwrap();
        assert_eq!(active.api_key, "preemptive-fresh-token");
        assert_eq!(active.plan_type.as_deref(), Some("team"));

        let snapshot = state.runtime_stats_snapshot().await;
        assert_eq!(snapshot.request_counters.total_proxied_requests, 1);
        assert_eq!(snapshot.request_counters.successful_upstream_responses, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_attempts, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_failures, 0);
        assert_eq!(
            snapshot.last_refresh.kind,
            crate::auth_runtime::RefreshStatusKind::Success
        );

        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[tokio::test]
    async fn unsupported_routes_do_not_change_success_counters() {
        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: "https://example.com".to_string(),
                api_key: "fallback-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(app).await;
        let client = Client::new();

        let response = client
            .post(format!("{proxy_url}/v1/files"))
            .json(&json!({}))
            .send()
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::NOT_FOUND);
        let snapshot: Value = client
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_eq!(snapshot["request_counters"]["total_proxied_requests"], 0);
        assert_eq!(
            snapshot["request_counters"]["successful_upstream_responses"],
            0
        );

        let _ = proxy_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_endpoint_is_local_and_does_not_hit_upstream() {
        let upstream_hits = std::sync::Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let upstream_hits_for_handler = upstream_hits.clone();
        let upstream_app = Router::new().route(
            "/v1/models",
            get(move || {
                let upstream_hits_for_handler = upstream_hits_for_handler.clone();
                async move {
                    upstream_hits_for_handler.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                    Json(json!({
                        "object": "list",
                        "data": []
                    }))
                }
            }),
        );
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url,
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(app).await;

        let payload: Value = Client::new()
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();

        assert_eq!(payload["bind_address"], "127.0.0.1:0");
        assert_eq!(upstream_hits.load(std::sync::atomic::Ordering::SeqCst), 0);

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_reports_in_memory_snapshot_and_counters() {
        async fn upstream_handler() -> impl IntoResponse {
            Json(json!({
                "object": "list",
                "data": []
            }))
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "fallback-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_or_replace_codex_account("primary".to_string(), "state-token".to_string(), true)
            .unwrap();
        accounts.accounts.push(crate::state::AccountEntry {
            name: "mixed".to_string(),
            provider: "anthropic".to_string(),
            api_key: "other-token".to_string(),
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            plan_type: None,
            expires_at: None,
            source: Some("manual".to_string()),
        });
        accounts.accounts.push(crate::state::AccountEntry {
            name: "blank".to_string(),
            provider: "codex".to_string(),
            api_key: "   ".to_string(),
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            plan_type: None,
            expires_at: None,
            source: Some("manual".to_string()),
        });

        let app = build_app(config, accounts);
        let (proxy_url, proxy_shutdown) = start_listener(app).await;
        let client = Client::new();

        let snapshot: Value = client
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();

        assert_eq!(snapshot["bind_address"], "127.0.0.1:0");
        assert_eq!(snapshot["upstream_base_url"], upstream_url);
        assert_eq!(snapshot["active_account_name"], "primary");
        assert_eq!(snapshot["account_count"], 1);
        assert_eq!(snapshot["auth_health"]["state"], "static");
        assert_eq!(snapshot["request_counters"]["total_proxied_requests"], 0);
        assert_eq!(snapshot["last_refresh"]["kind"], "unknown");

        let response = client
            .get(format!("{proxy_url}/v1/models"))
            .send()
            .await
            .unwrap();
        assert_eq!(response.status(), StatusCode::OK);

        let snapshot: Value = client
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_eq!(snapshot["request_counters"]["total_proxied_requests"], 1);
        assert_eq!(
            snapshot["request_counters"]["successful_upstream_responses"],
            1
        );

        let _ = proxy_shutdown.send(());

        let restarted_app = build_app(
            AppConfig {
                server: crate::config::ServerConfig {
                    bind: "127.0.0.1:0".to_string(),
                },
                state: crate::config::StateConfig::default(),
                codex: crate::config::CodexConfig {
                    upstream_base_url: upstream_url.clone(),
                    api_key: "fallback-token".to_string(),
                    refresh_token_url: String::new(),
                },
                ..AppConfig::default()
            },
            {
                let mut accounts = AccountState::default();
                accounts
                    .add_or_replace_codex_account(
                        "primary".to_string(),
                        "state-token".to_string(),
                        true,
                    )
                    .unwrap();
                accounts
            },
        );
        let (restarted_url, restarted_shutdown) = start_listener(restarted_app).await;
        let restarted_snapshot: Value = client
            .get(format!("{restarted_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();
        assert_eq!(
            restarted_snapshot["request_counters"]["total_proxied_requests"],
            0
        );
        assert_eq!(restarted_snapshot["last_refresh"]["kind"], "unknown");

        let _ = upstream_shutdown.send(());
        let _ = restarted_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_account_count_stays_on_runtime_snapshot_after_disk_changes() {
        let upstream_app = Router::new().route(
            "/v1/models",
            get(|| async { Json(json!({"object": "list", "data": []})) }),
        );
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url,
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_or_replace_codex_account("primary".to_string(), "state-token".to_string(), true)
            .unwrap();

        let state = AppState::new(config, accounts);
        {
            let mut live_accounts = state.accounts.write().await;
            live_accounts.accounts.push(crate::state::AccountEntry {
                name: "mixed".to_string(),
                provider: "anthropic".to_string(),
                api_key: "other-token".to_string(),
                refresh_token: None,
                id_token: None,
                email: None,
                account_id: None,
                plan_type: None,
                expires_at: None,
                source: Some("manual".to_string()),
            });
            live_accounts.accounts.push(crate::state::AccountEntry {
                name: "blank".to_string(),
                provider: "codex".to_string(),
                api_key: "   ".to_string(),
                refresh_token: None,
                id_token: None,
                email: None,
                account_id: None,
                plan_type: None,
                expires_at: None,
                source: Some("manual".to_string()),
            });
        }

        let snapshot = state.runtime_stats_snapshot().await;
        assert_eq!(snapshot.account_count, 1);

        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_uses_fallback_key_without_implying_active_saved_account() {
        let upstream_app = Router::new().route(
            "/v1/models",
            get(|| async { Json(json!({"object": "list", "data": []})) }),
        );
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "fallback-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let app = build_app(config, AccountState::default());
        let (proxy_url, proxy_shutdown) = start_listener(app).await;
        let payload: Value = Client::new()
            .get(format!("{proxy_url}/v0/runtime/stats"))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap();

        assert!(payload["active_account_name"].is_null());
        assert_eq!(payload["auth_health"]["state"], "static");
        assert_eq!(payload["request_counters"]["total_proxied_requests"], 0);

        let _ = proxy_shutdown.send(());
        let _ = upstream_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_recomputes_after_successful_refresh() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            Json(json!({
                "received_authorization": auth
            }))
        }

        async fn refresh_handler() -> impl IntoResponse {
            Json(json!({
                "access_token": "fresh-token",
                "refresh_token": "fresh-refresh-token",
                "id_token": format!("header.{}.sig", encoded_claims_placeholder("pro")),
                "expires_in": 3600
            }))
        }

        fn encoded_claims_placeholder(plan: &str) -> String {
            let payload = serde_json::json!({
                "email": "refresh@example.com",
                "exp": 1893456000_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": plan
                }
            });
            base64::engine::general_purpose::URL_SAFE_NO_PAD
                .encode(serde_json::to_vec(&payload).unwrap())
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "fallback-token".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "stale-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("free".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state =
            build_test_state_with_endpoints(config, accounts, format!("{refresh_url}/oauth/token"));

        let before = state.runtime_stats_snapshot().await;
        assert_eq!(
            before.auth_health.state,
            crate::auth_runtime::AuthHealthState::Expired
        );
        assert_eq!(
            before.auth_health.expires_at.as_deref(),
            Some("1970-01-01T00:00:00Z")
        );

        let response = forward_upstream(
            &state,
            Method::GET,
            format!("{}/v1/models", state.config.codex.upstream_base_url),
            HeaderMap::new(),
            Bytes::new(),
        )
        .await
        .unwrap();
        let body = axum::body::to_bytes(response.into_body(), usize::MAX)
            .await
            .unwrap();
        let payload: Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(payload["received_authorization"], "Bearer fresh-token");

        let after = state.runtime_stats_snapshot().await;
        assert_eq!(
            after.auth_health.state,
            crate::auth_runtime::AuthHealthState::Valid
        );
        assert_eq!(
            after.auth_health.source,
            AuthCredentialSource::ActiveAccount
        );
        assert_eq!(
            after.auth_health.expires_at.as_deref(),
            Some("2030-01-01T00:00:00Z")
        );
        assert_eq!(after.request_counters.auth_refresh_attempts, 1);
        assert_eq!(after.request_counters.auth_refresh_failures, 0);
        assert_eq!(
            after.last_refresh.kind,
            crate::auth_runtime::RefreshStatusKind::Success
        );
        assert_eq!(after.last_refresh.account_name.as_deref(), Some("primary"));

        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_records_refresh_failure_on_proactive_refresh() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            Json(json!({
                "received_authorization": auth
            }))
        }

        async fn refresh_handler() -> impl IntoResponse {
            (
                StatusCode::BAD_GATEWAY,
                Json(json!({"error": "refresh failed"})),
            )
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "expired-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state =
            build_test_state_with_endpoints(config, accounts, format!("{refresh_url}/oauth/token"));

        let error = forward_upstream(
            &state,
            Method::GET,
            format!("{}/v1/models", state.config.codex.upstream_base_url),
            HeaderMap::new(),
            Bytes::new(),
        )
        .await
        .unwrap_err();
        assert_eq!(error.status, StatusCode::BAD_GATEWAY);

        let snapshot = state.runtime_stats_snapshot().await;
        assert_eq!(snapshot.request_counters.total_proxied_requests, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_attempts, 1);
        assert_eq!(snapshot.request_counters.auth_refresh_failures, 1);
        assert_eq!(
            snapshot.last_refresh.kind,
            crate::auth_runtime::RefreshStatusKind::Failure
        );
        assert_eq!(
            snapshot.auth_health.state,
            crate::auth_runtime::AuthHealthState::Expired
        );

        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[tokio::test]
    async fn runtime_stats_updates_health_after_refresh_rewrites_expired_account() {
        async fn upstream_handler(headers: HeaderMap) -> impl IntoResponse {
            let auth = headers
                .get("authorization")
                .and_then(|value| value.to_str().ok())
                .unwrap_or_default()
                .to_string();

            Json(json!({
                "received_authorization": auth
            }))
        }

        async fn refresh_handler() -> impl IntoResponse {
            Json(json!({
                "access_token": "fresh-token",
                "refresh_token": "fresh-refresh-token",
                "id_token": format!("header.{}.sig", encoded_claims_placeholder("pro")),
                "expires_in": 3600
            }))
        }

        fn encoded_claims_placeholder(plan: &str) -> String {
            let payload = serde_json::json!({
                "email": "refresh@example.com",
                "exp": 1893456000_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": plan
                }
            });
            base64::engine::general_purpose::URL_SAFE_NO_PAD
                .encode(serde_json::to_vec(&payload).unwrap())
        }

        let upstream_app = Router::new().route("/v1/models", get(upstream_handler));
        let refresh_app = Router::new().route("/oauth/token", post(refresh_handler));
        let (upstream_url, upstream_shutdown) = start_listener(upstream_app).await;
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: "127.0.0.1:0".to_string(),
            },
            state: crate::config::StateConfig::default(),
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "".to_string(),
                refresh_token_url: String::new(),
            },
            ..AppConfig::default()
        };

        let mut accounts = AccountState::default();
        accounts
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "stale-token".to_string(),
                    refresh_token: "old-refresh-token".to_string(),
                    id_token: "".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct_refresh".to_string()),
                    plan_type: Some("free".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state =
            build_test_state_with_endpoints(config, accounts, format!("{refresh_url}/oauth/token"));

        let before = state.runtime_stats_snapshot().await;
        assert_eq!(
            before.auth_health.state,
            crate::auth_runtime::AuthHealthState::Expired
        );
        assert_eq!(
            before.auth_health.expires_at.as_deref(),
            Some("1970-01-01T00:00:00Z")
        );

        let response = forward_upstream(
            &state,
            Method::GET,
            format!("{}/v1/models", state.config.codex.upstream_base_url),
            HeaderMap::new(),
            Bytes::new(),
        )
        .await
        .unwrap();
        let body = axum::body::to_bytes(response.into_body(), usize::MAX)
            .await
            .unwrap();
        let payload: Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(payload["received_authorization"], "Bearer fresh-token");

        let after = state.runtime_stats_snapshot().await;
        assert_eq!(
            after.auth_health.state,
            crate::auth_runtime::AuthHealthState::Valid
        );
        assert_eq!(
            after.auth_health.expires_at.as_deref(),
            Some("2030-01-01T00:00:00Z")
        );
        assert_eq!(after.request_counters.auth_refresh_attempts, 1);
        assert_eq!(
            after.last_refresh.kind,
            crate::auth_runtime::RefreshStatusKind::Success
        );

        let _ = upstream_shutdown.send(());
        let _ = refresh_shutdown.send(());
    }

    #[test]
    fn parses_proxy_rfc3339_z_timestamp() {
        assert_eq!(parse_rfc3339_z("1970-01-01T00:00:00Z"), Some(0));
        assert_eq!(parse_rfc3339_z("1970-01-02T00:00:00Z"), Some(86_400));
        assert_eq!(parse_rfc3339_z("2026-04-05 12:00:00"), None);
    }
}
