# sdk/ - Reusable SDK Packages

Potentially reusable outside ProxyPilot. Minimal internal/ dependencies.

## PACKAGE MAP

| Package | Purpose | Key Files |
|---------|---------|-----------|
| `translator/` | API format translation (OpenAI↔Claude↔Gemini) | registry.go, validation.go |
| `auth/` | Token store interfaces, persistence | token_store.go, file_store.go |
| `api/handlers/` | Base API handler, provider handlers | openai/, claude/, gemini/ |
| `access/` | Credential access abstraction | provider.go |
| `cliproxy/` | CLI proxy auth helpers | auth/round_robin.go |
| `config/` | Shared config utilities | config.go |
| `logging/` | Request logging interface | request_logger.go |

## WHEN TO USE

| Need | Use |
|------|-----|
| Format translation | `sdk/translator/` |
| Token persistence | `sdk/auth/` |
| API handler base | `sdk/api/handlers/` |
| Credential selection | `sdk/cliproxy/auth/` |
| App-specific logic | `internal/` (not sdk) |

## KEY INTERFACES

```go
// sdk/auth/token_store.go
type TokenStore interface {
    Load(filename string) ([]byte, error)
    Save(filename string, data []byte) error
}

// sdk/translator/types.go  
type Translator interface {
    CanTranslate(from, to Format) bool
    Translate(ctx, request) (response, error)
}
```
