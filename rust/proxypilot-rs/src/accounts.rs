use std::path::Path;

use anyhow::Result;

use crate::config::AppConfig;
use crate::state::AccountState;

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
        println!(
            "{marker} {:<16} provider={}",
            account.name, account.provider
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
