# Hindsight (Optional) Agent Memory Sidecar

Hindsight is an **agent memory system** (retain/recall/reflect) that can run locally and expose an **MCP server**. It can help reduce “context rot” and repeated “prompt too long” failures by moving long-term memory out of the chat history and into a separate store the agent can query.

CLIProxyAPI is a **proxy + translator**. It is not the “agent brain”, so the best integration is to run Hindsight as a **sidecar** and let your CLI agent call it via MCP tools.

Repo: https://github.com/vectorize-io/hindsight

## Recommended architecture

- Keep CLIProxyAPI focused on: auth, routing, translation, compatibility fixes, and request trimming.
- Run Hindsight separately (Docker or `pip install hindsight-api`).
- Add Hindsight as an MCP server to your agentic client (Droid/Factory CLI, Claude Code, etc.).
- Teach the agent (via your system prompt) to:
  - `retain` stable preferences and project facts
  - `recall` before large context dumps
  - `reflect` after successes/failures to improve consistency

## Run Hindsight locally

### Docker (easy)

```bash
# Provide an LLM key/model for Hindsight’s internal reasoning (separate from CLIProxyAPI)
export HINDSIGHT_API_LLM_PROVIDER=openai
export HINDSIGHT_API_LLM_API_KEY=sk-...
export HINDSIGHT_API_LLM_MODEL=gpt-4o-mini

docker run --rm -it -p 8888:8888 -p 9999:9999 \
  -e HINDSIGHT_API_LLM_PROVIDER \
  -e HINDSIGHT_API_LLM_API_KEY \
  -e HINDSIGHT_API_LLM_MODEL \
  -v $HOME/.hindsight-docker:/home/hindsight/.pg0 \
  ghcr.io/vectorize-io/hindsight:latest
```

- REST API: `http://localhost:8888`
- MCP server: `http://localhost:8888/mcp`
- UI: `http://localhost:9999`

### Python (no Docker)

```bash
pip install hindsight-api -U
export HINDSIGHT_API_LLM_PROVIDER=openai
export HINDSIGHT_API_LLM_API_KEY=sk-...
hindsight-api
```

## Connect your agent via MCP

Hindsight exposes MCP two ways:

- **HTTP MCP**: `http://localhost:8888/mcp`
- **stdio MCP**: `hindsight-local-mcp`

How you register an MCP server depends on the client. Many clients use a `.mcp.json` file. Typical patterns:

### Option A: stdio MCP (most portable)

```json
{
  "mcpServers": {
    "hindsight": {
      "command": "hindsight-local-mcp",
      "args": [],
      "env": {
        "HINDSIGHT_API_LLM_PROVIDER": "openai",
        "HINDSIGHT_API_LLM_API_KEY": "sk-..."
      }
    }
  }
}
```

### Option B: HTTP MCP

If your client supports HTTP MCP servers, point it at:

- `http://localhost:8888/mcp`

## Operational notes / gotchas

- **Privacy**: Hindsight stores memory content (and may call its own LLM). Treat it like a database of user data.
- **Tool-call correctness**: If you see tool sequencing errors in Claude/Vertex, check `docs/tool-calling-and-cli-compat.md`.
- **Keep it optional**: Don’t enable or inject memory tools by default from CLIProxyAPI; different clients have different tool behaviors, and tool injection can increase prompt size.

## Next step (if you want deeper integration)

If you want CLIProxyAPI to “own” the Hindsight lifecycle, we can spec a thin optional module:

- `config.yaml`:
  - `hindsight.enabled`
  - `hindsight.base_url`
  - `hindsight.auth_header` (optional)
- A small “memory helper” doc + scripts to start/stop the Hindsight sidecar alongside CLIProxyAPI.

