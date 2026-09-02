package server

import (
	"encoding/json"
	"regexp"
	"strings"
)

// aiReasoningHint asks OpenRouter/DeepSeek-style endpoints to include
// chain-of-thought in the stream (delta.reasoning / reasoning_content).
type aiReasoningHint struct {
	Enabled bool `json:"enabled"`
}

var thinkBlockRe = regexp.MustCompile(`(?is)<think(?:ing)?>\s*(.*?)\s*</think(?:ing)?>`)

func aiShouldRequestReasoning(provider, endpoint string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	e := strings.ToLower(endpoint)
	if p == "deepseek" {
		return true
	}
	return strings.Contains(e, "openrouter.ai") || strings.Contains(e, "openrouter.com")
}

// decodeStreamText accepts the shapes providers actually send for a streaming
// text field: a JSON string, null, or an array of {type,text} parts.
func decodeStreamText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Text)
		}
		return b.String()
	}
	return ""
}

func streamReasoningDelta(reasoning, reasoningContent json.RawMessage, details []struct {
	Type string `json:"type"`
	Text string `json:"text"`
}) string {
	if s := decodeStreamText(reasoning); s != "" {
		return s
	}
	if s := decodeStreamText(reasoningContent); s != "" {
		return s
	}
	var b strings.Builder
	for _, d := range details {
		b.WriteString(d.Text)
	}
	return b.String()
}

func splitThinkBlocks(content string) (think, visible string) {
	if content == "" {
		return "", ""
	}
	matches := thinkBlockRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		lower := strings.ToLower(content)
		for _, open := range []string{"<think>", "<thinking>"} {
			if i := strings.Index(lower, open); i >= 0 {
				return strings.TrimSpace(content[i+len(open):]), strings.TrimSpace(content[:i])
			}
		}
		return "", content
	}
	var thinks []string
	for _, m := range matches {
		if strings.TrimSpace(m[1]) != "" {
			thinks = append(thinks, strings.TrimSpace(m[1]))
		}
	}
	visible = strings.TrimSpace(thinkBlockRe.ReplaceAllString(content, ""))
	return strings.Join(thinks, "\n\n"), visible
}
