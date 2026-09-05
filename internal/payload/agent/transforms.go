//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

type agentTransformFunc func([]byte) ([]byte, error)

type agentTransformStep struct {
	Name  string
	Value string
}

func agentApplyTransforms(data []byte, steps []agentTransformStep, encode bool) ([]byte, error) {
	current := data
	if encode {
		for _, s := range steps {
			var err error
			current, err = agentExecTransform(current, s, true)
			if err != nil {
				return nil, err
			}
		}
	} else {
		for i := len(steps) - 1; i >= 0; i-- {
			var err error
			current, err = agentExecTransform(current, steps[i], false)
			if err != nil {
				return nil, err
			}
		}
	}
	return current, nil
}

func agentExecTransform(data []byte, step agentTransformStep, encode bool) ([]byte, error) {
	switch step.Name {
	case "base64":
		if encode {
			out := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(out, data)
			return out, nil
		}
		out := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(out, data)
		if err != nil {
			return nil, err
		}
		return out[:n], nil

	case "base64url":
		if encode {
			out := make([]byte, base64.URLEncoding.EncodedLen(len(data)))
			base64.URLEncoding.Encode(out, data)
			return out, nil
		}
		out := make([]byte, base64.URLEncoding.DecodedLen(len(data)))
		n, err := base64.URLEncoding.Decode(out, data)
		if err != nil {
			return nil, err
		}
		return out[:n], nil

	case "netbios":
		if encode {
			return agentNetBIOSEncode(data), nil
		}
		return agentNetBIOSDecode(data), nil

	case "netbiosu":
		if encode {
			return agentNetBIOSUEncode(data), nil
		}
		return agentNetBIOSUDecode(data), nil

	case "urlencode":
		if encode {
			return []byte(url.QueryEscape(string(data))), nil
		}
		s, err := url.QueryUnescape(string(data))
		if err != nil {
			return nil, err
		}
		return []byte(s), nil

	case "uri_append":
		if encode {
			out := make([]byte, len(data)+len(step.Value))
			copy(out, data)
			copy(out[len(data):], step.Value)
			return out, nil
		}
		if v := step.Value; v != "" && len(data) >= len(v) && string(data[len(data)-len(v):]) == v {
			return data[:len(data)-len(v)], nil
		}
		return data, nil

	case "strrep":
		old, rep := step.Value, ""
		if i := strings.Index(step.Value, ":"); i >= 0 {
			old, rep = step.Value[:i], step.Value[i+1:]
		}
		if old == "" {
			return data, nil
		}
		if encode {
			return []byte(strings.ReplaceAll(string(data), old, rep)), nil
		}
		return []byte(strings.ReplaceAll(string(data), rep, old)), nil

	case "case":
		if encode {
			return []byte(strings.ToUpper(string(data))), nil
		}
		return []byte(strings.ToLower(string(data))), nil

	case "xor":
		// Full repeating-key XOR, symmetric, matching the server engine.
		if len(step.Value) == 0 {
			return data, nil
		}
		key := step.Value
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ key[i%len(key)]
		}
		return out, nil

	case "mask":
		// Format "key;offset", symmetric like the server engine.
		key, offset := step.Value, 0
		if i := strings.Index(key, ";"); i >= 0 {
			fmt.Sscanf(key[i+1:], "%d", &offset)
			key = key[:i]
		}
		if len(key) == 0 {
			return data, nil
		}
		if offset < 0 {
			offset = 0
		}
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ key[(i+offset)%len(key)]
		}
		return out, nil

	case "prepend":
		if encode {
			out := make([]byte, len(step.Value)+len(data))
			copy(out, step.Value)
			copy(out[len(step.Value):], data)
			return out, nil
		}
		v := step.Value
		if len(data) >= len(v) && string(data[:len(v)]) == v {
			return data[len(v):], nil
		}
		return data, nil

	case "append":
		if encode {
			out := make([]byte, len(data)+len(step.Value))
			copy(out, data)
			copy(out[len(data):], step.Value)
			return out, nil
		}
		v := step.Value
		if len(data) >= len(v) && string(data[len(data)-len(v):]) == v {
			return data[:len(data)-len(v)], nil
		}
		return data, nil

	case "print":
		out := make([]byte, len(data))
		copy(out, data)
		return out, nil

	default:
		out := make([]byte, len(data))
		copy(out, data)
		return out, nil
	}
}

// parseTransformSteps parses a serialized transform pipeline (the over-the-wire
// form of a malleable profile's output transforms). Steps are separated by ';'
// and each step is "name" or "name:value". The order matches the server's apply
// order; callers decode with encode=false, which reverses the order, so the
// agent recovers the original body.
func parseTransformSteps(s string) []agentTransformStep {
	if s == "" {
		return nil
	}
	var steps []agentTransformStep
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.IndexByte(part, ':'); i >= 0 {
			steps = append(steps, agentTransformStep{Name: part[:i], Value: part[i+1:]})
		} else {
			steps = append(steps, agentTransformStep{Name: part})
		}
	}
	return steps
}

// malleableRespDecodeSteps caches the parsed response-decode pipeline so the
// per-beacon decode path does not re-parse the step string on every check-in.
var malleableRespDecodeSteps []agentTransformStep

// reparseMalleableTransforms re-parses the configured response-decode pipeline
// from MalleableRespDecode. Called at init (for build-time steps) and whenever
// an over-the-wire network config updates the steps.
func reparseMalleableTransforms() {
	malleableRespDecodeSteps = parseTransformSteps(MalleableRespDecode)
}

func init() {
	reparseMalleableTransforms()
}

func agentNetBIOSEncode(data []byte) []byte {
	var out []byte
	for _, b := range data {
		out = append(out, 'A'+byte((b>>4)&0xF), 'A'+byte(b&0xF))
	}
	return out
}

func agentNetBIOSDecode(data []byte) []byte {
	var out []byte
	for i := 0; i+1 < len(data); i += 2 {
		hi := data[i] - 'A'
		lo := data[i+1] - 'A'
		if hi < 16 && lo < 16 {
			out = append(out, (hi<<4)|lo)
		}
	}
	return out
}

func agentNetBIOSUEncode(data []byte) []byte {
	var out []byte
	for _, b := range data {
		out = append(out, 'A'+byte((b>>4)&0xF), 'a'+byte(b&0xF))
	}
	return out
}

func agentNetBIOSUDecode(data []byte) []byte {
	var out []byte
	for i := 0; i+1 < len(data); i += 2 {
		var hi, lo byte
		c0, c1 := data[i], data[i+1]
		switch {
		case c0 >= 'A' && c0 <= 'P':
			hi = c0 - 'A'
		case c0 >= 'a' && c0 <= 'p':
			hi = c0 - 'a'
		default:
			continue
		}
		switch {
		case c1 >= 'A' && c1 <= 'P':
			lo = c1 - 'A'
		case c1 >= 'a' && c1 <= 'p':
			lo = c1 - 'a'
		default:
			continue
		}
		out = append(out, (hi<<4)|lo)
	}
	return out
}
