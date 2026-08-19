package payload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
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

// AESEncoder implements AES-CTR. The key is zero-padded or truncated to 16
// bytes (as the loader modules do). The IV is NOT stored in the blob: it is
// derived deterministically from the key (SHA-256 prefix), so encoding the
// same payload with the same key always yields the same bytes. That is a
// deliberate property for a packer (blob content is per-build anyway because
// the payload differs per build) and keeps every decoder — Go loader, PS1
// loader — identical without transporting a per-blob IV.
type AESEncoder struct{}

func aesIV(key []byte) []byte {
	sum := sha256.Sum256(key)
	return sum[:aes.BlockSize]
}

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
	ciphertext := make([]byte, len(data))
	stream := cipher.NewCTR(block, aesIV(key))
	stream.XORKeyStream(ciphertext, data)
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
	plaintext := make([]byte, len(data))
	stream := cipher.NewCTR(block, aesIV(key))
	stream.XORKeyStream(plaintext, data)
	return plaintext, nil
}

// sgnStubLen is the size of the self-decoding decoder stub prepended by
// SGNEncoder. Layout (x64, entered at byte 0):
//
//	0x00 EB 10              jmp +0x10            -> 0x12 (the call below)
//	0x02 5A                 pop rdx              ; rdx = payload start
//	0x03 4A                 dec rdx              ; rdx = one byte before payload
//	0x04 33 C9              xor ecx, ecx
//	0x06 66 B9 <len16>      mov cx, len16        ; loop count (patched)
//	0x0A 80 34 0A <key>     xor byte [rdx+rcx], k ; decode one byte (key patched)
//	0x0E 41                 inc ecx
//	0x0F E2 FA              loop 0x0A
//	0x11 EB 05              jmp +0x05            -> 0x18 = payload start
//	0x13 E8 EB FF FF FF     call -0x15           -> 0x02 (pop rdx)
//
// The call pushes the payload start as the return address, so pop rdx lands
// exactly on the first payload byte. The loop count is payloadLen+1 because
// the decoder begins one byte before the payload (rdx was decremented) and
// must cover every payload byte including the last one. The key byte at
// offset 0x0D is patched at encode time so the stub always decodes with the
// same key the blob was encoded with.
const sgnStubLen = 23

const sgnKeyByteOffset = 13

type SGNEncoder struct{}

func (SGNEncoder) Encode(data []byte, key []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("sgn: cannot encode empty payload")
	}
	// The decoder uses a 16-bit loop counter, so the payload (plus the extra
	// iteration for the byte before the payload) must fit in 16 bits.
	if len(data) >= 0xFFFF {
		return nil, fmt.Errorf("sgn: payload too large for 16-bit decoder (max %d bytes)", 0xFFFE)
	}
	k := byte(0xAA)
	if len(key) > 0 {
		k = key[0]
	}
	stub := []byte{
		0xEB, 0x10, 0x5A, 0x4A, 0x33, 0xC9, 0x66, 0xB9,
		0x00, 0x00, 0x80, 0x34, 0x0A, 0x41, 0xE2, 0xFA,
		0xEB, 0x05, 0xE8, 0xEB, 0xFF, 0xFF, 0xFF,
	}
	stub[sgnKeyByteOffset] = k
	binary.LittleEndian.PutUint16(stub[8:10], uint16(len(data)+1))
	encoded, err := XOREncoder{}.Encode(data, []byte{k})
	if err != nil {
		return nil, fmt.Errorf("sgn: %w", err)
	}
	result := make([]byte, 0, sgnStubLen+len(encoded))
	result = append(result, stub...)
	result = append(result, encoded...)
	return result, nil
}

func (SGNEncoder) Decode(data []byte, key []byte) ([]byte, error) {
	if len(data) < sgnStubLen+1 {
		return nil, fmt.Errorf("data too short for sgn decoder")
	}
	// When the caller does not supply a key, fall back to the key embedded in
	// the stub so a blob can be decoded without out-of-band knowledge.
	k := byte(0xAA)
	if len(key) > 0 {
		k = key[0]
	} else {
		k = data[sgnKeyByteOffset]
	}
	decoded, err := XOREncoder{}.Decode(data[sgnStubLen:], []byte{k})
	if err != nil {
		return nil, fmt.Errorf("sgn: %w", err)
	}
	return decoded, nil
}

var (
	registeredEncoders = map[ShellcodeEncode]EncoderPlugin{
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
