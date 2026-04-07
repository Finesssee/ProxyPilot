use std::time::Duration;

use anyhow::{Context, Result, bail};
use base64::Engine;
use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use tokio::time::sleep;

use crate::provider::{Provider, ProviderId, RefreshResult};

pub const CLIENT_ID: &str = "app_EMoamEEZ73f0CkXaXp7hrann";
pub const DEVICE_USER_CODE_URL: &str = "https://auth.openai.com/api/accounts/deviceauth/usercode";
pub const DEVICE_TOKEN_URL: &str = "https://auth.openai.com/api/accounts/deviceauth/token";
pub const DEVICE_VERIFICATION_URL: &str = "https://auth.openai.com/codex/device";
pub const DEVICE_REDIRECT_URI: &str = "https://auth.openai.com/deviceauth/callback";
pub const OAUTH_TOKEN_URL: &str = "https://auth.openai.com/oauth/token";

const DEFAULT_POLL_INTERVAL_SECS: u64 = 5;
const DEVICE_TIMEOUT_SECS: u64 = 15 * 60;

#[derive(Debug, Clone)]
pub struct DeviceAuthResult {
    pub access_token: String,
    pub refresh_token: String,
    pub id_token: String,
    pub email: Option<String>,
    pub account_id: Option<String>,
    pub plan_type: Option<String>,
    pub expires_at: Option<String>,
}

pub struct CodexProvider {
    upstream_base_url: String,
    refresh_token_url: String,
    client: Client,
}

impl CodexProvider {
    pub fn new(upstream_base_url: String, refresh_token_url: String) -> Self {
        Self {
            upstream_base_url,
            refresh_token_url,
            client: Client::new(),
        }
    }

    pub fn from_config(config: &crate::config::AppConfig) -> Self {
        Self::new(
            config.codex.upstream_base_url.clone(),
            config.codex.refresh_token_url.clone(),
        )
    }
}

impl Provider for CodexProvider {
    fn id(&self) -> ProviderId {
        ProviderId::CODEX
    }

    fn display_name(&self) -> &'static str {
        "Codex (OpenAI)"
    }

    fn upstream_base_url(&self) -> &str {
        &self.upstream_base_url
    }

    fn default_upstream_base_url() -> &'static str {
        "https://api.openai.com"
    }

    async fn refresh_token(
        &self,
        refresh_token: &str,
    ) -> Result<RefreshResult> {
        let endpoints = if self.refresh_token_url.trim().is_empty() {
            DeviceEndpoints::default()
        } else {
            DeviceEndpoints {
                oauth_token_url: self.refresh_token_url.clone(),
                ..DeviceEndpoints::default()
            }
        };
        let result = refresh_with_refresh_token_for_test(
            self.client.clone(),
            endpoints,
            refresh_token,
        )
        .await?;

        Ok(RefreshResult {
            access_token: result.access_token,
            refresh_token: Some(result.refresh_token),
            id_token: Some(result.id_token),
            expires_at: result.expires_at,
        })
    }
}

#[derive(Debug, Clone)]
pub struct DeviceEndpoints {
    pub user_code_url: String,
    pub token_poll_url: String,
    pub verification_url: String,
    pub oauth_token_url: String,
    pub redirect_uri: String,
}

impl Default for DeviceEndpoints {
    fn default() -> Self {
        Self {
            user_code_url: DEVICE_USER_CODE_URL.to_string(),
            token_poll_url: DEVICE_TOKEN_URL.to_string(),
            verification_url: DEVICE_VERIFICATION_URL.to_string(),
            oauth_token_url: OAUTH_TOKEN_URL.to_string(),
            redirect_uri: DEVICE_REDIRECT_URI.to_string(),
        }
    }
}

pub fn device_endpoints_from_config(config: &crate::config::AppConfig) -> DeviceEndpoints {
    let refresh_token_url = config.codex.refresh_token_url.trim();
    if refresh_token_url.is_empty() {
        DeviceEndpoints::default()
    } else {
        DeviceEndpoints {
            oauth_token_url: refresh_token_url.to_string(),
            ..DeviceEndpoints::default()
        }
    }
}

#[derive(Debug, Serialize)]
struct DeviceCodeRequest<'a> {
    client_id: &'a str,
}

#[derive(Debug, Deserialize)]
struct DeviceCodeResponse {
    device_auth_id: String,
    #[serde(default)]
    user_code: String,
    #[serde(default)]
    usercode: String,
    #[serde(default)]
    interval: Option<Value>,
}

#[derive(Debug, Serialize)]
struct DeviceTokenPollRequest<'a> {
    device_auth_id: &'a str,
    user_code: &'a str,
}

#[derive(Debug, Deserialize)]
struct DeviceTokenPollResponse {
    authorization_code: String,
    code_verifier: String,
}

#[derive(Debug, Serialize)]
struct TokenExchangeRequest<'a> {
    grant_type: &'a str,
    client_id: &'a str,
    code: &'a str,
    redirect_uri: &'a str,
    code_verifier: &'a str,
}

#[derive(Debug, Deserialize)]
struct TokenExchangeResponse {
    access_token: String,
    refresh_token: String,
    id_token: String,
    #[serde(default)]
    expires_in: i64,
}

#[derive(Debug, Deserialize)]
struct JwtClaims {
    #[serde(default)]
    email: String,
    #[serde(default)]
    exp: i64,
    #[serde(default, rename = "https://api.openai.com/auth")]
    codex_auth: JwtCodexAuth,
}

#[derive(Debug, Default, Deserialize)]
struct JwtCodexAuth {
    #[serde(default)]
    chatgpt_account_id: String,
    #[serde(default)]
    chatgpt_plan_type: String,
}

pub async fn login_with_device_flow() -> Result<DeviceAuthResult> {
    login_with_device_flow_for_test(Client::new(), DeviceEndpoints::default()).await
}

pub async fn refresh_with_refresh_token(refresh_token: &str) -> Result<DeviceAuthResult> {
    refresh_with_refresh_token_for_test(Client::new(), DeviceEndpoints::default(), refresh_token)
        .await
}

pub async fn refresh_with_refresh_token_from_config(
    config: &crate::config::AppConfig,
    refresh_token: &str,
) -> Result<DeviceAuthResult> {
    refresh_with_refresh_token_for_test(
        Client::new(),
        device_endpoints_from_config(config),
        refresh_token,
    )
    .await
}

pub async fn login_with_device_flow_for_test(
    client: Client,
    endpoints: DeviceEndpoints,
) -> Result<DeviceAuthResult> {
    let code_response = request_device_code(&client, &endpoints).await?;
    let user_code = if code_response.user_code.trim().is_empty() {
        code_response.usercode.trim().to_string()
    } else {
        code_response.user_code.trim().to_string()
    };
    let device_auth_id = code_response.device_auth_id.trim().to_string();

    if user_code.is_empty() || device_auth_id.is_empty() {
        bail!("codex device flow did not return required fields");
    }

    let poll_interval = parse_poll_interval(code_response.interval.as_ref());

    println!("Starting Codex device authentication...");
    println!("Codex device URL: {}", endpoints.verification_url);
    println!("Codex device code: {}", user_code);
    println!("Approve the login in your browser, then come back here.");

    let device_tokens = poll_for_authorization(
        &client,
        &endpoints,
        &device_auth_id,
        &user_code,
        poll_interval,
    )
    .await?;

    if device_tokens.authorization_code.trim().is_empty()
        || device_tokens.code_verifier.trim().is_empty()
    {
        bail!("codex device flow token response missing required fields");
    }

    exchange_authorization_code(&client, &endpoints, &device_tokens).await
}

pub async fn refresh_with_refresh_token_for_test(
    client: Client,
    endpoints: DeviceEndpoints,
    refresh_token: &str,
) -> Result<DeviceAuthResult> {
    let trimmed = refresh_token.trim();
    if trimmed.is_empty() {
        bail!("refresh token is required");
    }

    let response = client
        .post(&endpoints.oauth_token_url)
        .header("Accept", "application/json")
        .form(&[
            ("client_id", CLIENT_ID),
            ("grant_type", "refresh_token"),
            ("refresh_token", trimmed),
            ("scope", "openid profile email"),
        ])
        .send()
        .await
        .context("failed to refresh Codex tokens")?;

    let status = response.status();
    let body = response
        .text()
        .await
        .context("failed to read Codex refresh response")?;
    if !status.is_success() {
        bail!(
            "codex token refresh failed with status {}: {}",
            status,
            body
        );
    }

    let payload: TokenExchangeResponse =
        serde_json::from_str(&body).context("failed to decode Codex refresh response")?;
    build_auth_result(payload)
}

async fn request_device_code(
    client: &Client,
    endpoints: &DeviceEndpoints,
) -> Result<DeviceCodeResponse> {
    let response = client
        .post(&endpoints.user_code_url)
        .header("Accept", "application/json")
        .json(&DeviceCodeRequest {
            client_id: CLIENT_ID,
        })
        .send()
        .await
        .context("failed to request Codex device code")?;

    let status = response.status();
    let body = response
        .text()
        .await
        .context("failed to read device code response")?;
    if !status.is_success() {
        bail!(
            "codex device code request failed with status {}: {}",
            status,
            body
        );
    }

    serde_json::from_str(&body).context("failed to decode Codex device code response")
}

async fn poll_for_authorization(
    client: &Client,
    endpoints: &DeviceEndpoints,
    device_auth_id: &str,
    user_code: &str,
    poll_interval: Duration,
) -> Result<DeviceTokenPollResponse> {
    let deadline = tokio::time::Instant::now() + Duration::from_secs(DEVICE_TIMEOUT_SECS);

    loop {
        if tokio::time::Instant::now() >= deadline {
            bail!("codex device authentication timed out after 15 minutes");
        }

        let response = client
            .post(&endpoints.token_poll_url)
            .header("Accept", "application/json")
            .json(&DeviceTokenPollRequest {
                device_auth_id,
                user_code,
            })
            .send()
            .await
            .context("failed to poll Codex device token")?;

        let status = response.status();
        let body = response
            .text()
            .await
            .context("failed to read Codex device poll response")?;

        if status.is_success() {
            return serde_json::from_str(&body)
                .context("failed to decode Codex device token response");
        }

        if status.as_u16() == 403 || status.as_u16() == 404 {
            sleep(poll_interval).await;
            continue;
        }

        let trimmed = body.trim();
        bail!(
            "codex device token polling failed with status {}: {}",
            status,
            if trimmed.is_empty() {
                "empty response body"
            } else {
                trimmed
            }
        );
    }
}

async fn exchange_authorization_code(
    client: &Client,
    endpoints: &DeviceEndpoints,
    device_tokens: &DeviceTokenPollResponse,
) -> Result<DeviceAuthResult> {
    let response = client
        .post(&endpoints.oauth_token_url)
        .header("Accept", "application/json")
        .form(&TokenExchangeRequest {
            grant_type: "authorization_code",
            client_id: CLIENT_ID,
            code: device_tokens.authorization_code.trim(),
            redirect_uri: &endpoints.redirect_uri,
            code_verifier: device_tokens.code_verifier.trim(),
        })
        .send()
        .await
        .context("failed to exchange device authorization code for tokens")?;

    let status = response.status();
    let body = response
        .text()
        .await
        .context("failed to read OAuth token response")?;
    if !status.is_success() {
        bail!(
            "codex token exchange failed with status {}: {}",
            status,
            body
        );
    }

    let payload: TokenExchangeResponse =
        serde_json::from_str(&body).context("failed to decode OAuth token response")?;
    build_auth_result(payload)
}

fn build_auth_result(payload: TokenExchangeResponse) -> Result<DeviceAuthResult> {
    let claims = parse_jwt_claims(&payload.id_token).ok();
    let expires_at = claims
        .as_ref()
        .and_then(|claims| {
            if claims.exp > 0 {
                Some(claims.exp)
            } else {
                None
            }
        })
        .map(format_unix_timestamp)
        .or_else(|| {
            if payload.expires_in > 0 {
                Some(format_unix_timestamp(
                    chrono_like_now_secs() + payload.expires_in,
                ))
            } else {
                None
            }
        });

    Ok(DeviceAuthResult {
        access_token: payload.access_token,
        refresh_token: payload.refresh_token,
        id_token: payload.id_token,
        email: claims
            .as_ref()
            .and_then(|claims| trimmed_option(&claims.email)),
        account_id: claims
            .as_ref()
            .and_then(|claims| trimmed_option(&claims.codex_auth.chatgpt_account_id)),
        plan_type: claims
            .as_ref()
            .and_then(|claims| trimmed_option(&claims.codex_auth.chatgpt_plan_type)),
        expires_at,
    })
}

fn parse_poll_interval(value: Option<&Value>) -> Duration {
    let default = Duration::from_secs(DEFAULT_POLL_INTERVAL_SECS);
    let Some(value) = value else {
        return default;
    };

    match value {
        Value::Number(number) => number.as_u64().map(Duration::from_secs).unwrap_or(default),
        Value::String(text) => text
            .trim()
            .parse::<u64>()
            .map(Duration::from_secs)
            .unwrap_or(default),
        _ => default,
    }
}

fn parse_jwt_claims(token: &str) -> Result<JwtClaims> {
    let payload = token
        .split('.')
        .nth(1)
        .context("invalid JWT token format")?;
    let bytes = URL_SAFE_NO_PAD
        .decode(payload)
        .context("failed to decode JWT payload")?;
    serde_json::from_slice(&bytes).context("failed to parse JWT claims")
}

fn trimmed_option(value: &str) -> Option<String> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        None
    } else {
        Some(trimmed.to_string())
    }
}

fn chrono_like_now_secs() -> i64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs() as i64)
        .unwrap_or_default()
}

fn format_unix_timestamp(timestamp: i64) -> String {
    use std::time::{Duration as StdDuration, UNIX_EPOCH};

    let when = UNIX_EPOCH + StdDuration::from_secs(timestamp.max(0) as u64);
    let datetime: chrono_like::DateTime = when.into();
    datetime.to_rfc3339()
}

mod chrono_like {
    use std::time::SystemTime;

    pub struct DateTime(SystemTime);

    impl From<SystemTime> for DateTime {
        fn from(value: SystemTime) -> Self {
            Self(value)
        }
    }

    impl DateTime {
        pub fn to_rfc3339(&self) -> String {
            use std::time::UNIX_EPOCH;

            let duration = self
                .0
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_secs() as i64;

            let secs = duration;
            let tm = time_parts(secs);
            format!(
                "{:04}-{:02}-{:02}T{:02}:{:02}:{:02}Z",
                tm.year, tm.month, tm.day, tm.hour, tm.minute, tm.second
            )
        }
    }

    struct Parts {
        year: i64,
        month: i64,
        day: i64,
        hour: i64,
        minute: i64,
        second: i64,
    }

    fn time_parts(mut secs: i64) -> Parts {
        let second = secs.rem_euclid(60);
        secs = (secs - second) / 60;
        let minute = secs.rem_euclid(60);
        secs = (secs - minute) / 60;
        let hour = secs.rem_euclid(24);
        let days = (secs - hour) / 24;

        civil_from_days(days).with_time(hour, minute, second)
    }

    struct Civil {
        year: i64,
        month: i64,
        day: i64,
    }

    impl Civil {
        fn with_time(self, hour: i64, minute: i64, second: i64) -> Parts {
            Parts {
                year: self.year,
                month: self.month,
                day: self.day,
                hour,
                minute,
                second,
            }
        }
    }

    fn civil_from_days(days: i64) -> Civil {
        let z = days + 719468;
        let era = if z >= 0 { z } else { z - 146096 } / 146097;
        let doe = z - era * 146097;
        let yoe = (doe - doe / 1460 + doe / 36524 - doe / 146096) / 365;
        let y = yoe + era * 400;
        let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
        let mp = (5 * doy + 2) / 153;
        let d = doy - (153 * mp + 2) / 5 + 1;
        let m = mp + if mp < 10 { 3 } else { -9 };
        let year = y + if m <= 2 { 1 } else { 0 };
        Civil {
            year,
            month: m,
            day: d,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::extract::State;
    use axum::response::IntoResponse;
    use axum::routing::post;
    use axum::{Json, Router};
    use serde_json::json;
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
    async fn device_flow_returns_tokens_and_claims() {
        async fn user_code() -> impl axum::response::IntoResponse {
            Json(json!({
                "device_auth_id": "device-auth-id",
                "user_code": "ABCD-EFGH",
                "interval": 0
            }))
        }

        async fn token_poll(
            State(done): State<std::sync::Arc<std::sync::atomic::AtomicBool>>,
        ) -> impl axum::response::IntoResponse {
            if done.swap(true, std::sync::atomic::Ordering::SeqCst) {
                (
                    axum::http::StatusCode::FORBIDDEN,
                    Json(json!({"pending": true})),
                )
                    .into_response()
            } else {
                Json(json!({
                    "authorization_code": "auth-code",
                    "code_verifier": "verifier",
                    "code_challenge": "challenge"
                }))
                .into_response()
            }
        }

        async fn exchange() -> impl axum::response::IntoResponse {
            let payload = json!({
                "email": "device@example.com",
                "exp": 1767225600_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_device",
                    "chatgpt_plan_type": "plus"
                }
            });
            let claims = URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload).unwrap());
            Json(json!({
                "access_token": "device-access-token",
                "refresh_token": "device-refresh-token",
                "id_token": format!("header.{}.sig", claims),
                "expires_in": 3600
            }))
        }

        let done = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
        let user_code_app = Router::new().route("/usercode", post(user_code));
        let token_app = Router::new()
            .route("/token", post(token_poll))
            .with_state(done);
        let exchange_app = Router::new().route("/oauth/token", post(exchange));

        let (user_code_url, user_shutdown) = start_listener(user_code_app).await;
        let (token_url, token_shutdown) = start_listener(token_app).await;
        let (oauth_url, oauth_shutdown) = start_listener(exchange_app).await;

        let result = login_with_device_flow_for_test(
            Client::new(),
            DeviceEndpoints {
                user_code_url: format!("{user_code_url}/usercode"),
                token_poll_url: format!("{token_url}/token"),
                verification_url: "https://example.test/device".to_string(),
                oauth_token_url: format!("{oauth_url}/oauth/token"),
                redirect_uri: "https://example.test/callback".to_string(),
            },
        )
        .await
        .unwrap();

        assert_eq!(result.access_token, "device-access-token");
        assert_eq!(result.refresh_token, "device-refresh-token");
        assert_eq!(result.email.as_deref(), Some("device@example.com"));
        assert_eq!(result.account_id.as_deref(), Some("acct_device"));
        assert_eq!(result.plan_type.as_deref(), Some("plus"));

        let _ = user_shutdown.send(());
        let _ = token_shutdown.send(());
        let _ = oauth_shutdown.send(());
    }

    #[tokio::test]
    async fn refresh_flow_returns_updated_tokens_and_claims() {
        async fn refresh_exchange() -> impl axum::response::IntoResponse {
            let payload = json!({
                "email": "refresh@example.com",
                "exp": 1767225600_i64,
                "https://api.openai.com/auth": {
                    "chatgpt_account_id": "acct_refresh",
                    "chatgpt_plan_type": "pro"
                }
            });
            let claims = URL_SAFE_NO_PAD.encode(serde_json::to_vec(&payload).unwrap());
            Json(json!({
                "access_token": "refreshed-access-token",
                "refresh_token": "refreshed-refresh-token",
                "id_token": format!("header.{}.sig", claims),
                "expires_in": 3600
            }))
        }

        let refresh_app = Router::new().route("/oauth/token", post(refresh_exchange));
        let (refresh_url, refresh_shutdown) = start_listener(refresh_app).await;

        let result = refresh_with_refresh_token_for_test(
            Client::new(),
            DeviceEndpoints {
                oauth_token_url: format!("{refresh_url}/oauth/token"),
                ..DeviceEndpoints::default()
            },
            "old-refresh-token",
        )
        .await
        .unwrap();

        assert_eq!(result.access_token, "refreshed-access-token");
        assert_eq!(result.refresh_token, "refreshed-refresh-token");
        assert_eq!(result.email.as_deref(), Some("refresh@example.com"));
        assert_eq!(result.account_id.as_deref(), Some("acct_refresh"));
        assert_eq!(result.plan_type.as_deref(), Some("pro"));

        let _ = refresh_shutdown.send(());
    }
}
