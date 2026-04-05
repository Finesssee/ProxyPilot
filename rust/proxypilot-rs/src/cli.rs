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
}
