# Provider Migration: iFlow → MiniMax + Zhipu AI

**Date:** 2025-12-28
**Version:** 0.1.6

## Summary

Removed the iFlow provider (complex OAuth/cookie authentication) and replaced it with two new direct API providers:
- **MiniMax** - `https://api.minimax.io/v1` (OpenAI-compatible, API key auth)
- **Zhipu AI** - `https://open.bigmodel.cn/api/paas/v4/` (OpenAI-compatible, API key auth)

Both new providers are significantly simpler than iFlow because they use API keys instead of OAuth.

---

## Changes Overview

### Providers Before
| Provider | Auth Method |
|----------|-------------|
| Claude | OAuth2 |
| Codex | OAuth2 |
| Gemini | OAuth2 |
| Gemini CLI | OAuth2 |
| Kiro | OAuth2 + AWS SSO |
| Qwen | OAuth2 |
| iFlow | OAuth + Cookie |
| **Total: 7** | |

### Providers After
| Provider | Auth Method |
|----------|-------------|
| Claude | OAuth2 |
| Codex | OAuth2 |
| Gemini | OAuth2 |
| Gemini CLI | OAuth2 |
| Kiro | OAuth2 + AWS SSO |
| Qwen | OAuth2 |
| MiniMax | API Key |
| Zhipu AI | API Key |
| **Total: 8** | |

---

## Files Deleted (iFlow Removal)

```
internal/auth/iflow/iflow_auth.go
internal/auth/iflow/iflow_token.go
internal/auth/iflow/oauth_server.go
internal/auth/iflow/cookie_helpers.go
internal/cmd/iflow_login.go
internal/cmd/iflow_cookie.go
internal/runtime/executor/iflow_executor.go
internal/api/handlers/management/iflow_import.go
internal/api/handlers/management/iflow_import_test.go
sdk/auth/iflow.go
```

---

## Files Created (New Providers)

### MiniMax Provider
```
internal/runtime/executor/minimax_executor.go   # OpenAI-compatible executor
sdk/auth/minimax.go                              # API key authenticator
```

### Zhipu AI Provider
```
internal/runtime/executor/zhipu_executor.go     # OpenAI-compatible executor
sdk/auth/zhipu.go                                # API key authenticator
```

---

## Files Modified

### Core Registration Files
| File | Changes |
|------|---------|
| `sdk/auth/refresh_registry.go` | Removed iFlow, added MiniMax + Zhipu registrations |
| `sdk/cliproxy/service.go` | Removed iFlow executor/model cases, added MiniMax + Zhipu |
| `internal/registry/model_definitions.go` | Removed `GetIFlowModels()`, added `GetMiniMaxModels()` + `GetZhipuModels()` |
| `internal/cmd/auth_manager.go` | Removed iFlow authenticator, added MiniMax + Zhipu |

### API/Server Files
| File | Changes |
|------|---------|
| `internal/api/server.go` | Removed iFlow OAuth routes |
| `internal/api/handlers/management/auth_files.go` | Removed iFlow import, removed `RequestIFlowToken` + `RequestIFlowCookieToken` handlers |
| `internal/api/handlers/management/oauth_sessions.go` | Removed iFlow case from provider normalization |
| `internal/api/handlers/management/generic_import.go` | Removed iFlow import, added `importResult` type definition |

### CLI Files
| File | Changes |
|------|---------|
| `cmd/server/main.go` | Removed `--iflow-login` and `--iflow-cookie` flags |
| `cmd/proxypilotui/main_windows.go` | Removed iFlow button and handler |
| `cmd/cliproxytray/main_windows.go` | Removed iFlow endpoint case |

### UI Files
| File | Changes |
|------|---------|
| `webui/src/components/layout/StatusBar.tsx` | Removed iFlow from provider list |
| `webui/src/components/dashboard/ProviderLogins.tsx` | Removed iFlow from provider list and auth detection |

### Documentation
| File | Changes |
|------|---------|
| `README.md` | Updated provider count (7→8), updated provider table |
| `docs/proxypilot.md` | Removed iFlow from one-click login list |
| `config.example.yaml` | Removed iFlow example |
| `cmd/installer/bundle/config.example.yaml` | Removed iFlow example |

---

## New Model Definitions

### MiniMax Models
| Model ID | Display Name | Description |
|----------|--------------|-------------|
| `minimax-m2` | MiniMax-M2 | Base model |
| `minimax-m2.1` | MiniMax-M2.1 | With reasoning support |
| `minimax-m2.1-lightning` | MiniMax-M2.1-Lightning | Fast variant |

### Zhipu AI Models
| Model ID | Display Name | Description |
|----------|--------------|-------------|
| `glm-4.7` | GLM-4.7 | Flagship model with thinking |
| `glm-4.6` | GLM-4.6 | High performance |
| `glm-4.5` | GLM-4.5 | Excellent for coding |
| `glm-4-long` | GLM-4-Long | 1M context window |
| `glm-4.6v` | GLM-4.6V | Vision model |

---

## API Details

### MiniMax
- **Base URL:** `https://api.minimax.io/v1`
- **Auth:** `Authorization: Bearer <API_KEY>`
- **Format:** OpenAI-compatible
- **Thinking:** M2.1 models support reasoning via `reasoning_split=true`

### Zhipu AI
- **Base URL:** `https://open.bigmodel.cn/api/paas/v4/`
- **Auth:** `Authorization: Bearer <API_KEY>`
- **Format:** OpenAI-compatible
- **Thinking:** GLM-4.6/4.7 support via `extra_body={"thinking": {"type": "enabled"}}`

---

## Auth File Format

Both providers use simple JSON with API key (no OAuth tokens needed):

```json
{
  "type": "minimax",
  "api_key": "sk-xxx",
  "label": "my-account",
  "created_at": "2025-12-28T12:00:00Z"
}
```

```json
{
  "type": "zhipu",
  "api_key": "xxx.yyy",
  "label": "my-account",
  "created_at": "2025-12-28T12:00:00Z"
}
```

---

## Usage

### Adding MiniMax API Key
Create a file `auth/minimax-myaccount.json`:
```json
{
  "type": "minimax",
  "api_key": "YOUR_MINIMAX_API_KEY",
  "label": "myaccount"
}
```

### Adding Zhipu AI API Key
Create a file `auth/zhipu-myaccount.json`:
```json
{
  "type": "zhipu",
  "api_key": "YOUR_ZHIPU_API_KEY",
  "label": "myaccount"
}
```

---

## Build Verification

```bash
go build ./...
# ✓ Build successful
```

---

## Migration Notes

1. **iFlow users:** If you have existing iFlow auth files, they will no longer work. Remove them from your `auth/` directory.

2. **Model mapping:** If you had model mappings pointing to iFlow models, update them to use the new providers:
   - `glm-4.6` → Use Zhipu provider directly
   - `glm-4.7` → Use Zhipu provider directly
   - `minimax-m2` → Use MiniMax provider directly

3. **API keys:** Both MiniMax and Zhipu require API keys from their respective platforms:
   - MiniMax: https://www.minimax.io/
   - Zhipu AI: https://open.bigmodel.cn/
