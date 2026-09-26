package config

import (
	"fmt"
	"sort"
	"strings"
)

// AIProvider is the canonical description of one supported LLM provider.
//
// This registry is the single source of truth for provider names, their wire
// dialect, default endpoint and default model. It exists because those four
// facts used to be duplicated across four independent switch statements
// (config.AIEndpoint, config.Validate, aiDefaultModel, aiDefaultEndpoint) and
// they drifted apart: "anthropic" was accepted by validation but had no
// endpoint or model case, so an Anthropic API key was silently POSTed to
// api.deepseek.com, and "google"/"local" fell through to DeepSeek with no
// error at all. See TestNoProviderSilentlyFallsBack.
type AIProvider struct {
	// Name is the value operators put in ai.provider / the profile provider.
	Name string
	// Dialect selects the request/response wire format: "openai" for
	// /chat/completions, "claude" for Anthropic /v1/messages.
	Dialect string
	// DefaultEndpoint is the base URL used when the operator supplies none.
	DefaultEndpoint string
	// DefaultModel is used when the operator supplies no model.
	DefaultModel string
	// AliasOf marks a deprecated spelling that resolves to another entry.
	// Alias entries never carry their own endpoint or model.
	AliasOf string
	// Loopback marks providers whose default endpoint is a local address and
	// therefore only usable when the operator allowlists it (see
	// Server.validateAIEndpoint). Local addresses are rejected by the SSRF
	// guard by default.
	Loopback bool
}

// AI wire dialects.
const (
	aiDialectOpenAI = "openai"
	aiDialectClaude = "claude"
)

// aiProviders is the authoritative registry. Adding a provider here is the
// only supported way to make it selectable: validation, the config endpoint,
// provider profiles and the runs path all read from this slice.
//
// Rules for keeping it safe:
//   - Every non-alias entry MUST have its own endpoint and model. A missing
//     case is a bug, not a fallback (TestNoProviderSilentlyFallsBack enforces).
//   - Never point one vendor's key at another vendor's host.
var aiProviders = []AIProvider{
	{
		Name:            "openai",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://api.openai.com/v1",
		DefaultModel:    "gpt-4o-mini",
	},
	{
		Name:            "anthropic",
		Dialect:         aiDialectClaude,
		DefaultEndpoint: "https://api.anthropic.com/v1",
		DefaultModel:    "claude-3-5-sonnet-latest",
	},
	{
		// "claude" is the spelling the settings UI writes; keep it working.
		Name:    "claude",
		AliasOf: "anthropic",
	},
	{
		Name:            "deepseek",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://api.deepseek.com/v1",
		DefaultModel:    "deepseek-chat",
	},
	{
		Name:            "qianwen",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		DefaultModel:    "qwen-plus",
	},
	{
		Name:            "zhipu",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel:    "glm-4-flash",
	},
	{
		Name:            "longcat",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://api.longcat.chat/openai/v1",
		DefaultModel:    "LongCat-Flash-Chat",
	},
	{
		// Google's OpenAI-compatibility surface. NOT verified against the live
		// API in this repository's tests (no credential available in CI);
		// the base URL follows Google's published v1beta/openai path.
		Name:            "google",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai",
		DefaultModel:    "gemini-2.0-flash",
	},
	{
		// Local Ollama / OpenAI-compatible server. The default endpoint is a
		// loopback address, which the SSRF guard rejects unless the operator
		// lists it under ai.allowed_endpoints.
		Name:            "ollama",
		Dialect:         aiDialectOpenAI,
		DefaultEndpoint: "http://localhost:11434/v1",
		DefaultModel:    "llama3.1",
		Loopback:        true,
	},
	{
		// Historical spelling kept for backwards compatibility with existing
		// config.yaml files; new configs should use "ollama".
		Name:    "local",
		AliasOf: "ollama",
	},
	{
		// "custom" has no vendor default on purpose: the operator must state
		// the endpoint explicitly. It previously defaulted to api.openai.com,
		// which sent the credential to a vendor the operator never named.
		// DefaultEndpoint is empty and DefaultAIEndpoint reports the miss so
		// callers can fail loudly. DefaultModel is a widely-compatible
		// OpenAI-format model name for gateways that ignore the field.
		Name:         "custom",
		Dialect:      aiDialectOpenAI,
		DefaultModel: "gpt-4o-mini",
	},
}

// aiProviderIndex is built once at init; aiProviders is a package-level slice
// so it is safe to read without a lock.
var aiProviderIndex = func() map[string]AIProvider {
	idx := make(map[string]AIProvider, len(aiProviders))
	for _, p := range aiProviders {
		idx[p.Name] = p
	}
	return idx
}()

// AIProviderNames returns every accepted provider spelling, sorted. Use this
// for validation and for error messages so the three call sites that used to
// carry private copies of the list cannot drift again.
func AIProviderNames() []string {
	names := make([]string, 0, len(aiProviders))
	for _, p := range aiProviders {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	return names
}

// NormalizeAIProvider lowercases and trims a provider name. It does not
// resolve aliases.
func NormalizeAIProvider(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// LookupAIProvider resolves a provider name, following alias entries. The
// bool reports whether the name is known at all.
func LookupAIProvider(name string) (AIProvider, bool) {
	key := NormalizeAIProvider(name)
	p, ok := aiProviderIndex[key]
	if !ok {
		return AIProvider{}, false
	}
	// Follow aliases (one hop is enough for this table, but loop defensively).
	for i := 0; p.AliasOf != "" && i < len(aiProviders); i++ {
		target, found := aiProviderIndex[p.AliasOf]
		if !found {
			return AIProvider{}, false
		}
		p = target
	}
	// Alias entries inherit the identity of what they point at, so callers
	// only ever see canonical records.
	p.Name = key
	return p, true
}

// IsKnownAIProvider reports whether name is an accepted provider spelling.
func IsKnownAIProvider(name string) bool {
	_, ok := LookupAIProvider(name)
	return ok
}

// AIProviderDefaultModel returns the default model for a provider, or
// "deepseek-chat" only when the provider is unknown (callers validate first,
// so this is a last-resort value and not a routing decision).
func AIProviderDefaultModel(provider string) string {
	if p, ok := LookupAIProvider(provider); ok && p.DefaultModel != "" {
		return p.DefaultModel
	}
	return "deepseek-chat"
}

// DefaultAIEndpoint returns the base URL for a provider. An empty result means
// the provider deliberately has no vendor default (only "custom" today) and
// the caller must require an explicit endpoint rather than guessing.
func DefaultAIEndpoint(provider string) string {
	if p, ok := LookupAIProvider(provider); ok {
		return p.DefaultEndpoint
	}
	return ""
}

// AIProviderNeedsExplicitEndpoint reports whether a provider requires the
// operator to configure an endpoint because it has no vendor default.
func AIProviderNeedsExplicitEndpoint(provider string) bool {
	p, ok := LookupAIProvider(provider)
	return ok && p.DefaultEndpoint == ""
}

// AIProviderDialect returns the wire dialect ("openai" or "claude") for a
// provider. Unknown providers are treated as OpenAI-compatible, which is the
// dialect every gateway speaks.
func AIProviderDialect(provider string) string {
	if p, ok := LookupAIProvider(provider); ok && p.Dialect != "" {
		return p.Dialect
	}
	return aiDialectOpenAI
}

// AIProviderUsesClaudeWire reports whether provider speaks the Anthropic
// /v1/messages dialect, which needs different auth headers and body shape.
func AIProviderUsesClaudeWire(provider string) bool {
	return AIProviderDialect(provider) == aiDialectClaude
}

// AIProviderError renders the canonical "must be one of" message.
func AIProviderError() error {
	return fmt.Errorf("ai.provider must be one of: %s", strings.Join(AIProviderNames(), ", "))
}
