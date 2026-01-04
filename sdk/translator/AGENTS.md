# sdk/translator/ - API Format Translation

Core translation engine: OpenAI ↔ Anthropic (Claude) ↔ Gemini format conversion.

## FILE MAP

| File | Purpose |
|------|---------|
| `registry.go` | Translator registration, lookup, pipeline building |
| `api.go` | Public translation API |
| `types.go` | Format enum, Translator interface |
| `validation.go` | Request/response validation |
| `pipeline.go` | Multi-step translation chains |
| `detect.go` | Auto-detect request format |
| `cache.go` | Translation result caching |
| `resilience.go` | Retry logic, fallbacks |
| `middleware.go` | Request/response middleware hooks |
| `pool.go` | Translator instance pooling |
| `quality.go` | Translation quality metrics |
| `builtin/` | Built-in translator implementations |

## HOW TO ADD

1. Create translator in `builtin/` or separate package
2. Implement `Translator` interface:
   ```go
   type Translator interface {
       CanTranslate(from, to Format) bool
       Translate(ctx context.Context, req *Request) (*Response, error)
   }
   ```
3. Register in `registry.go`:
   ```go
   Register("openai-to-claude", NewOpenAIToClaudeTranslator())
   ```
4. Add tests in `*_test.go`

## TESTING

```bash
go test ./sdk/translator/...
```

Test patterns:
- `*_test.go` files per component
- `testing.go` has test helpers and fixtures
- Benchmark tests in `benchmark_test.go`

## FORMATS

| Format | Constant |
|--------|----------|
| OpenAI | `FormatOpenAI` |
| Claude (Anthropic) | `FormatClaude` |
| Gemini | `FormatGemini` |
| Codex | `FormatCodex` |
