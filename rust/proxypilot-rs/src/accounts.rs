use std::fs;
use std::path::Path;

use anyhow::{Context, Result};

use crate::codex;
use crate::config::AppConfig;
use crate::state::{AccountState, ImportedCodexAuth};

fn saved_account_message(provider_label: &str, name: &str, active: bool) -> String {
    if active {
        format!(
            "saved {} account `{}` in local state and marked it active",
            provider_label, name
        )
    } else {
        format!("saved {} account `{}` in local state", provider_label, name)
    }
}

fn imported_account_message(name: &str, active: bool) -> String {
    if active {
        format!(
            "imported Codex auth `{}` into local state and marked it active",
            name
        )
    } else {
        format!("imported Codex auth `{}` into local state", name)
    }
}

fn device_login_message(name: &str, active: bool) -> String {
    if active {
        format!(
            "saved Codex device login `{}` in local state and marked it active",
            name
        )
    } else {
        format!("saved Codex device login `{}` in local state", name)
    }
}

fn refresh_message(name: &str) -> String {
    format!("refreshed saved Codex account `{}` in local state", name)
}

fn refresh_mode_label(refresh_token: Option<&str>) -> &'static str {
    if refresh_token
        .map(|value| !value.trim().is_empty())
        .unwrap_or(false)
    {
        "refreshable"
    } else {
        "static"
    }
}

fn account_list_row(account: &crate::state::AccountEntry, active_name: Option<&str>) -> String {
    let marker = if active_name == Some(account.name.as_str()) {
        "*"
    } else {
        "-"
    };
    let email = account.email.as_deref().unwrap_or("-");
    let source = account.source.as_deref().unwrap_or("-");
    let plan = account.plan_type.as_deref().unwrap_or("-");
    let refresh = refresh_mode_label(account.refresh_token.as_deref());
    format!(
        "{marker} {:<16} provider={} email={} plan={} refresh={} source={}",
        account.name, account.provider, email, plan, refresh, source
    )
}

pub fn add_codex_account(
    config: &AppConfig,
    config_path: &Path,
    name: String,
    api_key: String,
    activate: bool,
) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.add_or_replace_codex_account(name.clone(), api_key, activate)?;
    state.save(&state_path)?;

    println!(
        "{}",
        saved_account_message(
            "Codex",
            &name,
            state.active_account.as_deref() == Some(name.as_str())
        )
    );
    println!("state file: {}", state_path.display());
    Ok(())
}

pub fn add_claude_account(
    config: &AppConfig,
    config_path: &Path,
    name: String,
    api_key: String,
    activate: bool,
) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.add_or_replace_manual_account(
        crate::provider::CLAUDE_PROVIDER,
        name.clone(),
        api_key,
        activate,
    )?;
    state.save(&state_path)?;

    println!(
        "{}",
        saved_account_message(
            "Claude",
            &name,
            state.active_account.as_deref() == Some(name.as_str())
        )
    );
    println!("state file: {}", state_path.display());
    Ok(())
}

pub async fn refresh_claude_account(
    config: &AppConfig,
    config_path: &Path,
    name: Option<String>,
) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let state = AccountState::load_or_default(&state_path)?;

    let target = if let Some(name) = name.filter(|value| !value.trim().is_empty()) {
        state
            .account_by_name_and_provider(&name, crate::provider::CLAUDE_PROVIDER)
            .ok_or_else(|| anyhow::anyhow!("no saved Claude account named {}", name))?
    } else {
        state
            .active_account_for_provider(crate::provider::CLAUDE_PROVIDER)
            .ok_or_else(|| anyhow::anyhow!("no active Claude account to refresh"))?
    };

    let _ = target;
    anyhow::bail!("Claude refresh is not implemented yet")
}

pub fn list_accounts(config: &AppConfig, config_path: &Path) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let state = AccountState::load_or_default(&state_path)?;

    if state.accounts.is_empty() {
        println!("No local Rust accounts saved yet.");
        println!("state file: {}", state_path.display());
        return Ok(());
    }

    println!("ProxyPilot Rust local account state");
    println!("state file: {}", state_path.display());
    println!();
    for account in &state.accounts {
        println!(
            "{}",
            account_list_row(account, state.active_account.as_deref())
        );
    }

    Ok(())
}

pub fn activate_account(config: &AppConfig, config_path: &Path, name: String) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.activate(&name)?;
    state.save(&state_path)?;
    println!("local active account set to `{}`", name);
    println!("state file: {}", state_path.display());
    Ok(())
}

pub fn remove_account(config: &AppConfig, config_path: &Path, name: String) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.remove_account(&name)?;
    state.save(&state_path)?;

    println!("removed account `{}` from local state", name);
    match state.active_account.as_deref() {
        Some(active) => println!("local active account is now `{}`", active),
        None => println!("no local active account remains"),
    }
    println!("state file: {}", state_path.display());
    Ok(())
}

pub fn import_codex_account(
    config: &AppConfig,
    config_path: &Path,
    auth_file: &Path,
    name: Option<String>,
    activate: bool,
) -> Result<()> {
    let raw = fs::read_to_string(auth_file)
        .with_context(|| format!("failed to read Codex auth file {}", auth_file.display()))?;
    let imported: ImportedCodexAuth = serde_json::from_str(&raw)
        .with_context(|| format!("failed to parse Codex auth file {}", auth_file.display()))?;

    let resolved_name = name
        .filter(|value| !value.trim().is_empty())
        .or_else(|| {
            if imported.email.trim().is_empty() {
                None
            } else {
                Some(imported.email.trim().to_string())
            }
        })
        .or_else(|| {
            auth_file
                .file_stem()
                .map(|value| value.to_string_lossy().to_string())
        })
        .unwrap_or_else(|| "codex-import".to_string());

    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.add_imported_codex_account(
        resolved_name.clone(),
        imported,
        format!("import:{}", auth_file.display()),
        activate,
    )?;
    state.save(&state_path)?;

    println!(
        "{}",
        imported_account_message(
            &resolved_name,
            state.active_account.as_deref() == Some(resolved_name.as_str())
        )
    );
    println!("source file: {}", auth_file.display());
    println!("state file: {}", state_path.display());
    Ok(())
}

pub fn export_accounts(config: &AppConfig, config_path: &Path, export_file: &Path) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let state = AccountState::load_or_default(&state_path)?;

    if let Some(parent) = export_file.parent()
        && !parent.as_os_str().is_empty()
    {
        fs::create_dir_all(parent)
            .with_context(|| format!("failed to create export directory {}", parent.display()))?;
    }

    let raw = toml::to_string_pretty(&state).context("failed to serialize account state export")?;
    fs::write(export_file, raw)
        .with_context(|| format!("failed to write account export {}", export_file.display()))?;

    println!(
        "exported {} local account(s) to {}",
        state.accounts.len(),
        export_file.display()
    );
    println!("source state file: {}", state_path.display());
    Ok(())
}

pub fn import_accounts(
    config: &AppConfig,
    config_path: &Path,
    import_file: &Path,
    replace: bool,
) -> Result<()> {
    let raw = fs::read_to_string(import_file)
        .with_context(|| format!("failed to read account import {}", import_file.display()))?;
    let imported: AccountState = toml::from_str(&raw)
        .with_context(|| format!("failed to parse account import {}", import_file.display()))?;
    let imported_count = imported.accounts.len();

    let state_path = config.resolve_state_path(config_path);
    let mut state = if replace {
        imported
    } else {
        let mut state = AccountState::load_or_default(&state_path)?;
        state.merge_from(imported);
        state
    };
    state.accounts.sort_by(|a, b| a.name.cmp(&b.name));
    state.save(&state_path)?;

    println!(
        "{} {} local account(s) from {}",
        if replace { "replaced with" } else { "imported" },
        imported_count,
        import_file.display()
    );
    println!("state file: {}", state_path.display());
    Ok(())
}

pub async fn login_codex_device(
    config: &AppConfig,
    config_path: &Path,
    name: Option<String>,
    activate: bool,
) -> Result<()> {
    let result = codex::login_with_device_flow().await?;
    let resolved_name = name
        .filter(|value| !value.trim().is_empty())
        .or_else(|| result.email.clone())
        .or_else(|| result.account_id.clone())
        .unwrap_or_else(|| "codex-device".to_string());

    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.add_device_codex_account(resolved_name.clone(), result, activate)?;
    state.save(&state_path)?;

    println!(
        "{}",
        device_login_message(
            &resolved_name,
            state.active_account.as_deref() == Some(resolved_name.as_str())
        )
    );
    println!("state file: {}", state_path.display());
    Ok(())
}

pub async fn refresh_codex_account(
    config: &AppConfig,
    config_path: &Path,
    name: Option<String>,
) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;

    let target = if let Some(name) = name.filter(|value| !value.trim().is_empty()) {
        state
            .codex_account_by_name(&name)
            .ok_or_else(|| anyhow::anyhow!("no saved Codex account named {}", name))?
    } else {
        state
            .active_codex_account()
            .ok_or_else(|| anyhow::anyhow!("no active Codex account to refresh"))?
    };

    let refresh_token = target.refresh_token.as_deref().ok_or_else(|| {
        anyhow::anyhow!(
            "account `{}` is static and has no refresh token",
            target.name
        )
    })?;

    let result = codex::refresh_with_refresh_token_from_config(config, refresh_token).await?;
    let refreshed_name = target.name.clone();
    state.update_codex_account_tokens(&refreshed_name, result)?;
    state.save(&state_path)?;

    println!("{}", refresh_message(&refreshed_name));
    println!("state file: {}", state_path.display());
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::time::{SystemTime, UNIX_EPOCH};

    use super::*;

    fn temp_paths(label: &str) -> (std::path::PathBuf, std::path::PathBuf) {
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let base = std::env::temp_dir().join(format!("proxypilot-rs-{label}-{stamp}"));
        let config_path = base.with_extension("toml");
        let state_path = base.with_extension("state.toml");
        (config_path, state_path)
    }

    #[test]
    fn list_accounts_marks_refreshability_in_operator_copy() {
        assert_eq!(refresh_mode_label(Some("token")), "refreshable");
        assert_eq!(refresh_mode_label(None), "static");
        assert_eq!(refresh_mode_label(Some("   ")), "static");
    }

    #[test]
    fn account_list_row_marks_active_and_static_accounts_readably() {
        let account = crate::state::AccountEntry {
            name: "primary".to_string(),
            provider: "codex".to_string(),
            api_key: "key".to_string(),
            refresh_token: None,
            id_token: None,
            email: Some("dev@example.com".to_string()),
            account_id: None,
            plan_type: Some("pro".to_string()),
            expires_at: None,
            source: Some("manual".to_string()),
        };

        let row = account_list_row(&account, Some("primary"));
        assert!(row.starts_with("* primary"));
        assert!(row.contains("refresh=static"));
        assert!(row.contains("source=manual"));
    }

    #[test]
    fn saved_account_messages_stay_explicit_about_local_state() {
        assert_eq!(
            saved_account_message("Codex", "primary", true),
            "saved Codex account `primary` in local state and marked it active"
        );
        assert_eq!(
            saved_account_message("Codex", "backup", false),
            "saved Codex account `backup` in local state"
        );
        assert_eq!(
            imported_account_message("imported", true),
            "imported Codex auth `imported` into local state and marked it active"
        );
        assert_eq!(
            device_login_message("device", false),
            "saved Codex device login `device` in local state"
        );
        assert_eq!(
            refresh_message("primary"),
            "refreshed saved Codex account `primary` in local state"
        );
    }

    #[test]
    fn saved_account_messages_can_describe_claude_accounts() {
        assert_eq!(
            saved_account_message("Claude", "claude-main", true),
            "saved Claude account `claude-main` in local state and marked it active"
        );
    }

    #[test]
    fn account_list_row_includes_provider_name_for_claude() {
        let account = crate::state::AccountEntry {
            name: "claude-main".to_string(),
            provider: crate::provider::CLAUDE_PROVIDER.to_string(),
            api_key: "key".to_string(),
            refresh_token: None,
            id_token: None,
            email: Some("dev@example.com".to_string()),
            account_id: None,
            plan_type: Some("max".to_string()),
            expires_at: None,
            source: Some("manual".to_string()),
        };

        let row = account_list_row(&account, Some("claude-main"));
        assert!(row.contains("provider=claude"));
        assert!(row.contains("plan=max"));
    }

    #[tokio::test]
    async fn refresh_codex_account_reports_static_accounts_clearly() {
        let (config_path, state_path) = temp_paths("static-refresh");
        let config = AppConfig {
            state: crate::config::StateConfig {
                path: state_path.display().to_string(),
            },
            ..AppConfig::default()
        };
        fs::write(&config_path, AppConfig::example_toml()).unwrap();

        let mut state = AccountState::default();
        state
            .add_or_replace_codex_account("primary".to_string(), "sk-test".to_string(), true)
            .unwrap();
        state.save(&state_path).unwrap();

        let error = refresh_codex_account(&config, &config_path, Some("primary".to_string()))
            .await
            .unwrap_err()
            .to_string();
        assert!(error.contains("static"));
        assert!(error.contains("no refresh token"));

        let _ = fs::remove_file(config_path);
        let _ = fs::remove_file(state_path);
    }

    #[tokio::test]
    async fn refresh_codex_account_reports_missing_targets_clearly() {
        let (config_path, state_path) = temp_paths("missing-refresh");
        let config = AppConfig {
            state: crate::config::StateConfig {
                path: state_path.display().to_string(),
            },
            ..AppConfig::default()
        };
        fs::write(&config_path, AppConfig::example_toml()).unwrap();
        AccountState::default().save(&state_path).unwrap();

        let error = refresh_codex_account(&config, &config_path, Some("missing".to_string()))
            .await
            .unwrap_err()
            .to_string();
        assert!(error.contains("no saved Codex account named missing"));

        let _ = fs::remove_file(config_path);
        let _ = fs::remove_file(state_path);
    }

    #[test]
    fn export_and_import_accounts_round_trip_full_state() {
        let (config_path, state_path) = temp_paths("export-import");
        let export_path = state_path.with_extension("export.toml");
        let config = AppConfig {
            state: crate::config::StateConfig {
                path: state_path.display().to_string(),
            },
            ..AppConfig::default()
        };
        fs::write(&config_path, AppConfig::example_toml()).unwrap();

        let mut state = AccountState::default();
        state
            .add_or_replace_codex_account("primary".to_string(), "codex-key".to_string(), true)
            .unwrap();
        state
            .add_or_replace_manual_account(
                crate::provider::CLAUDE_PROVIDER,
                "claude-main".to_string(),
                "claude-key".to_string(),
                false,
            )
            .unwrap();
        state.save(&state_path).unwrap();

        export_accounts(&config, &config_path, &export_path).unwrap();
        fs::remove_file(&state_path).unwrap();
        import_accounts(&config, &config_path, &export_path, true).unwrap();

        let imported = AccountState::load_or_default(&state_path).unwrap();
        assert_eq!(imported.active_account.as_deref(), Some("primary"));
        assert_eq!(imported.accounts.len(), 2);
        assert_eq!(
            imported.codex_account_by_name("primary").unwrap().api_key,
            "codex-key"
        );
        assert_eq!(
            imported
                .account_by_name_and_provider("claude-main", crate::provider::CLAUDE_PROVIDER)
                .unwrap()
                .api_key,
            "claude-key"
        );

        let _ = fs::remove_file(config_path);
        let _ = fs::remove_file(state_path);
        let _ = fs::remove_file(export_path);
    }

    #[test]
    fn import_accounts_merges_without_overwriting_active_account() {
        let (config_path, state_path) = temp_paths("merge-import");
        let import_path = state_path.with_extension("import.toml");
        let config = AppConfig {
            state: crate::config::StateConfig {
                path: state_path.display().to_string(),
            },
            ..AppConfig::default()
        };
        fs::write(&config_path, AppConfig::example_toml()).unwrap();

        let mut local = AccountState::default();
        local
            .add_or_replace_codex_account("primary".to_string(), "old-key".to_string(), true)
            .unwrap();
        local.save(&state_path).unwrap();

        let mut imported = AccountState::default();
        imported
            .add_or_replace_codex_account("primary".to_string(), "new-key".to_string(), true)
            .unwrap();
        imported
            .add_or_replace_manual_account(
                crate::provider::CLAUDE_PROVIDER,
                "claude-main".to_string(),
                "claude-key".to_string(),
                false,
            )
            .unwrap();
        imported.save(&import_path).unwrap();

        import_accounts(&config, &config_path, &import_path, false).unwrap();

        let merged = AccountState::load_or_default(&state_path).unwrap();
        assert_eq!(merged.active_account.as_deref(), Some("primary"));
        assert_eq!(
            merged.codex_account_by_name("primary").unwrap().api_key,
            "new-key"
        );
        assert_eq!(
            merged
                .account_by_name_and_provider("claude-main", crate::provider::CLAUDE_PROVIDER)
                .unwrap()
                .api_key,
            "claude-key"
        );

        let _ = fs::remove_file(config_path);
        let _ = fs::remove_file(state_path);
        let _ = fs::remove_file(import_path);
    }

    #[tokio::test]
    async fn refresh_codex_account_uses_configured_refresh_endpoint() {
        use axum::{Json, Router, routing::post};
        use serde_json::json;
        use std::sync::{
            Arc,
            atomic::{AtomicUsize, Ordering},
        };
        use tokio::net::TcpListener;

        async fn refresh_handler() -> impl axum::response::IntoResponse {
            Json(json!({
                "access_token": "refreshed-access",
                "refresh_token": "refreshed-refresh",
                "id_token": "TEST_ID_TOKEN_PLACEHOLDER",
                "expires_in": 3600
            }))
        }

        let refresh_hits = Arc::new(AtomicUsize::new(0));
        let refresh_hits_for_handler = refresh_hits.clone();
        let refresh_app = Router::new().route(
            "/oauth/token",
            post(move || {
                let refresh_hits_for_handler = refresh_hits_for_handler.clone();
                async move {
                    refresh_hits_for_handler.fetch_add(1, Ordering::SeqCst);
                    refresh_handler().await
                }
            }),
        );
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let refresh_addr = listener.local_addr().unwrap();
        let server = tokio::spawn(async move {
            axum::serve(listener, refresh_app).await.unwrap();
        });

        let (config_path, state_path) = temp_paths("refresh-endpoint");
        let mut config = AppConfig::default();
        config.state.path = state_path.display().to_string();
        config.codex.refresh_token_url = format!("http://{refresh_addr}/oauth/token");
        fs::write(&config_path, AppConfig::example_toml()).unwrap();

        let mut state = AccountState::default();
        state
            .add_device_codex_account(
                "primary".to_string(),
                crate::codex::DeviceAuthResult {
                    access_token: "old-access".to_string(),
                    refresh_token: "old-refresh".to_string(),
                    id_token: "old-id".to_string(),
                    email: Some("refresh@example.com".to_string()),
                    account_id: Some("acct".to_string()),
                    plan_type: Some("pro".to_string()),
                    expires_at: Some("1970-01-01T00:00:00Z".to_string()),
                },
                true,
            )
            .unwrap();
        state.save(&state_path).unwrap();

        refresh_codex_account(&config, &config_path, Some("primary".to_string()))
            .await
            .unwrap();

        assert_eq!(refresh_hits.load(Ordering::SeqCst), 1);
        let updated = AccountState::load_or_default(&state_path).unwrap();
        let refreshed = updated.codex_account_by_name("primary").unwrap();
        assert_eq!(refreshed.api_key, "refreshed-access");
        assert_eq!(
            refreshed.refresh_token.as_deref(),
            Some("refreshed-refresh")
        );
        // The placeholder id_token does not encode an exp claim, so expires_at
        // is derived from the current time + expires_in rather than a fixed value.
        assert!(
            refreshed.expires_at.is_some(),
            "expires_at should be set after refresh"
        );

        server.abort();
        let _ = fs::remove_file(config_path);
        let _ = fs::remove_file(state_path);
    }
}
