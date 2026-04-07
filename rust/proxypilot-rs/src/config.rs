use std::fs;
use std::path::{Path, PathBuf};

use anyhow::{Context, Result, bail};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct AppConfig {
    #[serde(default)]
    pub server: ServerConfig,
    #[serde(default)]
    pub state: StateConfig,
    #[serde(default)]
    pub codex: CodexConfig,
    #[serde(default)]
    pub providers: ProvidersConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct ProvidersConfig {
    #[serde(default)]
    pub active: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_bind")]
    pub bind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateConfig {
    #[serde(default = "default_state_path")]
    pub path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CodexConfig {
    #[serde(default = "default_codex_base_url")]
    pub upstream_base_url: String,
    #[serde(default)]
    pub api_key: String,
    #[serde(default)]
    pub refresh_token_url: String,
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            bind: default_bind(),
        }
    }
}

impl Default for StateConfig {
    fn default() -> Self {
        Self {
            path: default_state_path(),
        }
    }
}

impl Default for CodexConfig {
    fn default() -> Self {
        Self {
            upstream_base_url: default_codex_base_url(),
            api_key: "set-me".to_string(),
            refresh_token_url: String::new(),
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
        if self.state.path.trim().is_empty() {
            bail!("state.path cannot be empty");
        }
        Ok(())
    }

    pub fn health_url(&self) -> String {
        format!("http://{}/healthz", self.server.bind)
    }

    pub fn config_summary(&self, path: &Path) -> Vec<String> {
        let mut lines = vec![
            format!("config: {}", path.display()),
            format!("listen: {}", self.server.bind),
            format!("state path: {}", self.resolve_state_path(path).display()),
            format!(
                "active provider: {}",
                self.active_provider_label()
            ),
            format!("codex upstream: {}", self.codex.upstream_base_url),
            format!(
                "codex refresh token endpoint: {}",
                if self.codex.refresh_token_url.trim().is_empty() {
                    "live default".to_string()
                } else {
                    self.codex.refresh_token_url.clone()
                }
            ),
            format!(
                "codex fallback api key: {}",
                if self.codex.api_key.trim().is_empty() {
                    "missing"
                } else {
                    "configured"
                }
            ),
        ];
        lines
    }

    pub fn active_provider(&self) -> &str {
        self.providers
            .active
            .as_deref()
            .unwrap_or(crate::provider::CODEX_PROVIDER)
    }

    pub fn active_provider_label(&self) -> String {
        let tag = self.active_provider();
        match tag {
            crate::provider::CODEX_PROVIDER => "codex (default)".to_string(),
            other => other.to_string(),
        }
    }

    pub fn example_toml() -> &'static str {
        r#"# ProxyPilot Rust replatform config
#
# The rewrite keeps long-lived account state in a separate local file.

[server]
bind = "127.0.0.1:8318"

[state]
path = "proxypilot-rs.state.toml"

[providers]
# active = "codex"  # default; change to "claude" or "gemini" when available

[codex]
upstream_base_url = "https://api.openai.com"
api_key = ""
# refresh_token_url = "http://127.0.0.1:18319/oauth/token"
"#
    }

    pub fn resolve_state_path(&self, config_path: &Path) -> PathBuf {
        let raw = PathBuf::from(self.state.path.trim());
        if raw.is_absolute() {
            raw
        } else if let Some(parent) = config_path.parent() {
            parent.join(raw)
        } else {
            raw
        }
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

fn default_state_path() -> String {
    "proxypilot-rs.state.toml".to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_example_config() {
        let config: AppConfig = toml::from_str(AppConfig::example_toml()).unwrap();
        assert_eq!(config.server.bind, "127.0.0.1:8318");
        assert_eq!(config.state.path, "proxypilot-rs.state.toml");
        assert_eq!(config.codex.upstream_base_url, "https://api.openai.com");
        assert!(config.codex.refresh_token_url.is_empty());
    }
}
