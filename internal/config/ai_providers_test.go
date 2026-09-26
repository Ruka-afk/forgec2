package config

import (
	"net/url"
	"strings"
	"testing"
)

// These tests guard the failure mode that motivated the provider registry:
// provider name, endpoint and model used to live in four independent switch
// statements that drifted, so "anthropic" was accepted by validation but had
// no endpoint or model case and its API key was POSTed to api.deepseek.com,
// while "google" and "local" silently fell through to DeepSeek.

// TestNoProviderSilentlyFallsBack is the load-bearing invariant: every real
// (non-alias) provider must carry its own endpoint and model. A missing case
// is a bug, never a fallback to another vendor.
func TestNoProviderSilentlyFallsBack(t *testing.T) {
	for _, p := range aiProviders {
		if p.AliasOf != "" {
			continue
		}
		t.Run(p.Name, func(t *testing.T) {
			if p.Dialect == "" {
				t.Errorf("provider %q has no dialect", p.Name)
			}
			if p.DefaultModel == "" {
				t.Errorf("provider %q has no default model; it would fall back to another vendor's model", p.Name)
			}
			// "custom" intentionally has no vendor endpoint.
			if p.DefaultEndpoint == "" {
				if p.Name != "custom" {
					t.Errorf("provider %q has no default endpoint and is not \"custom\"", p.Name)
				}
				return
			}
			u, err := url.Parse(p.DefaultEndpoint)
			if err != nil {
				t.Fatalf("provider %q endpoint unparseable: %v", p.Name, err)
			}
			if u.Scheme != "https" && u.Scheme != "http" {
				t.Errorf("provider %q endpoint scheme = %q", p.Name, u.Scheme)
			}
			// The actual regression: a provider whose host belongs to a
			// different vendor. Spot-check the vendors that share a dialect.
			host := u.Hostname()
			mustContain := map[string]string{
				"openai":    "openai.com",
				"anthropic": "anthropic.com",
				"deepseek":  "deepseek.com",
				"qianwen":   "aliyuncs.com",
				"zhipu":     "bigmodel.cn",
				"longcat":   "longcat.chat",
				"google":    "googleapis.com",
			}
			if want, ok := mustContain[p.Name]; ok && !strings.Contains(host, want) {
				t.Errorf("provider %q endpoint host %q does not belong to %q", p.Name, host, want)
			}
		})
	}
}

// TestAnthropicResolvesToAnthropic is the direct regression test for the
// credential-misdirection bug.
func TestAnthropicResolvesToAnthropic(t *testing.T) {
	if got := DefaultAIEndpoint("anthropic"); got != "https://api.anthropic.com/v1" {
		t.Errorf("anthropic endpoint = %q, want api.anthropic.com", got)
	}
	if got := AIProviderDefaultModel("anthropic"); got != "claude-3-5-sonnet-latest" {
		t.Errorf("anthropic model = %q, want a claude model", got)
	}
	if !AIProviderUsesClaudeWire("anthropic") {
		t.Error("anthropic must use the Anthropic /v1/messages wire dialect")
	}
	// "claude" is the spelling the settings UI writes and must behave identically.
	if got := DefaultAIEndpoint("claude"); got != "https://api.anthropic.com/v1" {
		t.Errorf("claude alias endpoint = %q", got)
	}
	if !AIProviderUsesClaudeWire("claude") {
		t.Error("claude alias must use the Claude wire dialect")
	}
}

// TestGoogleAndLocalNoLongerResolveToDeepSeek pins the two providers that used
// to validate successfully and then hit the DeepSeek default branch.
func TestGoogleAndLocalNoLongerResolveToDeepSeek(t *testing.T) {
	if got := DefaultAIEndpoint("google"); strings.Contains(got, "deepseek") {
		t.Errorf("google endpoint = %q, must not resolve to deepseek", got)
	}
	// "local" is a deprecated alias for ollama and must carry the loopback
	// default, not a cloud vendor.
	if got := DefaultAIEndpoint("local"); got != DefaultAIEndpoint("ollama") {
		t.Errorf("local alias endpoint = %q, want the ollama endpoint", got)
	}
	if p, ok := LookupAIProvider("local"); !ok || !p.Loopback {
		t.Error("local/ollama must be flagged Loopback so callers know it needs an allowlist entry")
	}
}

// TestCustomRequiresExplicitEndpoint documents the deliberate behaviour
// change: "custom" no longer defaults to api.openai.com.
func TestCustomRequiresExplicitEndpoint(t *testing.T) {
	if !AIProviderNeedsExplicitEndpoint("custom") {
		t.Error("custom must require an explicit endpoint")
	}
	if got := DefaultAIEndpoint("custom"); got != "" {
		t.Errorf("custom default endpoint = %q, want empty so callers must not guess", got)
	}
	for _, name := range []string{"openai", "deepseek", "anthropic", "ollama", "qianwen", "zhipu", "longcat", "google"} {
		if AIProviderNeedsExplicitEndpoint(name) {
			t.Errorf("provider %q must have a usable default endpoint", name)
		}
	}
}

func TestLookupAIProviderNormalizesInput(t *testing.T) {
	for _, spelling := range []string{"OpenAI", "  openai  ", "OPENAI"} {
		p, ok := LookupAIProvider(spelling)
		if !ok {
			t.Fatalf("LookupAIProvider(%q) not found", spelling)
		}
		if p.DefaultEndpoint != "https://api.openai.com/v1" {
			t.Errorf("LookupAIProvider(%q) endpoint = %q", spelling, p.DefaultEndpoint)
		}
	}
	if _, ok := LookupAIProvider("nosuchprovider"); ok {
		t.Error("unknown provider must not resolve")
	}
}

// TestEveryAliasTargetExists stops a typo in the registry from making an
// alias unresolvable at runtime.
func TestEveryAliasTargetExists(t *testing.T) {
	for _, p := range aiProviders {
		if p.AliasOf == "" {
			continue
		}
		target, ok := LookupAIProvider(p.AliasOf)
		if !ok {
			t.Errorf("provider %q aliases unknown provider %q", p.Name, p.AliasOf)
			continue
		}
		if target.DefaultEndpoint == "" {
			t.Errorf("provider %q aliases %q which has no endpoint", p.Name, p.AliasOf)
		}
	}
}

func TestAIProviderNamesIsSortedAndComplete(t *testing.T) {
	names := AIProviderNames()
	if len(names) != len(aiProviders) {
		t.Fatalf("AIProviderNames returned %d names for %d registry entries", len(names), len(aiProviders))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] > names[i] {
			t.Fatalf("AIProviderNames not sorted at %d: %q > %q", i, names[i-1], names[i])
		}
	}
	// Every registry entry must be selectable.
	for _, p := range aiProviders {
		if !IsKnownAIProvider(p.Name) {
			t.Errorf("registry entry %q is not accepted by IsKnownAIProvider", p.Name)
		}
	}
	if msg := AIProviderError().Error(); !strings.Contains(msg, "anthropic") {
		t.Errorf("provider error message should list the valid names, got %q", msg)
	}
}

// aiValidateConfig returns a DefaultConfig that satisfies every unrelated
// Validate() requirement, so these tests isolate the AI provider rules.
func aiValidateConfig() *Config {
	const k = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	cfg := DefaultConfig()
	cfg.Server.Port = 18001
	cfg.Crypto.LootKey = k
	cfg.Crypto.ExtC2Key = k
	cfg.Crypto.BackupKey = k
	cfg.Crypto.TotpKey = k
	cfg.Crypto.CsrfKey = k
	return cfg
}

// TestConfigValidateRejectsUnknownProvider proves Validate and the registry
// agree, and that the old hardcoded list cannot drift back in.
func TestConfigValidateRejectsUnknownProvider(t *testing.T) {
	cfg := aiValidateConfig()
	cfg.AI.Enabled = true
	cfg.AI.Provider = "definitely-not-a-provider"
	cfg.AI.APIKey = "k"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted an unknown AI provider")
	} else if !strings.Contains(err.Error(), "ai.provider must be one of") {
		t.Errorf("Validate error = %v, want the registry-generated message", err)
	}

	// Every registry name must pass validation when given an endpoint (custom
	// needs one; the rest have defaults).
	for _, name := range AIProviderNames() {
		cfg := aiValidateConfig()
		cfg.AI.Enabled = true
		cfg.AI.Provider = name
		cfg.AI.APIKey = "k"
		if AIProviderNeedsExplicitEndpoint(name) {
			cfg.AI.Endpoint = "https://example.invalid/v1"
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate rejected registry provider %q: %v", name, err)
		}
	}
}

// TestConfigValidateRequiresEndpointForCustom is the startup-time guard that
// replaces the old silent fallback to api.openai.com.
func TestConfigValidateRequiresEndpointForCustom(t *testing.T) {
	cfg := aiValidateConfig()
	cfg.AI.Enabled = true
	cfg.AI.Provider = "custom"
	cfg.AI.APIKey = "k"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate accepted custom provider with no endpoint")
	}
	if !strings.Contains(err.Error(), "ai.endpoint is required") {
		t.Errorf("Validate error = %v, want an explicit endpoint requirement", err)
	}

	cfg.AI.Endpoint = "https://openrouter.ai/api/v1"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate rejected custom provider with an endpoint: %v", err)
	}
}

// TestAIEndpointDoesNotCrossVendors exercises the method the one-shot assist
// endpoints depend on, which is the path that leaked the Anthropic key.
func TestAIEndpointDoesNotCrossVendors(t *testing.T) {
	cases := map[string]string{
		"anthropic": "anthropic.com",
		"openai":    "openai.com",
		"deepseek":  "deepseek.com",
		"google":    "googleapis.com",
	}
	for provider, wantHost := range cases {
		cfg := &Config{}
		cfg.AI.Provider = provider
		got := cfg.AIEndpoint()
		if !strings.Contains(got, wantHost) {
			t.Errorf("AIEndpoint(%q) = %q, want a %s host", provider, got, wantHost)
		}
	}
	// An explicit endpoint always wins.
	cfg := &Config{}
	cfg.AI.Provider = "deepseek"
	cfg.AI.Endpoint = "https://openrouter.ai/api/v1/chat/completions"
	if got := cfg.AIEndpoint(); got != cfg.AI.Endpoint {
		t.Errorf("explicit endpoint ignored: got %q", got)
	}
}
