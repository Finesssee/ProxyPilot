use std::path::PathBuf;

use anyhow::Result;
use clap::Parser;
use proxypilot_rs::cli::{AccountCommand, Cli, Command};
use proxypilot_rs::config::{AppConfig, default_config_path};
use tracing_subscriber::EnvFilter;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new("proxypilot_rs=info,tower_http=info")),
        )
        .with_target(false)
        .compact()
        .init();

    let cli = Cli::parse();

    match cli.command {
        Command::Init { config, force } => {
            let path = resolve_config_path(config);
            AppConfig::write_example(&path, force)?;
            println!("wrote {}", path.display());
        }
        Command::Run { config } => {
            let path = resolve_config_path(config);
            let config = AppConfig::load(&path)?;
            proxypilot_rs::proxy::run(config, &path).await?;
        }
        Command::Tui { config } => {
            let path = resolve_config_path(config);
            let config = AppConfig::load(&path)?;
            proxypilot_rs::tui::run(config, &path).await?;
        }
        Command::Account { command, .. } => match command {
            AccountCommand::AddCodex {
                shared,
                name,
                api_key,
                activate,
            } => {
                let path = resolve_config_path(shared.config);
                let config = AppConfig::load(&path)?;
                proxypilot_rs::accounts::add_codex_account(
                    &config, &path, name, api_key, activate,
                )?;
            }
            AccountCommand::List { shared } => {
                let path = resolve_config_path(shared.config);
                let config = AppConfig::load(&path)?;
                proxypilot_rs::accounts::list_accounts(&config, &path)?;
            }
            AccountCommand::Activate { shared, name } => {
                let path = resolve_config_path(shared.config);
                let config = AppConfig::load(&path)?;
                proxypilot_rs::accounts::activate_account(&config, &path, name)?;
            }
        },
    }

    Ok(())
}

fn resolve_config_path(path: PathBuf) -> PathBuf {
    if path.as_os_str().is_empty() {
        default_config_path()
    } else {
        path
    }
}
