# Agentic Harness Demo (ProxyPilot)

This is a static, copy-paste demo showing how the middleware rewrites requests.

## Responses API (Codex CLI style)

Input:
```json
{
  "model": "gpt-5-codex",
  "instructions": "Follow the user's request.",
  "input": [
    { "role": "user", "content": [{ "type": "input_text", "text": "Build a todo app" }] }
  ]
}
```

Injected (INITIALIZER mode):
```json
{
  "model": "gpt-5-codex",
  "instructions": "Follow the user's request.\n\n<harness_mode>INITIALIZER</harness_mode>\nYou are the **Initializer Agent** in a Long-Running Agent Harness.\nYour ONLY goal is to set up the project environment for future Coding Agents.\n\n**CRITICAL INSTRUCTIONS**:\n1.  **Analyze the User's Request**: Understand what feature or app needs to be built.\n2.  **Create 'feature_list.json'**:\n    -   Write a JSON file listing ALL features/requirements as separate items.\n    -   Mark them all as \"passes\": false.\n    -   Format:\n        Request: \"Build a todo app\"\n        File Content:\n        [\n          { \"category\": \"core\", \"description\": \"User can add a todo\", \"steps\": [\"Open app\", \"Enter text\", \"Press enter\", \"See new item\"], \"passes\": false },\n          { \"category\": \"core\", \"description\": \"User can delete a todo\", \"steps\": [\"Open app\", \"Click delete\", \"Item removed\"], \"passes\": false }\n        ]\n3.  **Create 'claude-progress.txt'**:\n    -   Create this file with a header \"## Progress Log\".\n    -   Add an initial entry: \"- [Initializer] Environment setup started.\"\n4.  **Create 'init.sh' (Optional)**:\n    -   If a dev server is needed, create a script to start it (e.g., 'npm run dev').\n5.  **Initial Commit**:\n    -   Initialize git if not present.\n    -   Commit these foundational files.\n\n**DO NOT** start implementing features yourself. Your job is ONLY to scaffold the harness.",
  "input": [
    { "role": "user", "content": [{ "type": "input_text", "text": "Build a todo app" }] }
  ]
}
```

## Chat Completions (Claude / OAI chat)

Input:
```json
{
  "model": "claude-sonnet-4-5-thinking",
  "messages": [
    { "role": "system", "content": "You are a helpful assistant." },
    { "role": "user", "content": "Set up the repo" }
  ]
}
```

Injected (INITIALIZER mode):
```json
{
  "model": "claude-sonnet-4-5-thinking",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant.\n\n<harness_mode>INITIALIZER</harness_mode>\nYou are the **Initializer Agent** in a Long-Running Agent Harness.\nYour ONLY goal is to set up the project environment for future Coding Agents.\n\n**CRITICAL INSTRUCTIONS**:\n... (same initializer prompt) ..."
    },
    { "role": "user", "content": "Set up the repo" }
  ]
}
```

## CODING mode trigger

If `feature_list.json` or `claude-progress.txt` exists under `harness-root-dir`,
the middleware switches to CODING mode and injects the Coding prompt instead.

The response header will include:
```
X-ProxyPilot-Harness-Mode: CODING
```
