use anyhow::{Result, bail};

use crate::provider::{Provider, ProviderId, RefreshResult};

pub struct ClaudeProvider {
    upstream_base_url: String,
}

impl ClaudeProvider {
    pub fn new(upstream_base_url: String) -> Self {
        Self { upstream_base_url }
    }

    pub fn from_config(config: &crate::config::AppConfig) -> Self {
        Self::new(config.claude.upstream_base_url.clone())
    }
}

impl Provider for ClaudeProvider {
    fn id(&self) -> ProviderId {
        ProviderId::CLAUDE
    }

    fn display_name(&self) -> &'static str {
        "Claude"
    }

    fn upstream_base_url(&self) -> &str {
        &self.upstream_base_url
    }

    fn default_upstream_base_url() -> &'static str
    where
        Self: Sized,
    {
        "https://api.anthropic.com"
    }

    fn refresh_token<'a>(
        &'a self,
        _refresh_token: &'a str,
    ) -> std::pin::Pin<Box<dyn std::future::Future<Output = Result<RefreshResult>> + Send + 'a>> {
        Box::pin(async move {
            bail!("Claude refresh is not implemented yet")
        })
    }
}
