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

use crate::config::AppConfig;
use crate::state::AccountState;

#[derive(Clone)]
pub struct AppState {
    config: Arc<AppConfig>,
    accounts: Arc<RwLock<AccountState>>,
    client: Client,
}

impl AppState {
    pub fn new(config: AppConfig, accounts: AccountState) -> Self {
        Self {
            config: Arc::new(config),
            accounts: Arc::new(RwLock::new(accounts)),
            client: Client::new(),
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
    let token = {
        let accounts = state.accounts.read().await;
        accounts.effective_codex_api_key(&state.config)
    };
    let mut request = state.client.request(method, target);

    for (name, value) in &headers {
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

    let upstream = request
        .body(body)
        .send()
        .await
        .context("failed to reach upstream Codex endpoint")?;

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
    use axum::routing::{get, post};
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
}
