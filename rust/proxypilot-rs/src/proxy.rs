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

use crate::auth_runtime::{self, AuthCredentialSource};
use crate::config::AppConfig;
use crate::state::AccountState;

#[derive(Clone)]
pub struct AppState {
    config: Arc<AppConfig>,
    accounts: Arc<RwLock<AccountState>>,
    client: Client,
    device_endpoints: Arc<crate::codex::DeviceEndpoints>,
}

impl AppState {
    pub fn new(config: AppConfig, accounts: AccountState) -> Self {
        Self {
            config: Arc::new(config),
            accounts: Arc::new(RwLock::new(accounts)),
            client: Client::new(),
            device_endpoints: Arc::new(crate::codex::DeviceEndpoints::default()),
        }
    }
}

pub fn build_app(config: AppConfig, accounts: AccountState) -> Router {
    let state = AppState::new(config, accounts);

    Router::new()
        .route("/healthz", get(healthz))
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
        upstream = %config.codex.upstream_base_url,
        "proxypilot-rs listening"
    );
    axum::serve(listener, app)
        .await
        .context("proxy server exited unexpectedly")?;
    Ok(())
}

async fn healthz(State(state): State<AppState>) -> impl IntoResponse {
    let account_state = state.accounts.read().await;
    let active_account = account_state
        .active_codex_account()
        .map(|account| account.name);
    Json(json!({
        "status": "ok",
        "provider": "codex",
        "listen": state.config.server.bind,
        "upstream": state.config.codex.upstream_base_url,
        "active_account": active_account,
    }))
}

async fn list_models(
    State(state): State<AppState>,
    original_uri: OriginalUri,
    headers: HeaderMap,
) -> Result<Response<Body>, ProxyError> {
    forward_upstream(
        &state,
        Method::GET,
        build_upstream_url(&state.config.codex.upstream_base_url, &original_uri)?,
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
        build_upstream_url(&state.config.codex.upstream_base_url, &original_uri)?,
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
        build_upstream_url(&state.config.codex.upstream_base_url, &original_uri)?,
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
    if active_account_needs_refresh(state).await {
        let _ = try_refresh_active_codex_account(state).await?;
    }

    let token = {
        let accounts = state.accounts.read().await;
        accounts.effective_codex_api_key(&state.config)
    };
    let upstream = send_upstream_request(state, &method, &target, &headers, &body, token)
        .await
        .context("failed to reach upstream Codex endpoint")?;

    let upstream = if upstream.status() == StatusCode::UNAUTHORIZED {
        if let Some(refreshed_token) = try_refresh_active_codex_account(state).await? {
            send_upstream_request(
                state,
                &method,
                &target,
                &headers,
                &body,
                Some(refreshed_token),
            )
            .await
            .context("failed to retry upstream Codex endpoint after refresh")?
        } else {
            upstream
        }
    } else {
        upstream
    };

    let status = upstream.status();
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

async fn try_refresh_active_codex_account(state: &AppState) -> Result<Option<String>, ProxyError> {
    let active = {
        let accounts = state.accounts.read().await;
        accounts.active_codex_account()
    };

    let Some(active) = active else {
        return Ok(None);
    };
    let Some(refresh_token) = active.refresh_token.as_deref() else {
        return Ok(None);
    };

    let refreshed = crate::codex::refresh_with_refresh_token_for_test(
        state.client.clone(),
        (*state.device_endpoints).clone(),
        refresh_token,
    )
    .await
    .map_err(ProxyError::from)?;

    {
        let mut accounts = state.accounts.write().await;
        accounts
            .update_codex_account_tokens(&active.name, refreshed)
            .map_err(ProxyError::from)?;
    }

    let refreshed_token = {
        let accounts = state.accounts.read().await;
        accounts
            .active_codex_account()
            .map(|account| account.api_key)
    };

    Ok(refreshed_token)
}

async fn active_account_needs_refresh(state: &AppState) -> bool {
    let active = {
        let accounts = state.accounts.read().await;
        accounts.active_codex_account()
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
            },
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
            },
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
            },
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
            },
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
            },
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
                "exp": 1767225600_i64,
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
            },
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
                    expires_at: Some("2026-04-06T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let state = AppState {
            config: std::sync::Arc::new(config),
            accounts: std::sync::Arc::new(tokio::sync::RwLock::new(accounts)),
            client: Client::new(),
            device_endpoints: std::sync::Arc::new(crate::codex::DeviceEndpoints {
                oauth_token_url: format!("{refresh_url}/oauth/token"),
                ..crate::codex::DeviceEndpoints::default()
            }),
        };

        let refreshed = {
            let active = state.accounts.read().await.active_codex_account().unwrap();
            let response = crate::codex::refresh_with_refresh_token_for_test(
                Client::new(),
                (*state.device_endpoints).clone(),
                active.refresh_token.as_deref().unwrap(),
            )
            .await
            .unwrap();
            state
                .accounts
                .write()
                .await
                .update_codex_account_tokens("primary", response)
                .unwrap();
            state
                .accounts
                .read()
                .await
                .active_codex_account()
                .unwrap()
                .api_key
        };

        assert_eq!(refreshed, "fresh-token");

        let response = send_upstream_request(
            &state,
            &Method::GET,
            &format!("{}/v1/models", state.config.codex.upstream_base_url),
            &HeaderMap::new(),
            &Bytes::new(),
            Some(
                state
                    .accounts
                    .read()
                    .await
                    .active_codex_account()
                    .unwrap()
                    .api_key,
            ),
        )
        .await
        .unwrap();
        let payload: Value = response.json().await.unwrap();
        assert_eq!(payload["received_authorization"], "Bearer fresh-token");

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
            },
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

        let state = AppState {
            config: std::sync::Arc::new(config),
            accounts: std::sync::Arc::new(tokio::sync::RwLock::new(accounts)),
            client: Client::new(),
            device_endpoints: std::sync::Arc::new(crate::codex::DeviceEndpoints {
                oauth_token_url: format!("{refresh_url}/oauth/token"),
                ..crate::codex::DeviceEndpoints::default()
            }),
        };

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
