use std::sync::Arc;

use anyhow::{Result, bail};
use tokio::sync::RwLock;

use crate::codex::CodexProvider;
use crate::config::AppConfig;
use crate::provider::{Provider, ProviderId};
use crate::state::AccountState;

pub struct ProviderRegistry {
    providers: Vec<Arc<dyn Provider>>,
}

impl ProviderRegistry {
    pub fn new(config: &AppConfig) -> Self {
        let providers: Vec<Arc<dyn Provider>> = vec![
            Arc::new(CodexProvider::from_config(config)),
        ];
        Self { providers }
    }

    pub fn active_provider(&self, config: &AppConfig) -> Arc<dyn Provider> {
        let tag = config.active_provider();
        self.providers
            .iter()
            .find(|p| p.provider_tag() == tag)
            .cloned()
            .unwrap_or_else(|| {
                self.providers
                    .first()
                    .cloned()
                    .expect("at least one provider must be registered")
            })
    }

    pub fn get(&self, id: ProviderId) -> Option<Arc<dyn Provider>> {
        self.providers
            .iter()
            .find(|p| p.id() == id)
            .cloned()
    }
}

#[derive(Clone)]
pub struct ResolvedProvider {
    pub provider: Arc<dyn Provider>,
    pub api_key: Option<String>,
    pub refresh_token: Option<String>,
    pub account_name: Option<String>,
}

pub async fn resolve_active_provider(
    config: &AppConfig,
    registry: &ProviderRegistry,
    accounts: &RwLock<AccountState>,
) -> ResolvedProvider {
    let provider = registry.active_provider(config);
    let tag = provider.provider_tag();

    let (api_key, refresh_token, account_name) = {
        let state = accounts.read().await;
        let active = state.active_account_for_provider(tag);
        let key = active
            .as_ref()
            .map(|a| a.api_key.clone())
            .or_else(|| match tag {
                crate::provider::CODEX_PROVIDER => {
                    let fallback = config.codex.api_key.trim();
                    if fallback.is_empty() { None } else { Some(fallback.to_string()) }
                }
                _ => None,
            });
        let refresh = active.as_ref().and_then(|a| a.refresh_token.clone());
        let name = active.as_ref().map(|a| a.name.clone());
        (key, refresh, name)
    };

    ResolvedProvider {
        provider,
        api_key,
        refresh_token,
        account_name,
    }
}
