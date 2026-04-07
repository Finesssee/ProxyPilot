use std::future::Future;
use std::pin::Pin;

use anyhow::Result;
use reqwest;
use serde::{Deserialize, Serialize};

use crate::state::AccountEntry;

pub const CODEX_PROVIDER: &str = "codex";
pub const CLAUDE_PROVIDER: &str = "claude";
pub const GEMINI_PROVIDER: &str = "gemini";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Hash)]
pub struct ProviderId(pub &'static str);

impl ProviderId {
    pub const CODEX: ProviderId = ProviderId(CODEX_PROVIDER);
    pub const CLAUDE: ProviderId = ProviderId(CLAUDE_PROVIDER);
    pub const GEMINI: ProviderId = ProviderId(GEMINI_PROVIDER);

    pub fn as_str(&self) -> &'static str {
        self.0
    }
}

impl std::fmt::Display for ProviderId {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.0)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthResult {
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub id_token: Option<String>,
    pub email: Option<String>,
    pub account_id: Option<String>,
    pub plan_type: Option<String>,
    pub expires_at: Option<String>,
}

impl AuthResult {
    pub fn into_account_entry(
        self,
        name: String,
        source: String,
        _activate: bool,
    ) -> AccountEntry {
        AccountEntry {
            name,
            provider: self.access_token_provider_name().to_string(),
            api_key: self.access_token,
            refresh_token: self.refresh_token,
            id_token: self.id_token,
            email: self.email,
            account_id: self.account_id,
            plan_type: self.plan_type,
            expires_at: self.expires_at,
            source: Some(source),
        }
    }

    fn access_token_provider_name(&self) -> &str {
        CODEX_PROVIDER
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RefreshResult {
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub id_token: Option<String>,
    pub email: Option<String>,
    pub account_id: Option<String>,
    pub plan_type: Option<String>,
    pub expires_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ModelInfo {
    pub id: String,
    pub owned_by: Option<String>,
}

pub trait Provider: Send + Sync + 'static {
    fn id(&self) -> ProviderId;

    fn display_name(&self) -> &'static str;

    fn provider_tag(&self) -> &'static str {
        self.id().as_str()
    }

    fn upstream_base_url(&self) -> &str;

    fn default_upstream_base_url() -> &'static str
    where
        Self: Sized;

    fn refresh_token<'a>(
        &'a self,
        refresh_token: &'a str,
    ) -> Pin<Box<dyn Future<Output = Result<RefreshResult>> + Send + 'a>>;

    fn models_path(&self) -> &str {
        "/v1/models"
    }

    fn chat_completions_path(&self) -> &str {
        "/v1/chat/completions"
    }

    fn responses_path(&self) -> &str {
        "/v1/responses"
    }

    fn apply_auth_headers(
        &self,
        request: reqwest::RequestBuilder,
        api_key: &str,
    ) -> reqwest::RequestBuilder {
        request.bearer_auth(api_key)
    }
}

pub fn provider_from_tag(tag: &str) -> Option<ProviderId> {
    match tag {
        CODEX_PROVIDER => Some(ProviderId::CODEX),
        CLAUDE_PROVIDER => Some(ProviderId::CLAUDE),
        GEMINI_PROVIDER => Some(ProviderId::GEMINI),
        _ => None,
    }
}

pub fn is_known_provider(tag: &str) -> bool {
    provider_from_tag(tag).is_some()
}
