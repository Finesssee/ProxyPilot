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
    #[serde(default)]
    pub refresh_token: Option<String>,
    #[serde(default)]
    pub id_token: Option<String>,
    #[serde(default)]
    pub email: Option<String>,
    #[serde(default)]
    pub account_id: Option<String>,
    #[serde(default)]
    pub expires_at: Option<String>,
    #[serde(default)]
    pub source: Option<String>,
}

#[derive(Debug, Clone)]
pub struct ActiveCodexAccount {
    pub name: String,
    pub api_key: String,
    pub refresh_token: Option<String>,
    pub id_token: Option<String>,
    pub email: Option<String>,
    pub account_id: Option<String>,
    pub expires_at: Option<String>,
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
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            expires_at: None,
            source: Some("manual".to_string()),
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
                refresh_token: account.refresh_token.clone(),
                id_token: account.id_token.clone(),
                email: account.email.clone(),
                account_id: account.account_id.clone(),
                expires_at: account.expires_at.clone(),
            })
    }

    pub fn codex_account_by_name(&self, name: &str) -> Option<ActiveCodexAccount> {
        let trimmed = name.trim();
        self.accounts
            .iter()
            .find(|account| account.provider == "codex" && account.name == trimmed)
            .map(|account| ActiveCodexAccount {
                name: account.name.clone(),
                api_key: account.api_key.clone(),
                refresh_token: account.refresh_token.clone(),
                id_token: account.id_token.clone(),
                email: account.email.clone(),
                account_id: account.account_id.clone(),
                expires_at: account.expires_at.clone(),
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

#[derive(Debug, Clone, Deserialize)]
pub struct ImportedCodexAuth {
    #[serde(default)]
    pub access_token: String,
    #[serde(default)]
    pub refresh_token: String,
    #[serde(default)]
    pub email: String,
    #[serde(default)]
    pub account_id: String,
    #[serde(default, alias = "expired")]
    pub expires_at: String,
    #[serde(default, rename = "type")]
    pub provider_type: String,
}

impl AccountState {
    pub fn add_imported_codex_account(
        &mut self,
        name: String,
        imported: ImportedCodexAuth,
        source: String,
        activate: bool,
    ) -> Result<()> {
        let trimmed_name = name.trim();
        if trimmed_name.is_empty() {
            bail!("account name cannot be empty");
        }
        if imported.access_token.trim().is_empty() {
            bail!("imported Codex auth file is missing access_token");
        }

        self.accounts.retain(|account| account.name != trimmed_name);
        self.accounts.push(AccountEntry {
            name: trimmed_name.to_string(),
            provider: if imported.provider_type.trim().is_empty() {
                "codex".to_string()
            } else {
                imported.provider_type.trim().to_string()
            },
            api_key: imported.access_token.trim().to_string(),
            refresh_token: optional_trimmed(imported.refresh_token),
            id_token: None,
            email: optional_trimmed(imported.email),
            account_id: optional_trimmed(imported.account_id),
            expires_at: optional_trimmed(imported.expires_at),
            source: Some(source),
        });

        if activate || self.active_account.is_none() {
            self.active_account = Some(trimmed_name.to_string());
        }
        self.accounts.sort_by(|a, b| a.name.cmp(&b.name));
        Ok(())
    }
}

impl AccountState {
    pub fn add_device_codex_account(
        &mut self,
        name: String,
        result: crate::codex::DeviceAuthResult,
        activate: bool,
    ) -> Result<()> {
        let trimmed_name = name.trim();
        if trimmed_name.is_empty() {
            bail!("account name cannot be empty");
        }
        if result.access_token.trim().is_empty() {
            bail!("device auth result missing access token");
        }

        self.accounts.retain(|account| account.name != trimmed_name);
        self.accounts.push(AccountEntry {
            name: trimmed_name.to_string(),
            provider: "codex".to_string(),
            api_key: result.access_token,
            refresh_token: optional_trimmed(result.refresh_token),
            id_token: optional_trimmed(result.id_token),
            email: result.email,
            account_id: result.account_id,
            expires_at: result.expires_at,
            source: Some("device-login".to_string()),
        });

        if activate || self.active_account.is_none() {
            self.active_account = Some(trimmed_name.to_string());
        }
        self.accounts.sort_by(|a, b| a.name.cmp(&b.name));
        Ok(())
    }

    pub fn update_codex_account_tokens(
        &mut self,
        account_name: &str,
        result: crate::codex::DeviceAuthResult,
    ) -> Result<()> {
        let trimmed = account_name.trim();
        let entry = self
            .accounts
            .iter_mut()
            .find(|account| account.provider == "codex" && account.name == trimmed)
            .ok_or_else(|| anyhow::anyhow!("no saved Codex account named {}", trimmed))?;

        entry.api_key = result.access_token;
        entry.refresh_token = optional_trimmed(result.refresh_token);
        entry.id_token = optional_trimmed(result.id_token);
        if result.email.is_some() {
            entry.email = result.email;
        }
        if result.account_id.is_some() {
            entry.account_id = result.account_id;
        }
        if result.expires_at.is_some() {
            entry.expires_at = result.expires_at;
        }
        Ok(())
    }
}

fn optional_trimmed(value: String) -> Option<String> {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        None
    } else {
        Some(trimmed.to_string())
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

    #[test]
    fn imported_codex_account_carries_metadata() {
        let mut state = AccountState::default();
        state
            .add_imported_codex_account(
                "imported".to_string(),
                ImportedCodexAuth {
                    access_token: "imported-token".to_string(),
                    refresh_token: "refresh".to_string(),
                    email: "dev@example.com".to_string(),
                    account_id: "acct_123".to_string(),
                    expires_at: "2026-04-06T00:00:00Z".to_string(),
                    provider_type: "codex".to_string(),
                },
                "file".to_string(),
                true,
            )
            .unwrap();

        let account = state
            .accounts
            .iter()
            .find(|entry| entry.name == "imported")
            .unwrap();
        assert_eq!(account.email.as_deref(), Some("dev@example.com"));
        assert_eq!(account.account_id.as_deref(), Some("acct_123"));
        assert_eq!(account.expires_at.as_deref(), Some("2026-04-06T00:00:00Z"));
        assert_eq!(account.source.as_deref(), Some("file"));
    }
}
