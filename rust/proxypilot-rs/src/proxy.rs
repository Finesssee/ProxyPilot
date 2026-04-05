use std::net::SocketAddr;
use std::sync::Arc;

use anyhow::{Context, Result};
use axum::body::{Body, Bytes};
use axum::extract::{OriginalUri, Path, State};
use axum::http::{HeaderMap, Method, Response, StatusCode};
use axum::response::IntoResponse;
use axum::routing::{any, get};
use axum::{Json, Router};
use reqwest::Client;
use serde_json::json;
use tracing::info;

use crate::config::AppConfig;

#[derive(Clone)]
pub struct AppState {
    config: Arc<AppConfig>,
    client: Client,
}

impl AppState {
    pub fn new(config: AppConfig) -> Self {
        Self {
            config: Arc::new(config),
            client: Client::new(),
        }
    }
}

pub fn build_app(config: AppConfig) -> Router {
    let state = AppState::new(config);

    Router::new()
        .route("/healthz", get(healthz))
        .route("/v1/{*path}", any(proxy_codex))
        .with_state(state)
}

pub async fn run(config: AppConfig) -> Result<()> {
    let bind_addr: SocketAddr = config
        .server
        .bind
        .parse()
        .with_context(|| format!("invalid bind address {}", config.server.bind))?;

    let app = build_app(config.clone());
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
    Json(json!({
        "status": "ok",
        "provider": "codex",
        "listen": state.config.server.bind,
        "upstream": state.config.codex.upstream_base_url,
    }))
}

async fn proxy_codex(
    State(state): State<AppState>,
    Path(path): Path<String>,
    original_uri: OriginalUri,
    method: Method,
    headers: HeaderMap,
    body: Bytes,
) -> Result<Response<Body>, ProxyError> {
    let target = build_upstream_url(&state.config.codex.upstream_base_url, &path, &original_uri)?;
    let mut request = state.client.request(method, target);

    for (name, value) in &headers {
        if should_skip_request_header(name.as_str()) {
            continue;
        }
        request = request.header(name, value);
    }

    if !state.config.codex.api_key.trim().is_empty() {
        request = request.bearer_auth(state.config.codex.api_key.trim());
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

fn build_upstream_url(
    base_url: &str,
    path: &str,
    original_uri: &OriginalUri,
) -> Result<String, ProxyError> {
    let cleaned_base = base_url.trim_end_matches('/');
    let suffix = if path.is_empty() {
        original_uri
            .0
            .path_and_query()
            .map(|value| value.as_str())
            .unwrap_or("/v1")
    } else {
        original_uri
            .0
            .path_and_query()
            .map(|value| value.as_str())
            .unwrap_or("/v1")
    };

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
    use axum::routing::post;
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
            codex: crate::config::CodexConfig {
                upstream_base_url: upstream_url.clone(),
                api_key: "test-token".to_string(),
            },
        };

        let proxy_app = build_app(config);
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
}
