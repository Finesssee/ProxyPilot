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

use crate::config::AppConfig;

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
    models: Vec<String>,
    last_poll: Instant,
}

impl TuiApp {
    fn new(config: AppConfig, config_path: &Path) -> Self {
        Self {
            config,
            config_path: config_path.display().to_string(),
            health_status: "waiting for first health check".to_string(),
            health_color: Color::Yellow,
            models: Vec::new(),
            last_poll: Instant::now() - Duration::from_secs(3),
        }
    }

    async fn refresh_health(&mut self) {
        self.last_poll = Instant::now();

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
}

fn render(frame: &mut ratatui::Frame<'_>, app: &TuiApp) {
    let chunks = Layout::default()
        .direction(Direction::Vertical)
        .margin(1)
        .constraints([
            Constraint::Length(4),
            Constraint::Length(8),
            Constraint::Length(10),
            Constraint::Min(8),
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

    let footer = Paragraph::new("q quit  |  r refresh health  |  run the server in another terminal with `proxypilot-rs run`")
        .block(Block::default().borders(Borders::ALL).title("Keys"));

    frame.render_widget(title, chunks[0]);
    frame.render_widget(health, chunks[1]);
    frame.render_widget(models, chunks[2]);
    frame.render_widget(summary, chunks[3]);
    frame.render_widget(footer, chunks[4]);
}

#[derive(Debug, Deserialize)]
struct ModelsResponse {
    data: Vec<ModelEntry>,
}

#[derive(Debug, Deserialize)]
struct ModelEntry {
    id: String,
}
