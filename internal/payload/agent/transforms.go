//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"encoding/base64"
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

	case "xor":
		if len(step.Value) == 0 {
			return data, nil
		}
		k := step.Value[0]
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ k
		}
		return out, nil

	case "mask":
		key := step.Value
		if len(key) == 0 {
			return data, nil
		}
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ key[i%len(key)]
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
