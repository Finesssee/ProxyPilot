use std::fs;
use std::path::Path;

use anyhow::{Context, Result, bail};
use serde::{Deserialize, Serialize};

use crate::config::AppConfig;

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AccountState {
    #[serde(default)]
    pub active_account: Option<String>,
    #[serde(default)]
    pub accounts: Vec<AccountEntry>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AccountEntry {
    pub name: String,
    pub provider: String,
    pub api_key: String,
}

#[derive(Debug, Clone)]
pub struct ActiveCodexAccount {
    pub name: String,
    pub api_key: String,
}

impl AccountState {
    pub fn load_or_default(path: &Path) -> Result<Self> {
        if !path.exists() {
            return Ok(Self::default());
        }
        let raw = fs::read_to_string(path)
            .with_context(|| format!("failed to read state file {}", path.display()))?;
        let state: Self = toml::from_str(&raw)
            .with_context(|| format!("failed to parse state file {}", path.display()))?;
        Ok(state)
    }

    pub fn save(&self, path: &Path) -> Result<()> {
        if let Some(parent) = path.parent()
            && !parent.as_os_str().is_empty()
        {
            fs::create_dir_all(parent).with_context(|| {
                format!("failed to create state directory {}", parent.display())
            })?;
        }
        let raw = toml::to_string_pretty(self).context("failed to serialize account state")?;
        fs::write(path, raw)
            .with_context(|| format!("failed to write state file {}", path.display()))?;
        Ok(())
    }

    pub fn add_or_replace_codex_account(
        &mut self,
        name: String,
        api_key: String,
        activate: bool,
    ) -> Result<()> {
        let trimmed_name = name.trim();
        let trimmed_key = api_key.trim();
        if trimmed_name.is_empty() {
            bail!("account name cannot be empty");
        }
        if trimmed_key.is_empty() {
            bail!("api key cannot be empty");
        }

        self.accounts.retain(|account| account.name != trimmed_name);
        self.accounts.push(AccountEntry {
            name: trimmed_name.to_string(),
            provider: "codex".to_string(),
            api_key: trimmed_key.to_string(),
        });

        if activate || self.active_account.is_none() {
            self.active_account = Some(trimmed_name.to_string());
        }
        self.accounts.sort_by(|a, b| a.name.cmp(&b.name));
        Ok(())
    }

    pub fn activate(&mut self, name: &str) -> Result<()> {
        let trimmed = name.trim();
        if trimmed.is_empty() {
            bail!("account name cannot be empty");
        }
        if self.accounts.iter().any(|account| account.name == trimmed) {
            self.active_account = Some(trimmed.to_string());
            Ok(())
        } else {
            bail!("no saved account named {}", trimmed)
        }
    }

    pub fn active_codex_account(&self) -> Option<ActiveCodexAccount> {
        let active_name = self.active_account.as_deref()?;
        self.accounts
            .iter()
            .find(|account| account.provider == "codex" && account.name == active_name)
            .map(|account| ActiveCodexAccount {
                name: account.name.clone(),
                api_key: account.api_key.clone(),
            })
    }

    pub fn effective_codex_api_key(&self, config: &AppConfig) -> Option<String> {
        self.active_codex_account()
            .map(|account| account.api_key)
            .or_else(|| {
                let fallback = config.codex.api_key.trim();
                if fallback.is_empty() {
                    None
                } else {
                    Some(fallback.to_string())
                }
            })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn active_account_overrides_config_fallback() {
        let mut state = AccountState::default();
        state
            .add_or_replace_codex_account("primary".to_string(), "state-key".to_string(), true)
            .unwrap();

        let config = AppConfig::default();
        assert_eq!(
            state.effective_codex_api_key(&config).as_deref(),
            Some("state-key")
        );
    }

    #[test]
    fn config_fallback_is_used_when_no_account_exists() {
        let config = AppConfig {
            codex: crate::config::CodexConfig {
                upstream_base_url: "https://api.openai.com".to_string(),
                api_key: "fallback-key".to_string(),
            },
            ..AppConfig::default()
        };

        let state = AccountState::default();
        assert_eq!(
            state.effective_codex_api_key(&config).as_deref(),
            Some("fallback-key")
        );
    }
}
