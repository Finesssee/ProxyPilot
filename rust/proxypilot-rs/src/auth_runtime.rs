use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

pub const EXPIRING_SOON_WINDOW_SECS: i64 = 300;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthHealthState {
    Valid,
    ExpiringSoon,
    Expired,
    Static,
    Unknown,
}

impl AuthHealthState {
    pub fn label(self) -> &'static str {
        match self {
            Self::Valid => "valid",
            Self::ExpiringSoon => "expiring soon",
            Self::Expired => "expired",
            Self::Static => "static",
            Self::Unknown => "unknown",
        }
    }

    pub fn is_refresh_worthy(self) -> bool {
        matches!(self, Self::ExpiringSoon | Self::Expired)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthCredentialSource {
    ActiveAccount,
    ConfigFallbackKey,
    NoCredential,
}

impl AuthCredentialSource {
    pub fn label(self) -> &'static str {
        match self {
            Self::ActiveAccount => "saved account",
            Self::ConfigFallbackKey => "config fallback key",
            Self::NoCredential => "no credential",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthMetadataState {
    Present,
    Missing,
    Malformed,
}

impl AuthMetadataState {
    pub fn label(self) -> &'static str {
        match self {
            Self::Present => "present",
            Self::Missing => "missing",
            Self::Malformed => "malformed",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuthHealthSnapshot {
    pub state: AuthHealthState,
    pub source: AuthCredentialSource,
    pub metadata_state: AuthMetadataState,
    pub expires_at: Option<String>,
    pub expires_at_unix_secs: Option<i64>,
    pub expires_in_secs: Option<i64>,
}

impl AuthHealthSnapshot {
    pub fn summary_label(&self) -> &'static str {
        self.state.label()
    }

    pub fn source_label(&self) -> &'static str {
        self.source.label()
    }

    pub fn expiry_detail(&self) -> String {
        match self.state {
            AuthHealthState::Valid => self
                .expires_at
                .as_deref()
                .map(|value| format!("valid until {}", value))
                .unwrap_or_else(|| "valid".to_string()),
            AuthHealthState::ExpiringSoon => self
                .expires_at
                .as_deref()
                .map(|value| format!("expiring soon at {}", value))
                .unwrap_or_else(|| "expiring soon".to_string()),
            AuthHealthState::Expired => self
                .expires_at
                .as_deref()
                .map(|value| format!("expired at {}", value))
                .unwrap_or_else(|| "expired".to_string()),
            AuthHealthState::Static => match self.source {
                AuthCredentialSource::ConfigFallbackKey => "config fallback key".to_string(),
                _ => "static access token".to_string(),
            },
            AuthHealthState::Unknown => match self.metadata_state {
                AuthMetadataState::Malformed => self
                    .expires_at
                    .as_deref()
                    .map(|value| format!("malformed expiry metadata ({value})"))
                    .unwrap_or_else(|| "malformed expiry metadata".to_string()),
                AuthMetadataState::Missing => "expiry metadata missing".to_string(),
                AuthMetadataState::Present => "auth state unknown".to_string(),
            },
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "snake_case")]
pub enum RefreshStatusKind {
    Success,
    Failure,
    Skipped,
    #[default]
    Unknown,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct RefreshStatusSnapshot {
    pub kind: RefreshStatusKind,
    pub account_name: Option<String>,
    pub occurred_at_unix_secs: Option<i64>,
    pub message: Option<String>,
}

impl RefreshStatusSnapshot {
    pub fn success(account_name: impl Into<String>, occurred_at_unix_secs: i64) -> Self {
        Self {
            kind: RefreshStatusKind::Success,
            account_name: Some(account_name.into()),
            occurred_at_unix_secs: Some(occurred_at_unix_secs),
            message: None,
        }
    }

    pub fn failure(
        account_name: Option<impl Into<String>>,
        occurred_at_unix_secs: i64,
        message: impl Into<String>,
    ) -> Self {
        Self {
            kind: RefreshStatusKind::Failure,
            account_name: account_name.map(Into::into),
            occurred_at_unix_secs: Some(occurred_at_unix_secs),
            message: Some(message.into()),
        }
    }

    pub fn skipped(message: impl Into<String>) -> Self {
        Self {
            kind: RefreshStatusKind::Skipped,
            account_name: None,
            occurred_at_unix_secs: None,
            message: Some(message.into()),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct RuntimeRequestCounters {
    pub total_proxied_requests: u64,
    pub successful_upstream_responses: u64,
    pub auth_refresh_attempts: u64,
    pub auth_refresh_failures: u64,
    pub upstream_401_count: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct RuntimeStatsSnapshot {
    pub bind_address: String,
    pub upstream_base_url: String,
    pub active_account_name: Option<String>,
    pub account_count: usize,
    pub auth_health: AuthHealthSnapshot,
    pub request_counters: RuntimeRequestCounters,
    pub last_refresh: RefreshStatusSnapshot,
}

impl RuntimeStatsSnapshot {
    pub fn new(
        bind_address: impl Into<String>,
        upstream_base_url: impl Into<String>,
        active_account_name: Option<String>,
        account_count: usize,
        auth_health: AuthHealthSnapshot,
    ) -> Self {
        Self {
            bind_address: bind_address.into(),
            upstream_base_url: upstream_base_url.into(),
            active_account_name,
            account_count,
            auth_health,
            request_counters: RuntimeRequestCounters::default(),
            last_refresh: RefreshStatusSnapshot::default(),
        }
    }
}

pub fn now_unix_secs() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs() as i64)
        .unwrap_or_default()
}

pub fn parse_rfc3339_z(value: &str) -> Option<i64> {
    if value.len() != 20 || !value.ends_with('Z') {
        return None;
    }

    if value.as_bytes().get(4) != Some(&b'-')
        || value.as_bytes().get(7) != Some(&b'-')
        || value.as_bytes().get(10) != Some(&b'T')
        || value.as_bytes().get(13) != Some(&b':')
        || value.as_bytes().get(16) != Some(&b':')
    {
        return None;
    }

    let year = value.get(0..4)?.parse::<i64>().ok()?;
    let month = value.get(5..7)?.parse::<i64>().ok()?;
    let day = value.get(8..10)?.parse::<i64>().ok()?;
    let hour = value.get(11..13)?.parse::<i64>().ok()?;
    let minute = value.get(14..16)?.parse::<i64>().ok()?;
    let second = value.get(17..19)?.parse::<i64>().ok()?;

    if !(1..=12).contains(&month)
        || !(1..=31).contains(&day)
        || !(0..=23).contains(&hour)
        || !(0..=59).contains(&minute)
        || !(0..=59).contains(&second)
    {
        return None;
    }

    if day > i64::from(days_in_month(year, month as u32)?) {
        return None;
    }

    let adjusted_year = year - if month <= 2 { 1 } else { 0 };
    let era = if adjusted_year >= 0 {
        adjusted_year
    } else {
        adjusted_year - 399
    } / 400;
    let yoe = adjusted_year - era * 400;
    let month_prime = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * month_prime + 2) / 5 + day - 1;
    if !(0..=365).contains(&doy) {
        return None;
    }
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    let days = era * 146_097 + doe - 719_468;
    Some(days * 86_400 + hour * 3_600 + minute * 60 + second)
}

fn days_in_month(year: i64, month: u32) -> Option<u8> {
    Some(match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if is_leap_year(year) => 29,
        2 => 28,
        _ => return None,
    })
}

fn is_leap_year(year: i64) -> bool {
    let year = year.rem_euclid(400);
    year % 4 == 0 && (year % 100 != 0 || year == 0)
}

pub fn evaluate_auth_health(
    source: AuthCredentialSource,
    refresh_token: Option<&str>,
    expires_at: Option<&str>,
    now_unix_secs: i64,
) -> AuthHealthSnapshot {
    let expires_at = normalize_optional(expires_at);
    let refresh_token = normalize_optional(refresh_token);

    match source {
        AuthCredentialSource::ConfigFallbackKey => AuthHealthSnapshot {
            state: AuthHealthState::Static,
            source,
            metadata_state: if expires_at.is_some() {
                AuthMetadataState::Present
            } else {
                AuthMetadataState::Missing
            },
            expires_at,
            expires_at_unix_secs: None,
            expires_in_secs: None,
        },
        AuthCredentialSource::NoCredential => AuthHealthSnapshot {
            state: AuthHealthState::Unknown,
            source,
            metadata_state: if expires_at.is_some() {
                AuthMetadataState::Present
            } else {
                AuthMetadataState::Missing
            },
            expires_at: expires_at.clone(),
            expires_at_unix_secs: expires_at.as_deref().and_then(parse_rfc3339_z),
            expires_in_secs: None,
        },
        AuthCredentialSource::ActiveAccount => {
            if refresh_token.is_none() {
                return AuthHealthSnapshot {
                    state: AuthHealthState::Static,
                    source,
                    metadata_state: if expires_at.is_some() {
                        AuthMetadataState::Present
                    } else {
                        AuthMetadataState::Missing
                    },
                    expires_at: expires_at.clone(),
                    expires_at_unix_secs: expires_at.as_deref().and_then(parse_rfc3339_z),
                    expires_in_secs: None,
                };
            }

            let Some(expires_at_text) = expires_at.clone() else {
                return AuthHealthSnapshot {
                    state: AuthHealthState::Unknown,
                    source,
                    metadata_state: AuthMetadataState::Missing,
                    expires_at: None,
                    expires_at_unix_secs: None,
                    expires_in_secs: None,
                };
            };

            let Some(expires_at_unix_secs) = parse_rfc3339_z(&expires_at_text) else {
                return AuthHealthSnapshot {
                    state: AuthHealthState::Unknown,
                    source,
                    metadata_state: AuthMetadataState::Malformed,
                    expires_at: Some(expires_at_text),
                    expires_at_unix_secs: None,
                    expires_in_secs: None,
                };
            };

            let expires_in_secs = expires_at_unix_secs - now_unix_secs;
            let state = if expires_in_secs <= 0 {
                AuthHealthState::Expired
            } else if expires_in_secs <= EXPIRING_SOON_WINDOW_SECS {
                AuthHealthState::ExpiringSoon
            } else {
                AuthHealthState::Valid
            };

            AuthHealthSnapshot {
                state,
                source,
                metadata_state: AuthMetadataState::Present,
                expires_at: Some(expires_at_text),
                expires_at_unix_secs: Some(expires_at_unix_secs),
                expires_in_secs: Some(expires_in_secs),
            }
        }
    }
}

fn normalize_optional(value: Option<&str>) -> Option<String> {
    value
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(ToOwned::to_owned)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_rfc3339_z_timestamps() {
        assert_eq!(parse_rfc3339_z("1970-01-01T00:00:00Z"), Some(0));
        assert_eq!(parse_rfc3339_z("1970-01-02T00:00:00Z"), Some(86_400));
    }

    #[test]
    fn rejects_non_rfc3339_z_timestamps() {
        assert_eq!(parse_rfc3339_z("2026-04-05 12:00:00"), None);
        assert_eq!(parse_rfc3339_z("2026-13-05T12:00:00Z"), None);
        assert_eq!(parse_rfc3339_z("2026-04-31T12:00:00Z"), None);
        assert_eq!(parse_rfc3339_z("2023-02-29T12:00:00Z"), None);
    }

    #[test]
    fn accepts_valid_leap_day_timestamps() {
        assert_eq!(parse_rfc3339_z("2024-02-29T12:00:00Z"), Some(1_709_208_000));
        assert_eq!(parse_rfc3339_z("2000-02-29T23:59:59Z"), Some(951_868_799));
    }

    #[test]
    fn classifies_active_account_auth_health_states() {
        let now = 1_700_000_000;
        let valid = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            Some("refresh"),
            Some("2023-11-15T12:00:30Z"),
            now,
        );
        assert_eq!(valid.state, AuthHealthState::Valid);
        assert_eq!(valid.summary_label(), "valid");

        let expiring_soon = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            Some("refresh"),
            Some("2023-11-14T22:14:05Z"),
            now,
        );
        assert_eq!(expiring_soon.state, AuthHealthState::ExpiringSoon);
        assert!(expiring_soon.state.is_refresh_worthy());

        let expired = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            Some("refresh"),
            Some("2023-11-14T22:07:00Z"),
            now,
        );
        assert_eq!(expired.state, AuthHealthState::Expired);
        assert!(expired.state.is_refresh_worthy());
    }

    #[test]
    fn classifies_static_fallback_and_malformed_metadata_explicitly() {
        let static_fallback = evaluate_auth_health(
            AuthCredentialSource::ConfigFallbackKey,
            None,
            None,
            1_700_000_000,
        );
        assert_eq!(static_fallback.state, AuthHealthState::Static);
        assert_eq!(static_fallback.source_label(), "config fallback key");

        let static_saved_token = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            None,
            Some("2026-04-06T00:00:00Z"),
            1_700_000_000,
        );
        assert_eq!(static_saved_token.state, AuthHealthState::Static);

        let malformed = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            Some("refresh"),
            Some("not-a-timestamp"),
            1_700_000_000,
        );
        assert_eq!(malformed.state, AuthHealthState::Unknown);
        assert_eq!(malformed.metadata_state, AuthMetadataState::Malformed);
        assert!(
            malformed
                .expiry_detail()
                .contains("malformed expiry metadata")
        );

        let impossible_calendar_date = evaluate_auth_health(
            AuthCredentialSource::ActiveAccount,
            Some("refresh"),
            Some("2023-02-29T12:00:00Z"),
            1_700_000_000,
        );
        assert_eq!(impossible_calendar_date.state, AuthHealthState::Unknown);
        assert_eq!(
            impossible_calendar_date.metadata_state,
            AuthMetadataState::Malformed
        );
    }

    #[test]
    fn runtime_stats_snapshot_serializes_with_stable_fields() {
        let snapshot = RuntimeStatsSnapshot::new(
            "127.0.0.1:8318",
            "https://upstream.example/v1",
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

        let payload = serde_json::to_value(snapshot).unwrap();
        assert_eq!(payload["bind_address"], "127.0.0.1:8318");
        assert_eq!(payload["auth_health"]["state"], "valid");
        assert_eq!(payload["request_counters"]["total_proxied_requests"], 0);
        assert_eq!(payload["last_refresh"]["kind"], "unknown");
    }
}
