use std::path::PathBuf;

use clap::{Parser, Subcommand};

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
}
