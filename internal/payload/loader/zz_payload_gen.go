//go:build windows

// Package-level stub declarations for the loader package when it compiles as
// part of the main module (go build ./...). A real build of the loader happens
// in a throwaway module where writePayloadGen materializes a zz_payload_gen.go
// with the actual per-build payload; that generated file overwrites this stub
// (same filename) in the build directory, and the stub here is only so the
// source directory compiles standalone. The loader is never run from a build
// that carries these zero values.
package main

var (
	payloadBlob   []byte
	payloadKey    []byte
	payloadMethod string
	payloadEntry  string
)
