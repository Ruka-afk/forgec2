package malleable

import (
	"encoding/base64"
	"fmt"
)

type TransformFunc func([]byte) ([]byte, error)

type TransformPipeline struct {
	Encode TransformFunc
	Decode TransformFunc
}

func Prepend(value string) TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, len(value)+len(data))
		copy(out, value)
		copy(out[len(value):], data)
		return out, nil
	}
}

func Append(value string) TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, len(data)+len(value))
		copy(out, data)
		copy(out[len(data):], value)
		return out, nil
	}
}

func StripPrefix(prefix string) TransformFunc {
	return func(data []byte) ([]byte, error) {
		if len(data) >= len(prefix) && string(data[:len(prefix)]) == prefix {
			return data[len(prefix):], nil
		}
		return data, nil
	}
}

func StripSuffix(suffix string) TransformFunc {
	return func(data []byte) ([]byte, error) {
		if len(data) >= len(suffix) && string(data[len(data)-len(suffix):]) == suffix {
			return data[:len(data)-len(suffix)], nil
		}
		return data, nil
	}
}

func Base64() TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(out, data)
		return out, nil
	}
}

func Base64Decode() TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
		n, err := base64.StdEncoding.Decode(out, data)
		if err != nil {
			return nil, err
		}
		return out[:n], nil
	}
}

func Base64URL() TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, base64.URLEncoding.EncodedLen(len(data)))
		base64.URLEncoding.Encode(out, data)
		return out, nil
	}
}

func Base64URLDecode() TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, base64.URLEncoding.DecodedLen(len(data)))
		n, err := base64.URLEncoding.Decode(out, data)
		if err != nil {
			return nil, err
		}
		return out[:n], nil
	}
}

func NetBIOS() TransformFunc {
	return func(data []byte) ([]byte, error) {
		var out []byte
		for _, b := range data {
			out = append(out, 'A'+byte((b>>4)&0xF), 'A'+byte(b&0xF))
		}
		return out, nil
	}
}

func NetBIOSDecode() TransformFunc {
	return func(data []byte) ([]byte, error) {
		var out []byte
		for i := 0; i+1 < len(data); i += 2 {
			hi := data[i] - 'A'
			lo := data[i+1] - 'A'
			if hi < 16 && lo < 16 {
				out = append(out, (hi<<4)|lo)
			}
		}
		return out, nil
	}
}

func XOR(key byte) TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ key
		}
		return out, nil
	}
}

func Mask(key string) TransformFunc {
	return func(data []byte) ([]byte, error) {
		if len(key) == 0 {
			return data, nil
		}
		out := make([]byte, len(data))
		for i, b := range data {
			out[i] = b ^ key[i%len(key)]
		}
		return out, nil
	}
}

func Print() TransformFunc {
	return func(data []byte) ([]byte, error) {
		out := make([]byte, len(data))
		copy(out, data)
		return out, nil
	}
}

func (p *MalleableProfile) CompileTransforms() (encode, decode TransformPipeline, err error) {
	steps := p.HTTPPost.Server.Output
	if len(steps) == 0 {
		steps = p.HTTPGet.Server.Output
	}
	var encFuncs []TransformFunc
	var decFuncs []TransformFunc
	for _, step := range steps {
		enc, dec, err := compileStep(step)
		if err != nil {
			return encode, decode, fmt.Errorf("compile step %q: %w", step.Name, err)
		}
		if enc != nil {
			encFuncs = append(encFuncs, enc)
		}
		if dec != nil {
			decFuncs = append([]TransformFunc{dec}, decFuncs...)
		}
	}
	encode.Encode = chain(encFuncs)
	decode.Decode = chain(decFuncs)
	return encode, decode, nil
}

func compileStep(step TransformStep) (enc, dec TransformFunc, err error) {
	switch step.Name {
	case "base64":
		return Base64(), Base64Decode(), nil
	case "base64url":
		return Base64URL(), Base64URLDecode(), nil
	case "netbios":
		return NetBIOS(), NetBIOSDecode(), nil
	case "xor":
		if len(step.Value) == 0 {
			return nil, nil, fmt.Errorf("xor requires a non-empty value")
		}
		k := step.Value[0]
		return XOR(k), XOR(k), nil
	case "mask":
		return Mask(step.Value), Mask(step.Value), nil
	case "prepend":
		return Prepend(step.Value), StripPrefix(step.Value), nil
	case "append":
		return Append(step.Value), StripSuffix(step.Value), nil
	case "print":
		return Print(), Print(), nil
	default:
		return Print(), Print(), nil
	}
}

func chain(funcs []TransformFunc) TransformFunc {
	return func(data []byte) ([]byte, error) {
		var err error
		for _, fn := range funcs {
			data, err = fn(data)
			if err != nil {
				return nil, err
			}
		}
		return data, nil
	}
}
