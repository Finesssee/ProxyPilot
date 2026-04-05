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

use crate::auth_runtime::{self, AuthCredentialSource, AuthHealthSnapshot, RuntimeStatsSnapshot};
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
            app.poll_runtime_observability().await;
        }

        terminal.draw(|frame| render(frame, &app))?;

        if event::poll(Duration::from_millis(150))?
            && let Event::Key(key) = event::read()?
        {
            match key.code {
                KeyCode::Char('q') => break,
                KeyCode::Char('r') => app.refresh_operator_state().await,
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
    runtime_stats: Option<RuntimeStatsSnapshot>,
    runtime_error: Option<String>,
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
            runtime_stats: None,
            runtime_error: None,
            accounts: Vec::new(),
            selected_account_idx: 0,
            active_account: "none".to_string(),
            account_count: 0,
            models: Vec::new(),
            feedback: "Ready.".to_string(),
            feedback_color: Color::DarkGray,
            last_poll: Instant::now() - Duration::from_secs(3),
        }
    }

    async fn poll_runtime_observability(&mut self) -> bool {
        self.last_poll = Instant::now();
        self.refresh_accounts();

        match reqwest::get(self.config.health_url()).await {
            Ok(response) if response.status() == StatusCode::OK => {
                self.health_status = "local proxy reachable".to_string();
                self.health_color = Color::Green;
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

        self.refresh_runtime_stats().await;
        self.runtime_stats.is_some()
    }

    async fn refresh_operator_state(&mut self) {
        let proxy_reachable = self.poll_runtime_observability().await;
        if proxy_reachable {
            self.refresh_models().await;
        }
    }

    async fn refresh_runtime_stats(&mut self) {
        let runtime_url = format!("http://{}/v0/runtime/stats", self.config.server.bind);
        match reqwest::get(runtime_url).await {
            Ok(response) if response.status() == StatusCode::OK => {
                match response.json::<RuntimeStatsSnapshot>().await {
                    Ok(snapshot) => {
                        self.runtime_error = None;
                        self.runtime_stats = Some(snapshot);
                    }
                    Err(err) => {
                        self.runtime_stats = None;
                        self.runtime_error = Some(format!("failed to decode runtime stats: {err}"));
                    }
                }
            }
            Ok(response) => {
                self.runtime_stats = None;
                self.runtime_error = Some(format!(
                    "runtime stats endpoint returned {}",
                    response.status()
                ));
            }
            Err(err) => {
                self.runtime_stats = None;
                self.runtime_error = Some(format!("runtime stats unavailable: {err}"));
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
        match runtime_refresh_target(
            self.runtime_stats.as_ref(),
            self.runtime_error.as_deref(),
            &self.active_account,
        ) {
            Ok(active_runtime_account) => {
                self.refresh_account_named(active_runtime_account).await;
            }
            Err(message) => {
                self.feedback = message;
                self.feedback_color = Color::Yellow;
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
            let refresh_token = target.refresh_token.as_deref().ok_or_else(|| {
                anyhow::anyhow!(
                    "account `{}` is static and has no refresh token",
                    target.name
                )
            })?;
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
                self.poll_runtime_observability().await;
            }
            Err(err) => {
                self.feedback = format!("Failed to refresh account `{}`: {err}", account_name);
                self.feedback_color = Color::Red;
            }
        }
    }
}

fn render(frame: &mut ratatui::Frame<'_>, app: &TuiApp) {
    let footer = footer_widget(app);
    let footer_height = footer_render_height(app, frame.area().width).max(6);

    let outer_chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([Constraint::Min(1), Constraint::Length(footer_height)])
        .split(frame.area());

    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(4),
            Constraint::Min(6),
            Constraint::Min(7),
            Constraint::Min(3),
            Constraint::Min(7),
        ])
        .split(outer_chunks[0]);

    let title = Paragraph::new(vec![
        Line::from(Span::styled(
            "Codex Operator Console",
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        )),
        Line::from("Terminal-first Rust proxy slice with live runtime stats and disk-backed account control."),
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
        Line::from(format!("Disk active account: {}", app.active_account)),
        Line::from(format!("Bind: {}", app.config.server.bind)),
        Line::from(format!("Upstream: {}", app.config.codex.upstream_base_url)),
        Line::from(format!("Saved accounts: {}", app.account_count)),
        Line::from(format!("Config: {}", app.config_path)),
    ])
    .block(Block::default().borders(Borders::ALL).title("Runtime"));

    let runtime_lines = runtime_panel_lines(
        app.runtime_stats.as_ref(),
        app.runtime_error.as_deref(),
        &app.active_account,
    )
    .into_iter()
    .map(Line::from)
    .collect::<Vec<_>>();
    let runtime = Paragraph::new(runtime_lines)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .title("Runtime Stats"),
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

    frame.render_widget(title, chunks[0]);
    frame.render_widget(health, chunks[1]);
    frame.render_widget(runtime, chunks[2]);
    frame.render_widget(models, chunks[3]);
    frame.render_widget(accounts, chunks[4]);
    frame.render_widget(footer, outer_chunks[1]);
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
            "static"
        }
    }

    fn expiry_badge(&self) -> &'static str {
        self.auth_health.summary_label()
    }

    fn expiry_detail(&self) -> String {
        self.auth_health.expiry_detail()
    }
}

fn runtime_panel_lines(
    runtime_stats: Option<&RuntimeStatsSnapshot>,
    runtime_error: Option<&str>,
    disk_active_account: &str,
) -> Vec<String> {
    match runtime_stats {
        Some(stats) => vec![
            "Runtime stats: live".to_string(),
            format!(
                "Runtime active account: {}",
                stats.active_account_name.as_deref().unwrap_or("none")
            ),
            format!("Local disk active account: {}", disk_active_account),
            format!("Runtime-usable Codex accounts: {}", stats.account_count),
            format!("Auth state: {}", stats.auth_health.summary_label()),
            format!(
                "Request counters: total={} success={} 401={} refresh_attempts={} refresh_failures={}",
                stats.request_counters.total_proxied_requests,
                stats.request_counters.successful_upstream_responses,
                stats.request_counters.upstream_401_count,
                stats.request_counters.auth_refresh_attempts,
                stats.request_counters.auth_refresh_failures,
            ),
            format!(
                "Last refresh: {}",
                refresh_status_summary(&stats.last_refresh)
            ),
            format!("Bind: {}", stats.bind_address),
            format!("Upstream: {}", stats.upstream_base_url),
        ],
        None => vec![
            "Runtime unavailable".to_string(),
            runtime_error
                .map(ToOwned::to_owned)
                .unwrap_or_else(|| "Runtime stats endpoint is unavailable.".to_string()),
            format!("Local disk active account: {}", disk_active_account),
            "Local disk-backed account list is still available.".to_string(),
        ],
    }
}

fn refresh_status_summary(status: &crate::auth_runtime::RefreshStatusSnapshot) -> String {
    let mut parts = vec![match status.kind {
        crate::auth_runtime::RefreshStatusKind::Success => "success".to_string(),
        crate::auth_runtime::RefreshStatusKind::Failure => "failure".to_string(),
        crate::auth_runtime::RefreshStatusKind::Skipped => "skipped".to_string(),
        crate::auth_runtime::RefreshStatusKind::Unknown => "unknown".to_string(),
    }];
    if let Some(account) = status.account_name.as_deref() {
        parts.push(format!("for `{account}`"));
    }
    if let Some(occurred_at_unix_secs) = status.occurred_at_unix_secs {
        parts.push(format!("at {occurred_at_unix_secs}"));
    }
    if let Some(message) = status.message.as_deref() {
        parts.push(format!("- {message}"));
    }
    parts.join(" ")
}

fn footer_widget(app: &TuiApp) -> Paragraph<'_> {
    Paragraph::new(vec![
        Line::from(vec![Span::styled(
            app.feedback.as_str(),
            Style::default().fg(app.feedback_color),
        )]),
        Line::from(footer_status_hint(
            app.accounts.get(app.selected_account_idx),
            app.runtime_stats.is_some(),
        )),
        Line::from(footer_help_text_primary()),
        Line::from(footer_help_text_secondary()),
    ])
    .block(Block::default().borders(Borders::ALL).title("Keys"))
    .wrap(Wrap { trim: false })
}

fn footer_render_height(app: &TuiApp, total_width: u16) -> u16 {
    let content_width = total_width.saturating_sub(4).max(1);
    footer_content_lines(app)
        .into_iter()
        .map(|line| wrapped_line_count(&line, content_width))
        .sum::<usize>()
        .saturating_add(2)
        .min(u16::MAX as usize) as u16
}

fn footer_content_lines(app: &TuiApp) -> Vec<String> {
    vec![
        app.feedback.clone(),
        footer_status_hint(
            app.accounts.get(app.selected_account_idx),
            app.runtime_stats.is_some(),
        ),
        footer_help_text_primary().to_string(),
        footer_help_text_secondary().to_string(),
    ]
}

fn wrapped_line_count(text: &str, width: u16) -> usize {
    let width = width.max(1) as usize;
    if text.trim().is_empty() {
        return 1;
    }

    let mut lines = 1usize;
    let mut current_width = 0usize;

    for word in text.split_whitespace() {
        let word_width = word.chars().count();
        if word_width > width {
            if current_width != 0 {
                lines += 1;
            }
            let mut remaining = word_width;
            while remaining > width {
                lines += 1;
                remaining -= width;
            }
            current_width = remaining;
            continue;
        }

        let separator = usize::from(current_width != 0);
        if current_width + separator + word_width > width {
            lines += 1;
            current_width = word_width;
        } else {
            current_width += separator + word_width;
        }
    }

    lines
}

fn footer_help_text_primary() -> &'static str {
    "q quit  |  r reload/poll  |  arrows move  |  a activate  |  f refresh selected"
}

fn footer_help_text_secondary() -> &'static str {
    "R refresh active  |  d delete  |  c clear feedback"
}

fn runtime_action_hint(runtime_available: bool) -> &'static str {
    if runtime_available {
        "runtime live"
    } else {
        "runtime unavailable"
    }
}

fn account_selection_hint(selected: Option<&AccountRow>) -> String {
    match selected {
        Some(account) => format!(
            "Selected account: {} ({})",
            account.name,
            account.token_mode()
        ),
        None => "Selected account: none".to_string(),
    }
}

fn footer_status_hint(selected: Option<&AccountRow>, runtime_available: bool) -> String {
    format!(
        "{}  |  {}",
        account_selection_hint(selected),
        runtime_action_hint(runtime_available)
    )
}

fn runtime_refresh_target(
    runtime_stats: Option<&RuntimeStatsSnapshot>,
    runtime_error: Option<&str>,
    disk_active_account: &str,
) -> std::result::Result<String, String> {
    let disk_active_account = normalized_active_account_label(disk_active_account);

    match runtime_stats
        .and_then(|stats| stats.active_account_name.as_deref())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        Some(active_runtime_account) => Ok(active_runtime_account.to_string()),
        None => Err(match runtime_stats {
            Some(_) => format!(
                "Runtime active account unavailable; runtime stats do not report a live active account. Disk active account remains `{}`.",
                disk_active_account
            ),
            None => format!(
                "Runtime active account unavailable; {}. Disk active account remains `{}`.",
                runtime_error.unwrap_or("runtime stats are unavailable"),
                disk_active_account
            ),
        }),
    }
}

fn normalized_active_account_label(value: &str) -> String {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        "none".to_string()
    } else {
        trimmed.to_string()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::auth_runtime::{
        AuthCredentialSource, AuthHealthSnapshot, AuthHealthState, AuthMetadataState,
        RefreshStatusKind, RefreshStatusSnapshot, RuntimeRequestCounters, RuntimeStatsSnapshot,
    };

    fn sample_runtime_stats() -> RuntimeStatsSnapshot {
        let mut snapshot = RuntimeStatsSnapshot::new(
            "127.0.0.1:8318",
            "https://api.openai.com",
            Some("primary".to_string()),
            2,
            AuthHealthSnapshot {
                state: AuthHealthState::Valid,
                source: AuthCredentialSource::ActiveAccount,
                metadata_state: AuthMetadataState::Present,
                expires_at: Some("2026-04-06T00:00:00Z".to_string()),
                expires_at_unix_secs: Some(1_765_440_000),
                expires_in_secs: Some(86_400),
            },
        );
        snapshot.request_counters = RuntimeRequestCounters {
            total_proxied_requests: 7,
            successful_upstream_responses: 6,
            auth_refresh_attempts: 2,
            auth_refresh_failures: 1,
            upstream_401_count: 1,
        };
        snapshot.last_refresh = RefreshStatusSnapshot {
            kind: RefreshStatusKind::Success,
            account_name: Some("primary".to_string()),
            occurred_at_unix_secs: Some(1_765_440_123),
            message: Some("refreshed after expiry".to_string()),
        };
        snapshot
    }

    #[test]
    fn runtime_panel_lines_include_counters_and_last_refresh_outcome() {
        let lines = runtime_panel_lines(Some(&sample_runtime_stats()), None, "primary");
        let joined = lines.join("\n");

        assert!(joined.contains("Runtime stats: live"));
        assert!(joined.contains("Runtime active account: primary"));
        assert!(joined.contains("Local disk active account: primary"));
        assert!(joined.contains("Runtime-usable Codex accounts: 2"));
        assert!(!joined.contains("Saved accounts on disk: 2"));
        assert!(joined.contains("Auth state: valid"));
        assert!(joined.contains("Request counters: total=7"));
        assert!(joined.contains("Last refresh: success for `primary`"));
        assert!(joined.contains("refreshed after expiry"));
    }

    #[test]
    fn runtime_panel_lines_show_unavailable_copy_when_runtime_is_down() {
        let lines = runtime_panel_lines(None, Some("connect refused"), "primary");
        let joined = lines.join("\n");

        assert!(joined.contains("Runtime unavailable"));
        assert!(joined.contains("connect refused"));
        assert!(joined.contains("Local disk active account: primary"));
        assert!(joined.contains("Local disk-backed account list is still available."));
    }

    #[test]
    fn footer_help_text_matches_milestone_actions() {
        let help_primary = footer_help_text_primary();
        let help_secondary = footer_help_text_secondary();

        assert!(help_primary.contains("q quit"));
        assert!(help_primary.contains("r reload"));
        assert!(help_primary.contains("a activate"));
        assert!(help_primary.contains("f refresh selected"));
        assert!(help_secondary.contains("R refresh active"));
        assert!(help_secondary.contains("d delete"));
        assert!(help_secondary.contains("c clear feedback"));
    }

    #[test]
    fn runtime_action_hint_distinguishes_runtime_status() {
        assert_eq!(runtime_action_hint(true), "runtime live");
        assert_eq!(runtime_action_hint(false), "runtime unavailable");
    }

    #[test]
    fn account_selection_hint_reports_selected_account_mode() {
        let account = AccountRow {
            name: "primary".to_string(),
            email: "-".to_string(),
            account_id: "-".to_string(),
            plan_type: "-".to_string(),
            source: "manual".to_string(),
            is_active: false,
            auth_health: AuthHealthSnapshot {
                state: AuthHealthState::Static,
                source: AuthCredentialSource::ActiveAccount,
                metadata_state: AuthMetadataState::Missing,
                expires_at: None,
                expires_at_unix_secs: None,
                expires_in_secs: None,
            },
            can_refresh: false,
        };

        assert_eq!(
            account_selection_hint(Some(&account)),
            "Selected account: primary (static)"
        );
        assert_eq!(account_selection_hint(None), "Selected account: none");
    }

    #[test]
    fn runtime_refresh_target_prefers_runtime_active_account_over_disk_state() {
        let target =
            runtime_refresh_target(Some(&sample_runtime_stats()), None, "disk-selected").unwrap();

        assert_eq!(target, "primary");
    }

    #[test]
    fn runtime_refresh_target_reports_truthful_failure_when_runtime_is_unavailable() {
        let message =
            runtime_refresh_target(None, Some("connect refused"), "disk-selected").unwrap_err();

        assert!(message.contains("Runtime active account unavailable"));
        assert!(message.contains("connect refused"));
        assert!(message.contains("disk-selected"));
    }

    #[test]
    fn runtime_refresh_target_reports_missing_live_active_account_truthfully() {
        let mut stats = sample_runtime_stats();
        stats.active_account_name = None;

        let message = runtime_refresh_target(Some(&stats), None, "disk-selected").unwrap_err();

        assert!(message.contains("do not report a live active account"));
        assert!(message.contains("disk-selected"));
    }

    #[tokio::test]
    async fn passive_polling_does_not_touch_models_endpoint() {
        use std::sync::{
            Arc,
            atomic::{AtomicUsize, Ordering},
        };

        use axum::{Json, Router, routing::get};
        use serde_json::json;
        use tokio::net::TcpListener;

        let health_hits = Arc::new(AtomicUsize::new(0));
        let stats_hits = Arc::new(AtomicUsize::new(0));
        let models_hits = Arc::new(AtomicUsize::new(0));

        let health_hits_for_handler = health_hits.clone();
        let stats_hits_for_handler = stats_hits.clone();
        let models_hits_for_handler = models_hits.clone();

        let app = Router::new()
            .route(
                "/healthz",
                get(move || {
                    let health_hits_for_handler = health_hits_for_handler.clone();
                    async move {
                        health_hits_for_handler.fetch_add(1, Ordering::SeqCst);
                        StatusCode::OK
                    }
                }),
            )
            .route(
                "/v0/runtime/stats",
                get(move || {
                    let stats_hits_for_handler = stats_hits_for_handler.clone();
                    async move {
                        stats_hits_for_handler.fetch_add(1, Ordering::SeqCst);
                        Json(sample_runtime_stats())
                    }
                }),
            )
            .route(
                "/v1/models",
                get(move || {
                    let models_hits_for_handler = models_hits_for_handler.clone();
                    async move {
                        models_hits_for_handler.fetch_add(1, Ordering::SeqCst);
                        Json(json!({
                            "data": [{"id": "model-1"}]
                        }))
                    }
                }),
            );

        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let bind = listener.local_addr().unwrap();
        let server = tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });

        let config = AppConfig {
            server: crate::config::ServerConfig {
                bind: bind.to_string(),
            },
            ..AppConfig::default()
        };
        let mut app = TuiApp::new(config, Path::new("/tmp/proxypilot-tui-test.toml"));
        app.models = vec!["existing-model".to_string()];

        app.poll_runtime_observability().await;

        assert_eq!(health_hits.load(Ordering::SeqCst), 1);
        assert_eq!(stats_hits.load(Ordering::SeqCst), 1);
        assert_eq!(models_hits.load(Ordering::SeqCst), 0);
        assert_eq!(app.models, vec!["existing-model".to_string()]);

        server.abort();
    }

    #[test]
    fn footer_layout_snapshot_keeps_all_guidance_visible() {
        use ratatui::Terminal;
        use ratatui::backend::TestBackend;

        let mut app = TuiApp::new(
            AppConfig::default(),
            Path::new("/tmp/proxypilot-tui-test.toml"),
        );
        app.feedback = "Failed to refresh account `primary`: Runtime active account unavailable; runtime stats do not report a live active account. Disk active account remains `primary`.".to_string();
        app.feedback_color = Color::Green;

        let backend = TestBackend::new(120, 36);
        let mut terminal = Terminal::new(backend).unwrap();

        terminal.draw(|frame| render(frame, &app)).unwrap();

        let snapshot = terminal
            .backend()
            .buffer()
            .content()
            .chunks(terminal.backend().buffer().area.width as usize)
            .map(|row| row.iter().map(|cell| cell.symbol()).collect::<String>())
            .collect::<Vec<_>>()
            .join("\n");

        assert!(snapshot.contains("Failed to refresh account `primary`:"));
        assert!(snapshot.contains("Runtime active account unavailable;"));
        assert!(snapshot.contains("runtime stats do not report a live active"));
        assert!(snapshot.contains("account. Disk active account remains `primary`."));
        assert!(snapshot.contains("Disk active account remains `primary`."));
        assert!(snapshot.contains("Selected account: none"));
        assert!(snapshot.contains("runtime live") || snapshot.contains("runtime unavailable"));
        assert!(snapshot.contains("q quit"));
        assert!(snapshot.contains("R refresh active"));
        assert!(snapshot.contains("c clear feedback"));
        assert!(!snapshot.contains("Use arrows to select an account"));
        assert!(footer_render_height(&app, 120) >= 7);
    }
}
