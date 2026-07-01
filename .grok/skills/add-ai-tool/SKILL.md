---
name: add-ai-tool
description: Add a new AI function-calling tool to ForgeC2 via handlers_ai.go buildTools and executeTool
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Expose a new ForgeC2 capability to the AI assistant (SSE chat at `/ai/chat`).

## File

`internal/server/handlers_ai.go`

## Step 1 — Define tool in `buildTools()`

```go
{
    Type: "function",
    Function: toolFuncDef{
        Name:        "your_tool_name",
        Description: "What the tool does and when the AI should use it",
        Parameters: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "param1": map[string]string{
                    "type":        "string",
                    "description": "Parameter description",
                },
            },
            "required": []string{"param1"},
        },
    },
},
```

## Step 2 — Implement in `executeTool()`

```go
case "your_tool_name":
    param1 := args["param1"]
    if param1 == "" {
        return `{"error":"param1 required"}`
    }
    // query DB or call server helpers
    b, _ := json.Marshal(result)
    return string(b)
```

Add a `default` branch if missing — unknown tools should return `{"error":"unknown tool"}`.

## Design rules

- **Name**: `snake_case`, unique among tools in `buildTools()`
- **Description**: clear enough for the LLM to choose correctly
- **Return**: always JSON string; errors as `{"error":"reason"}`
- **Side effects**: `execute_command` queues tasks — document async behavior in the description
- **No secrets**: never return plaintext passwords or API keys

## Existing tools (reference)

| Tool | Purpose |
|------|---------|
| `list_agents` | List implants |
| `get_agent_detail` | Agent metadata + task count |
| `execute_command` | Queue shell task |
| `get_agent_tasks` | Fetch task results |
| `list_listeners` | Listener config |
| `list_credentials` | Credential summary (no plaintext) |
| `get_online_operators` | Active operators |

## Verify

```bash
go build ./internal/server/...
```

- AI chat invokes the new tool when prompted
- Tool returns valid JSON (no panic)
- SSE stream completes with a user-visible answer