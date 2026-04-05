use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result, bail};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AppConfig {
    #[serde(default)]
    pub server: ServerConfig,
    #[serde(default)]
    pub codex: CodexConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_bind")]
    pub bind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CodexConfig {
    #[serde(default = "default_codex_base_url")]
    pub upstream_base_url: String,
    #[serde(default)]
    pub api_key: String,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            server: ServerConfig::default(),
            codex: CodexConfig::default(),
        }
    }
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            bind: default_bind(),
        }
    }
}

impl Default for CodexConfig {
    fn default() -> Self {
        Self {
            upstream_base_url: default_codex_base_url(),
            api_key: "set-me".to_string(),
        }
    }
}

impl AppConfig {
    pub fn load(path: &Path) -> Result<Self> {
        let raw = fs::read_to_string(path)
            .with_context(|| format!("failed to read config at {}", path.display()))?;
        let config: Self = toml::from_str(&raw)
            .with_context(|| format!("failed to parse TOML config at {}", path.display()))?;
        config.validate()?;
        Ok(config)
    }

    pub fn write_example(path: &Path, force: bool) -> Result<()> {
        if path.exists() && !force {
            bail!(
                "config already exists at {} (pass --force to overwrite)",
                path.display()
            );
        }

        if let Some(parent) = path.parent()
            && !parent.as_os_str().is_empty()
        {
            fs::create_dir_all(parent).with_context(|| {
                format!("failed to create config directory {}", parent.display())
            })?;
        }

        fs::write(path, Self::example_toml())
            .with_context(|| format!("failed to write example config to {}", path.display()))?;
        Ok(())
    }

    pub fn validate(&self) -> Result<()> {
        if self.server.bind.trim().is_empty() {
            bail!("server.bind cannot be empty");
        }
        if self.codex.upstream_base_url.trim().is_empty() {
            bail!("codex.upstream_base_url cannot be empty");
        }
        Ok(())
    }

    pub fn health_url(&self) -> String {
        format!("http://{}/healthz", self.server.bind)
    }

    pub fn config_summary(&self, path: &Path) -> Vec<String> {
        vec![
            format!("config: {}", path.display()),
            format!("listen: {}", self.server.bind),
            format!("codex upstream: {}", self.codex.upstream_base_url),
            format!(
                "codex api key: {}",
                if self.codex.api_key.trim().is_empty() {
                    "missing"
                } else {
                    "configured"
                }
            ),
        ]
    }

    pub fn example_toml() -> &'static str {
        r#"# ProxyPilot Rust replatform config
#
# This first milestone keeps the config deliberately small:
# one local bind address and one Codex-compatible upstream.

[server]
bind = "127.0.0.1:8318"

[codex]
upstream_base_url = "https://api.openai.com"
api_key = "set-me"
"#
    }
}

pub fn default_config_path() -> PathBuf {
    PathBuf::from("proxypilot-rs.toml")
}

fn default_bind() -> String {
    "127.0.0.1:8318".to_string()
}

fn default_codex_base_url() -> String {
    "https://api.openai.com".to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_example_config() {
        let config: AppConfig = toml::from_str(AppConfig::example_toml()).unwrap();
        assert_eq!(config.server.bind, "127.0.0.1:8318");
        assert_eq!(config.codex.upstream_base_url, "https://api.openai.com");
    }
}
