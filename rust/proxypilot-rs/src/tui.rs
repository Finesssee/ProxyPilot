use std::io;
use std::path::Path;
use std::time::{Duration, Instant};

use anyhow::Result;
use crossterm::event::{self, Event, KeyCode};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;
use ratatui::layout::{Constraint, Direction, Layout};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Paragraph, Wrap};
use reqwest::StatusCode;
use serde::Deserialize;

use crate::auth_runtime::{self, AuthCredentialSource, AuthHealthSnapshot};
use crate::codex;
use crate::config::AppConfig;
use crate::state::AccountState;

pub async fn run(config: AppConfig, config_path: &Path) -> Result<()> {
    enable_raw_mode()?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen)?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend)?;

    let result = run_loop(&mut terminal, config, config_path).await;

    disable_raw_mode()?;
    execute!(terminal.backend_mut(), LeaveAlternateScreen)?;
    terminal.show_cursor()?;

    result
}

async fn run_loop(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    config: AppConfig,
    config_path: &Path,
) -> Result<()> {
    let mut app = TuiApp::new(config, config_path);

    loop {
        if app.last_poll.elapsed() >= Duration::from_secs(2) {
            app.refresh_health().await;
        }

        terminal.draw(|frame| render(frame, &app))?;

        if event::poll(Duration::from_millis(150))?
            && let Event::Key(key) = event::read()?
        {
            match key.code {
                KeyCode::Char('q') => break,
                KeyCode::Char('r') => app.refresh_health().await,
                KeyCode::Up => app.move_selection_up(),
                KeyCode::Down => app.move_selection_down(),
                KeyCode::Char('a') => app.activate_selected_account(),
                KeyCode::Char('d') => app.delete_selected_account(),
                KeyCode::Char('f') => app.refresh_selected_account().await,
                KeyCode::Char('R') => app.refresh_active_account().await,
                KeyCode::Char('c') => app.clear_feedback(),
                _ => {}
            }
        }
    }

    Ok(())
}

struct TuiApp {
    config: AppConfig,
    config_path: String,
    health_status: String,
    health_color: Color,
    accounts: Vec<AccountRow>,
    selected_account_idx: usize,
    active_account: String,
    account_count: usize,
    models: Vec<String>,
    feedback: String,
    feedback_color: Color,
    last_poll: Instant,
}

impl TuiApp {
    fn new(config: AppConfig, config_path: &Path) -> Self {
        Self {
            config,
            config_path: config_path.display().to_string(),
            health_status: "waiting for first health check".to_string(),
            health_color: Color::Yellow,
            accounts: Vec::new(),
            selected_account_idx: 0,
            active_account: "none".to_string(),
            account_count: 0,
            models: Vec::new(),
            feedback: "Use arrows to select an account, `a` to activate, `f` to refresh selected, `R` to refresh active, `d` to delete, `c` to clear.".to_string(),
            feedback_color: Color::DarkGray,
            last_poll: Instant::now() - Duration::from_secs(3),
        }
    }

    async fn refresh_health(&mut self) {
        self.last_poll = Instant::now();
        self.refresh_accounts();

        match reqwest::get(self.config.health_url()).await {
            Ok(response) if response.status() == StatusCode::OK => {
                self.health_status = "local proxy reachable".to_string();
                self.health_color = Color::Green;
                self.refresh_models().await;
            }
            Ok(response) => {
                self.health_status = format!("proxy responded with {}", response.status());
                self.health_color = Color::Yellow;
                self.models.clear();
            }
            Err(err) => {
                self.health_status = format!("proxy not running yet: {err}");
                self.health_color = Color::Red;
                self.models.clear();
            }
        }
    }

    fn refresh_accounts(&mut self) {
        let state_path = self.config.resolve_state_path(Path::new(&self.config_path));
        match AccountState::load_or_default(&state_path) {
            Ok(state) => {
                self.account_count = state.accounts.len();
                self.active_account = state
                    .active_codex_account()
                    .map(|account| account.name)
                    .unwrap_or_else(|| "none".to_string());
                self.accounts = state
                    .accounts
                    .iter()
                    .filter(|account| account.provider == "codex")
                    .map(|account| AccountRow {
                        name: account.name.clone(),
                        email: account.email.clone().unwrap_or_else(|| "-".to_string()),
                        account_id: account
                            .account_id
                            .clone()
                            .unwrap_or_else(|| "-".to_string()),
                        plan_type: account.plan_type.clone().unwrap_or_else(|| "-".to_string()),
                        source: account.source.clone().unwrap_or_else(|| "-".to_string()),
                        is_active: state.active_account.as_deref() == Some(account.name.as_str()),
                        auth_health: auth_runtime::evaluate_auth_health(
                            AuthCredentialSource::ActiveAccount,
                            account.refresh_token.as_deref(),
                            account.expires_at.as_deref(),
                            auth_runtime::now_unix_secs(),
                        ),
                        can_refresh: account
                            .refresh_token
                            .as_deref()
                            .map(|value| !value.trim().is_empty())
                            .unwrap_or(false),
                    })
                    .collect();

                if self.accounts.is_empty() {
                    self.selected_account_idx = 0;
                } else if self.selected_account_idx >= self.accounts.len() {
                    self.selected_account_idx = self.accounts.len() - 1;
                }
            }
            Err(err) => {
                self.account_count = 0;
                self.active_account = format!("state error: {err}");
                self.accounts.clear();
            }
        }
    }

    async fn refresh_models(&mut self) {
        let models_url = format!("http://{}/v1/models", self.config.server.bind);
        match reqwest::get(models_url).await {
            Ok(response) if response.status() == StatusCode::OK => {
                match response.json::<ModelsResponse>().await {
                    Ok(payload) => {
                        self.models = payload
                            .data
                            .into_iter()
                            .map(|entry| entry.id)
                            .take(8)
                            .collect();
                    }
                    Err(err) => {
                        self.models = vec![format!("failed to parse models response: {err}")];
                    }
                }
            }
            Ok(response) => {
                self.models = vec![format!("models endpoint returned {}", response.status())];
            }
            Err(err) => {
                self.models = vec![format!("failed to fetch models: {err}")];
            }
        }
    }

    fn move_selection_up(&mut self) {
        if self.accounts.is_empty() {
            return;
        }
        if self.selected_account_idx == 0 {
            self.selected_account_idx = self.accounts.len() - 1;
        } else {
            self.selected_account_idx -= 1;
        }
    }

    fn move_selection_down(&mut self) {
        if self.accounts.is_empty() {
            return;
        }
        self.selected_account_idx = (self.selected_account_idx + 1) % self.accounts.len();
    }

    fn activate_selected_account(&mut self) {
        let Some(selected) = self.accounts.get(self.selected_account_idx) else {
            self.feedback = "No saved Codex account to activate.".to_string();
            self.feedback_color = Color::Yellow;
            return;
        };

        let state_path = self.config.resolve_state_path(Path::new(&self.config_path));
        match AccountState::load_or_default(&state_path).and_then(|mut state| {
            state.activate(&selected.name)?;
            state.save(&state_path)?;
            Ok(())
        }) {
            Ok(()) => {
                self.feedback = format!("Activated account `{}`.", selected.name);
                self.feedback_color = Color::Green;
                self.refresh_accounts();
            }
            Err(err) => {
                self.feedback = format!("Failed to activate account: {err}");
                self.feedback_color = Color::Red;
            }
        }
    }

    fn delete_selected_account(&mut self) {
        let Some(selected) = self.accounts.get(self.selected_account_idx) else {
            self.feedback = "No saved Codex account to delete.".to_string();
            self.feedback_color = Color::Yellow;
            return;
        };

        let state_path = self.config.resolve_state_path(Path::new(&self.config_path));
        match AccountState::load_or_default(&state_path).and_then(|mut state| {
            state.remove_account(&selected.name)?;
            state.save(&state_path)?;
            Ok(())
        }) {
            Ok(()) => {
                self.feedback = format!("Deleted account `{}`.", selected.name);
                self.feedback_color = Color::Green;
                self.refresh_accounts();
            }
            Err(err) => {
                self.feedback = format!("Failed to delete account: {err}");
                self.feedback_color = Color::Red;
            }
        }
    }

    async fn refresh_selected_account(&mut self) {
        let Some(selected) = self.accounts.get(self.selected_account_idx) else {
            self.feedback = "No saved Codex account to refresh.".to_string();
            self.feedback_color = Color::Yellow;
            return;
        };

        self.refresh_account_named(selected.name.clone()).await;
    }

    async fn refresh_active_account(&mut self) {
        let state_path = self.config.resolve_state_path(Path::new(&self.config_path));
        match AccountState::load_or_default(&state_path) {
            Ok(state) => {
                if let Some(active) = state.active_codex_account() {
                    self.refresh_account_named(active.name).await;
                } else {
                    self.feedback = "No active Codex account to refresh.".to_string();
                    self.feedback_color = Color::Yellow;
                }
            }
            Err(err) => {
                self.feedback = format!("Failed to read account state: {err}");
                self.feedback_color = Color::Red;
            }
        }
    }

    fn clear_feedback(&mut self) {
        self.feedback.clear();
        self.feedback_color = Color::DarkGray;
    }

    async fn refresh_account_named(&mut self, account_name: String) {
        let state_path = self.config.resolve_state_path(Path::new(&self.config_path));
        let result = async {
            let mut state = AccountState::load_or_default(&state_path)?;
            let target = state
                .codex_account_by_name(&account_name)
                .ok_or_else(|| anyhow::anyhow!("account `{}` no longer exists", account_name))?;
            let refresh_token = target
                .refresh_token
                .as_deref()
                .ok_or_else(|| anyhow::anyhow!("account `{}` has no refresh token", target.name))?;
            let refreshed = codex::refresh_with_refresh_token(refresh_token).await?;
            state.update_codex_account_tokens(&target.name, refreshed)?;
            state.save(&state_path)?;
            Result::<(), anyhow::Error>::Ok(())
        }
        .await;

        match result {
            Ok(()) => {
                self.feedback = format!("Refreshed account `{}`.", account_name);
                self.feedback_color = Color::Green;
                self.refresh_accounts();
                self.refresh_health().await;
            }
            Err(err) => {
                self.feedback = format!("Failed to refresh account `{}`: {err}", account_name);
                self.feedback_color = Color::Red;
            }
        }
    }
}

fn render(frame: &mut ratatui::Frame<'_>, app: &TuiApp) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([
            Constraint::Length(4),
            Constraint::Length(8),
            Constraint::Length(10),
            Constraint::Length(12),
            Constraint::Min(7),
            Constraint::Length(3),
        ])
        .split(frame.area());

    let title = Paragraph::new(vec![
        Line::from(Span::styled(
            "ProxyPilot Rust Replatform",
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        )),
        Line::from("Terminal-first rewrite branch with a real Codex proxy slice."),
    ])
    .block(Block::default().borders(Borders::ALL).title("Overview"));

    let health = Paragraph::new(vec![
        Line::from(vec![
            Span::raw("Health: "),
            Span::styled(
                app.health_status.as_str(),
                Style::default()
                    .fg(app.health_color)
                    .add_modifier(Modifier::BOLD),
            ),
        ]),
        Line::from(format!("Bind: {}", app.config.server.bind)),
        Line::from(format!("Upstream: {}", app.config.codex.upstream_base_url)),
        Line::from(format!("Active account: {}", app.active_account)),
        Line::from(format!("Saved accounts: {}", app.account_count)),
        Line::from(format!("Config: {}", app.config_path)),
    ])
    .block(Block::default().borders(Borders::ALL).title("Runtime"));

    let summary_lines = app
        .config
        .config_summary(Path::new(&app.config_path))
        .into_iter()
        .map(Line::from)
        .collect::<Vec<_>>();
    let summary = Paragraph::new(summary_lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .title("Config Summary"),
        )
        .wrap(Wrap { trim: false });

    let model_lines = if app.models.is_empty() {
        vec![Line::from(
            "No models loaded yet. Start the proxy and press r.",
        )]
    } else {
        app.models
            .iter()
            .map(|model| Line::from(model.as_str()))
            .collect()
    };
    let models = Paragraph::new(model_lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .title("Available Models"),
        )
        .wrap(Wrap { trim: false });

    let account_lines = if app.accounts.is_empty() {
        vec![Line::from(
            "No saved Codex accounts yet. Use the CLI account commands first.",
        )]
    } else {
        app.accounts
            .iter()
            .enumerate()
            .map(|(idx, account)| {
                let selector = if idx == app.selected_account_idx {
                    ">"
                } else {
                    " "
                };
                let active = if account.is_active { "*" } else { " " };
                let health = account.auth_health.summary_label();
                Line::from(format!(
                    "{}{} {:<14} {:<20} {:<14} {:<9} {}",
                    selector,
                    active,
                    account.name,
                    account.email,
                    health,
                    account.source,
                    account.expiry_badge()
                ))
            })
            .collect()
    };
    let selected_lines = if let Some(selected) = app.accounts.get(app.selected_account_idx) {
        vec![
            Line::from(format!("Name: {}", selected.name)),
            Line::from(format!("Email: {}", selected.email)),
            Line::from(format!("Account ID: {}", selected.account_id)),
            Line::from(format!("Plan: {}", selected.plan_type)),
            Line::from(format!(
                "Auth health: {}",
                selected.auth_health.summary_label()
            )),
            Line::from(format!(
                "Auth source: {}",
                selected.auth_health.source_label()
            )),
            Line::from(format!("Refreshability: {}", selected.token_mode())),
            Line::from(format!("Expiry: {}", selected.expiry_detail())),
            Line::from(format!("Source: {}", selected.source)),
        ]
    } else {
        vec![
            Line::from("No Codex account selected yet."),
            Line::from("Save or import an account, then reopen or press r."),
        ]
    };
    let mut account_lines = account_lines;
    account_lines.push(Line::from(""));
    account_lines.push(Line::from(Span::styled(
        "Selected Account",
        Style::default()
            .fg(Color::Cyan)
            .add_modifier(Modifier::BOLD),
    )));
    account_lines.extend(selected_lines);

    let accounts = Paragraph::new(account_lines)
        .block(Block::default().borders(Borders::ALL).title("Accounts"))
        .wrap(Wrap { trim: false });

    let footer = Paragraph::new(vec![
        Line::from(vec![Span::styled(
            app.feedback.as_str(),
            Style::default().fg(app.feedback_color),
        )]),
        Line::from(
            "q quit  |  r reload  |  arrows move  |  a activate  |  f refresh selected  |  R refresh active  |  d delete  |  c clear feedback",
        ),
    ])
    .block(Block::default().borders(Borders::ALL).title("Keys"));

    frame.render_widget(title, chunks[0]);
    frame.render_widget(health, chunks[1]);
    frame.render_widget(models, chunks[2]);
    frame.render_widget(accounts, chunks[3]);
    frame.render_widget(summary, chunks[4]);
    frame.render_widget(footer, chunks[5]);
}

#[derive(Debug, Deserialize)]
struct ModelsResponse {
    data: Vec<ModelEntry>,
}

#[derive(Debug, Deserialize)]
struct ModelEntry {
    id: String,
}

struct AccountRow {
    name: String,
    email: String,
    account_id: String,
    plan_type: String,
    source: String,
    is_active: bool,
    auth_health: AuthHealthSnapshot,
    can_refresh: bool,
}

impl AccountRow {
    fn token_mode(&self) -> &'static str {
        if self.can_refresh {
            "refreshable"
        } else {
            "static access token"
        }
    }

    fn expiry_badge(&self) -> &'static str {
        self.auth_health.summary_label()
    }

    fn expiry_detail(&self) -> String {
        self.auth_health.expiry_detail()
    }
}
