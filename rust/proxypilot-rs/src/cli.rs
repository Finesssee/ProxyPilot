use std::path::PathBuf;

use clap::{Args, Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "proxypilot-rs")]
#[command(about = "Experimental terminal-first Rust replatform for ProxyPilot")]
#[command(version)]
pub struct Cli {
    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    /// Write an example config to disk.
    Init {
        #[arg(long, default_value = "proxypilot-rs.toml")]
        config: PathBuf,
        #[arg(long, default_value_t = false)]
        force: bool,
    },
    /// Run the local proxy server.
    Run {
        #[arg(long, default_value = "proxypilot-rs.toml")]
        config: PathBuf,
    },
    /// Open a terminal operator view for the Rust rewrite.
    Tui {
        #[arg(long, default_value = "proxypilot-rs.toml")]
        config: PathBuf,
    },
    /// Manage local account state for the Rust rewrite.
    Account {
        #[command(flatten)]
        shared: SharedConfig,
        #[command(subcommand)]
        command: AccountCommand,
    },
}

#[derive(Debug, Clone, Args)]
pub struct SharedConfig {
    #[arg(long, default_value = "proxypilot-rs.toml")]
    pub config: PathBuf,
}

#[derive(Debug, Subcommand)]
pub enum AccountCommand {
    /// Add or replace a Codex account in the local state file.
    AddCodex {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: String,
        #[arg(long)]
        api_key: String,
        #[arg(long, default_value_t = false)]
        activate: bool,
    },
    /// Add or replace a Claude account in the local state file.
    AddClaude {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: String,
        #[arg(long)]
        api_key: String,
        #[arg(long, default_value_t = false)]
        activate: bool,
    },
    /// Import a Codex auth JSON file from the Go/ProxyPilot world.
    ImportCodex {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        file: PathBuf,
        #[arg(long)]
        name: Option<String>,
        #[arg(long, default_value_t = false)]
        activate: bool,
    },
    /// Start the Codex device flow and save the resulting account locally.
    LoginCodexDevice {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: Option<String>,
        #[arg(long, default_value_t = false)]
        activate: bool,
    },
    /// Refresh a saved Codex account using its refresh token.
    RefreshCodex {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: Option<String>,
    },
    /// Refresh a saved Claude account.
    RefreshClaude {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: Option<String>,
    },
    /// List saved accounts and show which one is active.
    List {
        #[command(flatten)]
        shared: SharedConfig,
    },
    /// Select the active account used by the proxy.
    Activate {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: String,
    },
    /// Remove a saved account from the local state file.
    Remove {
        #[command(flatten)]
        shared: SharedConfig,
        #[arg(long)]
        name: String,
    },
}
