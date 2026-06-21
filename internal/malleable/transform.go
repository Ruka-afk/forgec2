package malleable

import (
	"encoding/base64"
	"fmt"
	"strings"
)

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

	case "mask":
		return applyMask(data, t.Value), nil

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
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = b ^ key[(i+offset)%len(key)]
	}
	return result
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
