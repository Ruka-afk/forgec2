package malleable

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

var base64URL = base64.URLEncoding

// Apply applies the transform chain to data.
// Each transform is applied in sequence, passing the output of one as input to the next.
func (tb *TransformBlock) Apply(data []byte, encode bool) ([]byte, error) {
	if tb == nil || len(tb.Transforms) == 0 {
		return data, nil
	}

	current := data
	for _, t := range tb.Transforms {
		var err error
		current, err = applySingle(current, t, encode)
		if err != nil {
			return nil, fmt.Errorf("transform %q: %v", t.Type, err)
		}
	}
	return current, nil
}

func applySingle(data []byte, t Transform, encode bool) ([]byte, error) {
	switch t.Type {
	case "base64":
		if encode {
			dst := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(dst, data)
			return dst, nil
		}
		dst := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(dst, data)
		if err != nil {
			return nil, err
		}
		return dst[:n], nil

	case "netbios":
		if encode {
			return netbiosEncode(data), nil
		}
		return netbiosDecode(data), nil

	case "base64url":
		if encode {
			return []byte(base64URL.EncodeToString(data)), nil
		}
		s := strings.TrimSpace(string(data))
		// Accept unpadded input.
		if m := len(s) % 4; m != 0 {
			s += strings.Repeat("=", 4-m)
		}
		dst := make([]byte, base64URL.DecodedLen(len(s)))
		n, err := base64URL.Decode(dst, []byte(s))
		if err != nil {
			return nil, err
		}
		return dst[:n], nil

	case "netbiosu":
		if encode {
			return netbiosUEncode(data), nil
		}
		return netbiosUDecode(data), nil

	case "strrep":
		// value is "old:new"; encode replaces old->new, decode reverses.
		old, rep := splitStrrep(t.Value)
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

	case "mask":
		v, err := normalizeMaskValue(t.Value)
		if err != nil {
			return nil, err
		}
		return applyMask(data, v), nil

	case "print":
		if encode {
			return printableEncode(data), nil
		}
		return printableDecode(string(data))

	case "append":
		return append(data, []byte(t.Value)...), nil

	case "prepend":
		return append([]byte(t.Value), data...), nil

	case "xor":
		return xorData(data, t.Value), nil

	case "urlencode":
		if encode {
			return []byte(urlEncode(string(data))), nil
		}
		decoded, err := urlDecode(string(data))
		if err != nil {
			return data, nil
		}
		return []byte(decoded), nil

	case "uri_append":
		if encode {
			return append(data, []byte(t.Value)...), nil
		}
		if len(data) > len(t.Value) && string(data[len(data)-len(t.Value):]) == t.Value {
			return data[:len(data)-len(t.Value)], nil
		}
		return data, nil

	case "header":
		// Used in metadata/output blocks to set HTTP header
		return data, nil

	case "parameter":
		// Used in metadata blocks to set URL parameter
		return data, nil

	default:
		return data, nil
	}
}

// netbiosEncode encodes data as NetBIOS name (A-encoding).
func netbiosEncode(data []byte) []byte {
	var result []byte
	for _, b := range data {
		result = append(result, 'A'+byte((b>>4)&0xF), 'A'+byte(b&0xF))
	}
	return result
}

// netbiosDecode decodes a NetBIOS A-encoded name.
func netbiosDecode(data []byte) []byte {
	var result []byte
	for i := 0; i+1 < len(data); i += 2 {
		hi := data[i] - 'A'
		lo := data[i+1] - 'A'
		if hi < 16 && lo < 16 {
			result = append(result, (hi<<4)|lo)
		}
	}
	return result
}

// normalizeMaskValue validates mask input. Empty key is rejected instead of
// panicking on modulo-zero (legacy path did not guard).
func normalizeMaskValue(param string) (string, error) {
	if strings.TrimSpace(param) == "" {
		return "", fmt.Errorf("mask requires a key value")
	}
	parts := strings.SplitN(param, ";", 2)
	if parts[0] == "" {
		return "", fmt.Errorf("mask requires a non-empty key")
	}
	return param, nil
}

// applyMask applies XOR mask with the given key and offset.
// Format: "key;offset" (e.g., "secret;3")
func applyMask(data []byte, param string) []byte {
	parts := strings.SplitN(param, ";", 2)
	key := param
	offset := 0
	if len(parts) == 2 {
		key = parts[0]
		fmt.Sscanf(parts[1], "%d", &offset)
	}
	if len(key) == 0 {
		return data
	}
	if offset < 0 {
		offset = 0
	}
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[(i+offset)%len(key)]
	}
	return result
}

func netbiosUEncode(data []byte) []byte {
	var out []byte
	for _, b := range data {
		hi, lo := (b>>4)&0xF, b&0xF
		hc, lc := 'A'+hi, 'a'+lo
		out = append(out, byte(hc), byte(lc))
	}
	return out
}

func netbiosUDecode(data []byte) []byte {
	var out []byte
	for i := 0; i+1 < len(data); i += 2 {
		var hi, lo byte
		c0, c1 := data[i], data[i+1]
		if c0 >= 'A' && c0 <= 'P' {
			hi = c0 - 'A'
		} else if c0 >= 'a' && c0 <= 'p' {
			hi = c0 - 'a'
		} else {
			continue
		}
		if c1 >= 'A' && c1 <= 'P' {
			lo = c1 - 'A'
		} else if c1 >= 'a' && c1 <= 'p' {
			lo = c1 - 'a'
		} else {
			continue
		}
		out = append(out, (hi<<4)|lo)
	}
	return out
}

func splitStrrep(v string) (string, string) {
	if i := strings.Index(v, ":"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
}

// printableEncode encodes binary data as printable hex-like string.
func printableEncode(data []byte) []byte {
	const hex = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hex[b>>4]
		result[i*2+1] = hex[b&0xF]
	}
	return result
}

// printableDecode decodes printable-encoded data.
func printableDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("printable decode: odd length")
	}
	result := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		hi := strings.IndexByte("0123456789abcdef", s[i])
		lo := strings.IndexByte("0123456789abcdef", s[i+1])
		if hi < 0 || lo < 0 {
			return nil, fmt.Errorf("printable decode: invalid char at %d", i)
		}
		result[i/2] = byte(hi<<4) | byte(lo)
	}
	return result, nil
}

// xorData XORs data with the given key, repeating the key as needed.
func xorData(data []byte, key string) []byte {
	if len(key) == 0 {
		return data
	}
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[i%len(key)]
	}
	return result
}

// urlEncode percent-encodes a string.
func urlEncode(s string) string {
	return url.QueryEscape(s)
}

// urlDecode percent-decodes a string.
func urlDecode(s string) (string, error) {
	return url.QueryUnescape(s)
}
