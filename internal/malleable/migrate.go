package malleable

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateProfileJSON converts v1 payload JSON (or legacy Profile JSON) to v2.
// It never fails hard: unknown shapes fall back to a minimal v2 with name.
func MigrateProfileJSON(raw []byte, fallbackName string) (*ProfileV2, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, fmt.Errorf("profile is not valid JSON: %w", err)
	}
	// Already v2 if it carries any v2-only chain or beacon_uris.
	if hasAnyKey(probe, "client_metadata", "client_id", "server_output", "beacon_uris", "uris", "parameter") {
		var v2 ProfileV2
		if err := json.Unmarshal(raw, &v2); err != nil {
			return nil, err
		}
		if v2.Name == "" {
			v2.Name = fallbackName
		}
		if err := ValidateProfileV2(&v2); err != nil {
			return &v2, err
		}
		return &v2, nil
	}
	// Legacy http_get/http_post blocks?
	if hasAnyKey(probe, "http_get", "http_post") {
		var legacy Profile
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil, err
		}
		v2 := FromLegacyProfile(&legacy)
		if v2.Name == "" {
			v2.Name = fallbackName
		}
		return v2, ValidateProfileV2(v2)
	}
	// v1 payload profile.
	var v1 struct {
		Name                string            `json:"name"`
		Description         string            `json:"description"`
		UserAgent           string            `json:"user_agent"`
		BeaconURI           string            `json:"beacon_uri"`
		Method              string            `json:"method"`
		Headers             map[string]string `json:"headers"`
		Sleep               int               `json:"sleep"`
		Jitter              int               `json:"jitter"`
		Prepend             string            `json:"prepend"`
		Append              string            `json:"append"`
		RequestPrepend      string            `json:"request_prepend"`
		RequestAppend       string            `json:"request_append"`
		RequestHeaders      map[string]string `json:"request_headers"`
		ServerOutput        string            `json:"server_output"`
		ClientMetadata      string            `json:"client_metadata"`
		ContentLengthJitter int               `json:"content_length_jitter"`
		Placements          json.RawMessage   `json:"placements"`
		UserAgents          []string          `json:"user_agents"`
		JitterURI           bool              `json:"jitter_uri"`
		JitterParameter     bool              `json:"jitter_parameter"`
		Parameter           string            `json:"parameter"`
		ParameterNames      []string          `json:"parameter_names"`
		WorkStart           string            `json:"work_start"`
		WorkEnd             string            `json:"work_end"`
		WorkTZ              string            `json:"work_tz"`
	}
	if err := json.Unmarshal(raw, &v1); err != nil {
		return nil, err
	}
	v2 := &ProfileV2{
		Name: v1.Name, Description: v1.Description, UserAgent: v1.UserAgent,
		BeaconURI: v1.BeaconURI, Method: v1.Method, Headers: v1.Headers,
		Sleep: v1.Sleep, Jitter: v1.Jitter, Prepend: v1.Prepend, Append: v1.Append,
		RequestPrepend: v1.RequestPrepend, RequestAppend: v1.RequestAppend,
		RequestHeaders: v1.RequestHeaders, ContentLengthJitter: v1.ContentLengthJitter,
	}
	if v2.Name == "" {
		v2.Name = fallbackName
	}
	if v1.ServerOutput != "" {
		v2.ServerOutput = ParseWire(v1.ServerOutput)
	}
	if v1.ClientMetadata != "" {
		v2.ClientMetadata = ParseWire(v1.ClientMetadata)
	}
	v2.UserAgents = append([]string{}, v1.UserAgents...)
	v2.JitterURI = v1.JitterURI
	v2.JitterParameter = v1.JitterParameter
	if v1.Parameter != "" {
		v2.Parameter = v1.Parameter
	}
	v2.ParameterNames = append([]string{}, v1.ParameterNames...)
	v2.WorkStart, v2.WorkEnd, v2.WorkTZ = v1.WorkStart, v1.WorkEnd, v1.WorkTZ
	if len(v1.Placements) > 0 {
		var pls []PlacementV2
		if err := json.Unmarshal(v1.Placements, &pls); err == nil {
			v2.Placements = pls
		}
	}
	if err := ValidateProfileV2(v2); err != nil {
		return v2, err
	}
	return v2, nil
}

func hasAnyKey(m map[string]json.RawMessage, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

// FromLegacyProfile converts the CS-inspired Profile to v2.
func FromLegacyProfile(p *Profile) *ProfileV2 {
	if p == nil {
		return &ProfileV2{}
	}
	v2 := &ProfileV2{Name: p.Name, Description: p.Description}
	if p.HTTPConfig.UserAgent != "" {
		v2.UserAgent = p.HTTPConfig.UserAgent
	}
	for _, u := range p.HttpGet.URI {
		if u != "" {
			v2.URIs = append(v2.URIs, u)
		}
	}
	for _, u := range p.HttpPost.URI {
		if u != "" {
			v2.URIs = append(v2.URIs, u)
		}
	}
	if p.HttpGet.Verb != "" {
		v2.Verbs = append(v2.Verbs, strings.ToUpper(p.HttpGet.Verb))
	}
	if p.HttpPost.Verb != "" {
		v2.Verbs = append(v2.Verbs, strings.ToUpper(p.HttpPost.Verb))
	}
	if len(p.HttpGet.Headers) > 0 {
		v2.Headers = p.HttpGet.Headers
	}
	if len(p.HttpPost.Headers) > 0 {
		if v2.Headers == nil {
			v2.Headers = map[string]string{}
		}
		for k, v := range p.HttpPost.Headers {
			v2.Headers[k] = v
		}
	}
	if p.HttpGet.Metadata != nil {
		for _, t := range p.HttpGet.Metadata.Transforms {
			v2.ClientMetadata = append(v2.ClientMetadata, StepV2{Type: t.Type, Value: t.Value})
		}
	}
	if p.HttpPost.ID != nil {
		for _, t := range p.HttpPost.ID.Transforms {
			v2.ClientID = append(v2.ClientID, StepV2{Type: t.Type, Value: t.Value})
		}
	}
	if p.HttpPost.Output != nil {
		for _, t := range p.HttpPost.Output.Transforms {
			v2.ServerOutput = append(v2.ServerOutput, StepV2{Type: t.Type, Value: t.Value})
		}
	} else if p.HttpGet.Output != nil {
		for _, t := range p.HttpGet.Output.Transforms {
			v2.ServerOutput = append(v2.ServerOutput, StepV2{Type: t.Type, Value: t.Value})
		}
	}
	if p.HttpPost.Parameter != "" {
		v2.Parameter = p.HttpPost.Parameter
	}
	v2.ContentLengthJitter = p.Jitter.ContentLength
	v2.JitterURI = p.Jitter.URI
	v2.JitterParameter = p.Jitter.Parameter
	v2.ParameterNames = p.Jitter.ParameterNames
	if len(v2.URIs) > 0 {
		v2.BeaconURI = v2.URIs[0]
		v2.BeaconURIs = dedupStrings(v2.URIs)
	}
	return v2
}

func dedupStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// MigrateProfileDir migrates every *.json under dir/profiles to v2 in place,
// keeping a *.bak of the original on first migration.
func MigrateProfileDir(dir string) (migrated int, err error) {
	profDir := filepath.Join(dir, "profiles")
	entries, err := os.ReadDir(profDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasSuffix(e.Name(), ".v2.json") {
			continue
		}
		path := filepath.Join(profDir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var probe map[string]json.RawMessage
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		if hasAnyKey(probe, "client_metadata", "beacon_uris") {
			continue // already v2
		}
		v2, err := MigrateProfileJSON(raw, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out, _ := json.MarshalIndent(v2, "", "  ")
		_ = os.WriteFile(path+".bak", raw, 0644)
		if err := os.WriteFile(path, out, 0644); err == nil {
			migrated++
		}
	}
	return migrated, nil
}
