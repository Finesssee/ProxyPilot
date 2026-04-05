use std::path::PathBuf;

use anyhow::Result;
use clap::Parser;
use proxypilot_rs::cli::{Cli, Command};
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
            proxypilot_rs::proxy::run(config).await?;
        }
        Command::Tui { config } => {
            let path = resolve_config_path(config);
            let config = AppConfig::load(&path)?;
            proxypilot_rs::tui::run(config, &path).await?;
        }
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
