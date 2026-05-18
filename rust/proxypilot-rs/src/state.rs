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
    pub plan_type: Option<String>,
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
    pub plan_type: Option<String>,
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

    pub fn add_or_replace_manual_account(
        &mut self,
        provider: &str,
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
            provider: provider.to_string(),
            api_key: trimmed_key.to_string(),
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            plan_type: None,
            expires_at: None,
            source: Some("manual".to_string()),
        });

        if activate || self.active_account.is_none() {
            self.active_account = Some(trimmed_name.to_string());
        }
        self.accounts.sort_by(|a, b| a.name.cmp(&b.name));
        Ok(())
    }

    pub fn add_or_replace_codex_account(
        &mut self,
        name: String,
        api_key: String,
        activate: bool,
    ) -> Result<()> {
        self.add_or_replace_manual_account(crate::provider::CODEX_PROVIDER, name, api_key, activate)
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

    pub fn remove_account(&mut self, name: &str) -> Result<()> {
        let trimmed = name.trim();
        if trimmed.is_empty() {
            bail!("account name cannot be empty");
        }

        let original_len = self.accounts.len();
        self.accounts.retain(|account| account.name != trimmed);
        if self.accounts.len() == original_len {
            bail!("no saved account named {}", trimmed);
        }

        if self.active_account.as_deref() == Some(trimmed) {
            self.active_account = self.accounts.first().map(|account| account.name.clone());
        }

        Ok(())
    }

    pub fn merge_from(&mut self, imported: AccountState) -> usize {
        let mut imported_count = 0;
        for imported_account in imported.accounts {
            self.accounts
                .retain(|account| account.name != imported_account.name);
            self.accounts.push(imported_account);
            imported_count += 1;
        }

        if self.active_account.is_none() {
            self.active_account = imported.active_account;
        }
        if let Some(active) = self.active_account.as_deref()
            && !self.accounts.iter().any(|account| account.name == active)
        {
            self.active_account = self.accounts.first().map(|account| account.name.clone());
        }
        self.accounts.sort_by(|a, b| a.name.cmp(&b.name));
        imported_count
    }

    pub fn active_account_for_provider(&self, provider: &str) -> Option<ActiveCodexAccount> {
        let active_name = self.active_account.as_deref()?;
        self.accounts
            .iter()
            .find(|account| account.provider == provider && account.name == active_name)
            .map(|account| ActiveCodexAccount {
                name: account.name.clone(),
                api_key: account.api_key.clone(),
                refresh_token: account.refresh_token.clone(),
                id_token: account.id_token.clone(),
                email: account.email.clone(),
                account_id: account.account_id.clone(),
                plan_type: account.plan_type.clone(),
                expires_at: account.expires_at.clone(),
            })
    }

    pub fn account_by_name_and_provider(
        &self,
        name: &str,
        provider: &str,
    ) -> Option<ActiveCodexAccount> {
        let trimmed = name.trim();
        self.accounts
            .iter()
            .find(|account| account.provider == provider && account.name == trimmed)
            .map(|account| ActiveCodexAccount {
                name: account.name.clone(),
                api_key: account.api_key.clone(),
                refresh_token: account.refresh_token.clone(),
                id_token: account.id_token.clone(),
                email: account.email.clone(),
                account_id: account.account_id.clone(),
                plan_type: account.plan_type.clone(),
                expires_at: account.expires_at.clone(),
            })
    }

    pub fn runtime_usable_account_count_for_provider(&self, provider: &str) -> usize {
        self.accounts
            .iter()
            .filter(|account| account.provider == provider && !account.api_key.trim().is_empty())
            .count()
    }

    pub fn update_account_tokens_for_provider(
        &mut self,
        account_name: &str,
        provider: &str,
        access_token: String,
        refresh_token: Option<String>,
        id_token: Option<String>,
        email: Option<String>,
        account_id: Option<String>,
        plan_type: Option<String>,
        expires_at: Option<String>,
    ) -> Result<()> {
        let trimmed = account_name.trim();
        let entry = self
            .accounts
            .iter_mut()
            .find(|account| account.provider == provider && account.name == trimmed)
            .ok_or_else(|| anyhow::anyhow!("no saved {} account named {}", provider, trimmed))?;

        entry.api_key = access_token;
        if refresh_token.is_some() {
            entry.refresh_token = refresh_token;
        }
        if id_token.is_some() {
            entry.id_token = id_token;
        }
        if email.is_some() {
            entry.email = email;
        }
        if account_id.is_some() {
            entry.account_id = account_id;
        }
        if plan_type.is_some() {
            entry.plan_type = plan_type;
        }
        if expires_at.is_some() {
            entry.expires_at = expires_at;
        }
        Ok(())
    }

    // Legacy codex-specific wrappers (delegate to provider-agnostic methods)

    pub fn active_codex_account(&self) -> Option<ActiveCodexAccount> {
        self.active_account_for_provider(crate::provider::CODEX_PROVIDER)
    }

    pub fn runtime_usable_codex_account_count(&self) -> usize {
        self.runtime_usable_account_count_for_provider(crate::provider::CODEX_PROVIDER)
    }

    pub fn codex_account_by_name(&self, name: &str) -> Option<ActiveCodexAccount> {
        self.account_by_name_and_provider(name, crate::provider::CODEX_PROVIDER)
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
    pub id_token: String,
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
            id_token: optional_trimmed(imported.id_token),
            email: optional_trimmed(imported.email),
            account_id: optional_trimmed(imported.account_id),
            plan_type: None,
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
            plan_type: result.plan_type,
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
        self.update_account_tokens_for_provider(
            account_name,
            crate::provider::CODEX_PROVIDER,
            result.access_token,
            optional_trimmed(result.refresh_token),
            optional_trimmed(result.id_token),
            result.email,
            result.account_id,
            result.plan_type,
            result.expires_at,
        )
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
                refresh_token_url: String::new(),
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
                    id_token: "id".to_string(),
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
        assert_eq!(account.id_token.as_deref(), Some("id"));
        assert_eq!(account.plan_type.as_deref(), None);
        assert_eq!(account.expires_at.as_deref(), Some("2026-04-06T00:00:00Z"));
        assert_eq!(account.source.as_deref(), Some("file"));
    }

    #[test]
    fn imported_codex_auth_accepts_go_token_storage_shape() {
        let imported: ImportedCodexAuth = serde_json::from_str(
            r#"{
                "id_token": "go-id-token",
                "access_token": "go-access-token",
                "refresh_token": "go-refresh-token",
                "account_id": "acct_go",
                "last_refresh": "2026-04-05T00:00:00Z",
                "email": "go@example.com",
                "type": "codex",
                "expired": "2026-04-06T00:00:00Z"
            }"#,
        )
        .unwrap();

        let mut state = AccountState::default();
        state
            .add_imported_codex_account(
                "go-file".to_string(),
                imported,
                "fixture".to_string(),
                true,
            )
            .unwrap();

        let account = state.active_codex_account().unwrap();
        assert_eq!(account.api_key, "go-access-token");
        assert_eq!(account.refresh_token.as_deref(), Some("go-refresh-token"));
        assert_eq!(account.id_token.as_deref(), Some("go-id-token"));
        assert_eq!(account.email.as_deref(), Some("go@example.com"));
        assert_eq!(account.account_id.as_deref(), Some("acct_go"));
        assert_eq!(account.expires_at.as_deref(), Some("2026-04-06T00:00:00Z"));
    }

    #[test]
    fn removing_active_account_promotes_next_saved_account() {
        let mut state = AccountState::default();
        state
            .add_or_replace_codex_account("primary".to_string(), "key-1".to_string(), true)
            .unwrap();
        state
            .add_or_replace_codex_account("backup".to_string(), "key-2".to_string(), false)
            .unwrap();

        state.remove_account("primary").unwrap();

        assert_eq!(state.active_account.as_deref(), Some("backup"));
        assert_eq!(state.accounts.len(), 1);
        assert_eq!(state.accounts[0].name, "backup");
    }

    #[test]
    fn merge_from_replaces_named_accounts_and_preserves_existing_active_account() {
        let mut state = AccountState::default();
        state
            .add_or_replace_codex_account("primary".to_string(), "old-key".to_string(), true)
            .unwrap();

        let mut imported = AccountState {
            active_account: Some("imported-active".to_string()),
            accounts: Vec::new(),
        };
        imported
            .add_or_replace_codex_account("primary".to_string(), "new-key".to_string(), false)
            .unwrap();
        imported
            .add_or_replace_manual_account(
                crate::provider::CLAUDE_PROVIDER,
                "claude-main".to_string(),
                "claude-key".to_string(),
                false,
            )
            .unwrap();

        assert_eq!(state.merge_from(imported), 2);

        assert_eq!(state.active_account.as_deref(), Some("primary"));
        assert_eq!(state.accounts.len(), 2);
        assert_eq!(
            state.codex_account_by_name("primary").unwrap().api_key,
            "new-key"
        );
        assert_eq!(
            state
                .account_by_name_and_provider("claude-main", crate::provider::CLAUDE_PROVIDER)
                .unwrap()
                .api_key,
            "claude-key"
        );
    }

    #[test]
    fn merge_from_uses_imported_active_when_local_has_none() {
        let mut state = AccountState::default();
        let mut imported = AccountState::default();
        imported
            .add_or_replace_codex_account("imported".to_string(), "key".to_string(), true)
            .unwrap();

        state.merge_from(imported);

        assert_eq!(state.active_account.as_deref(), Some("imported"));
    }

    #[test]
    fn device_account_carries_plan_type() {
        let mut state = AccountState::default();
        state
            .add_device_codex_account(
                "device".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "access".to_string(),
                    refresh_token: "refresh".to_string(),
                    id_token: "id".to_string(),
                    email: Some("dev@example.com".to_string()),
                    account_id: Some("acct_123".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("2026-04-06T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();

        let account = state.active_codex_account().unwrap();
        assert_eq!(account.plan_type.as_deref(), Some("pro"));
    }

    #[test]
    fn manual_claude_account_uses_provider_generic_path() {
        let mut state = AccountState::default();
        state
            .add_or_replace_manual_account(
                crate::provider::CLAUDE_PROVIDER,
                "claude-main".to_string(),
                "claude-key".to_string(),
                true,
            )
            .unwrap();

        let account = state
            .active_account_for_provider(crate::provider::CLAUDE_PROVIDER)
            .unwrap();
        assert_eq!(account.name, "claude-main");
        assert_eq!(account.api_key, "claude-key");
        assert_eq!(
            state.runtime_usable_account_count_for_provider(crate::provider::CLAUDE_PROVIDER),
            1
        );
    }

    #[test]
    fn runtime_usable_codex_account_count_ignores_mixed_provider_and_empty_key_entries() {
        let mut state = AccountState::default();
        state.accounts.push(AccountEntry {
            name: "primary".to_string(),
            provider: "codex".to_string(),
            api_key: "state-key".to_string(),
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            plan_type: None,
            expires_at: None,
            source: Some("manual".to_string()),
        });
        state.accounts.push(AccountEntry {
            name: "mixed".to_string(),
            provider: "anthropic".to_string(),
            api_key: "other-key".to_string(),
            refresh_token: None,
            id_token: None,
            email: None,
            account_id: None,
            plan_type: None,
            expires_at: None,
            source: Some("manual".to_string()),
        });
        state.accounts.push(AccountEntry {
            name: "broken".to_string(),
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

        assert_eq!(state.runtime_usable_codex_account_count(), 1);
    }
}
