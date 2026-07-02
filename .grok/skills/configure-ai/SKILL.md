---
name: configure-ai
description: Configure ForgeC2 AI assistant (provider, model, tool limits, system prompt). Use for AI config, tool call limits, or /configure-ai.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: config
---

## When to use

Configure AI provider, fix `[Max tool calls reached]`, tune system prompt, or add provider support.

## Config file (`config.yaml`)

```yaml
ai:
    enabled: true
    provider: deepseek   # deepseek | openai | claude | qianwen | custom
    api_key: sk-...
    model: deepseek-chat
    endpoint: ""         # required for custom
    system_prompt: "..."
    max_conversation_turns: 0   # 0 = unlimited
    max_tool_rounds: 0          # 0 = unlimited
    max_duplicate_tool_calls: 0 # 0 = unlimited; set 5 to block identical loops
```

Defaults in `internal/config/config.go` — `0` means no cap.

## Runtime save

- UI: `/ai` settings panel → `POST /ai/config` (`handleAIConfig` in `handlers_ai.go`)
- Admin only; API key blank = keep existing key

## Tool calling loop

| File | Function |
|------|----------|
| `handlers_ai.go` | `converse()` — SSE loop |
| `handlers_ai.go` | `buildTools()` — function definitions |
| `handlers_ai.go` | `executeTool()` — server-side execution |
| `handlers_ai.go` | `resolveAIToolLimits()` — reads config caps |

`[Max tool calls reached]` comes from `converse()` when `max_tool_rounds > 0` exceeded — not Cursor.

## Add provider adapter

1. `config.go` `GetAIEndpoint()` switch for default URLs.
2. `aiDoRequest()` — auth headers (`Bearer` vs `x-api-key` for Claude).
3. Stream parser: `parseStreamChunks` (OpenAI-compat) or `parseClaudeStream`.

## Verify

- `/ai` page shows configured model
- Chat streams response; tools invoke without premature cutoff
- Set `max_tool_rounds: 3` → limit triggers as expected