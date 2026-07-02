---
name: add-automation-rule
description: Add ForgeC2 automation rules and webhooks (event triggers, actions). Use for automation, webhooks, event rules, or /add-automation-rule.
license: MIT
compatibility: grok
metadata:
  audience: forgec2-devs
  workflow: feature
---

## When to use

Add event-driven automation (agent online, task complete) with command/webhook/notify actions.

## Key files

| File | Role |
|------|------|
| `internal/server/automation.go` | Rule eval, `evaluateRule`, `matchCondition`, `executeAction` |
| `internal/server/handlers_automation.go` | CRUD API + automation page |
| `internal/server/registerBuiltinAutomations()` | Built-in rule seeds |
| `templates/automation.html` | UI |
| `static/js/automation.js` | In `report.bundle.js` |

## Rule schema

```go
type AutomationRule struct {
    EventType  string          // e.g. "agent.online", "task.complete"
    Conditions []RuleCondition // field, operator, value
    Actions    []RuleAction    // type: command | webhook | notify
}
```

## Add a new event type

1. Emit event from handler (e.g. beacon online in `handlers_beacon.go`).
2. Call `s.dispatchEvent(Event{Type: "your.event", ...})`.
3. `automation.go` routes to `evaluateRule` for matching rules.
4. Document event fields available in `Conditions` (`agent.hostname`, `data.*`).

## Add webhook action

- `handleCreateWebhook`, `handleTestWebhook` in `handlers_automation.go`.
- `executeAction` POSTs JSON to configured URL on match.

## Verify

- Create rule in `/automation` UI
- Trigger event (e.g. agent check-in) → action fires
- Webhook test returns 200 in logs