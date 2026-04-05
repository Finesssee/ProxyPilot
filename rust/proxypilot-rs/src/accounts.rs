use std::fs;
use std::path::Path;

use anyhow::{Context, Result};

use crate::codex;
use crate::config::AppConfig;
use crate::state::{AccountState, ImportedCodexAuth};

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

    if state.active_account.as_deref() == Some(name.as_str()) {
        println!("saved Codex account `{}` and marked it active", name);
    } else {
        println!("saved Codex account `{}`", name);
    }
    println!("state file: {}", state_path.display());
    Ok(())
}

pub fn list_accounts(config: &AppConfig, config_path: &Path) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let state = AccountState::load_or_default(&state_path)?;

    if state.accounts.is_empty() {
        println!("No local Rust accounts saved yet.");
        println!("state file: {}", state_path.display());
        return Ok(());
    }

    println!("ProxyPilot Rust accounts");
    println!("state file: {}", state_path.display());
    println!();
    for account in &state.accounts {
        let marker = if state.active_account.as_deref() == Some(account.name.as_str()) {
            "*"
        } else {
            "-"
        };
        let email = account.email.as_deref().unwrap_or("-");
        let source = account.source.as_deref().unwrap_or("-");
        println!(
            "{marker} {:<16} provider={} email={} source={}",
            account.name, account.provider, email, source
        );
    }

    Ok(())
}

pub fn activate_account(config: &AppConfig, config_path: &Path, name: String) -> Result<()> {
    let state_path = config.resolve_state_path(config_path);
    let mut state = AccountState::load_or_default(&state_path)?;
    state.activate(&name)?;
    state.save(&state_path)?;
    println!("active account set to `{}`", name);
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

    if state.active_account.as_deref() == Some(resolved_name.as_str()) {
        println!(
            "imported Codex auth `{}` and marked it active",
            resolved_name
        );
    } else {
        println!("imported Codex auth `{}`", resolved_name);
    }
    println!("source file: {}", auth_file.display());
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

    if state.active_account.as_deref() == Some(resolved_name.as_str()) {
        println!(
            "saved Codex device login `{}` and marked it active",
            resolved_name
        );
    } else {
        println!("saved Codex device login `{}`", resolved_name);
    }
    println!("state file: {}", state_path.display());
    Ok(())
}
