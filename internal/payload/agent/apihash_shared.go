//go:build linux || windows || darwin
// +build linux windows darwin

package main

// fnv1a32 hashes an API name. The digest (not the name) is embedded at call
// sites; candidates are decrypted from the strxor table at runtime and
// matched by digest. Shared across all platforms so the vectors are testable
// on linux CI; the resolver itself stays windows-only.
func fnv1a32(s string) uint32 {
	const (
		offset = 2166136261
		prime  = 16777619
	)
	h := uint32(offset)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime
	}
	return h
}
