//go:build windows && wintun && !amd64 && !arm64

package main

// Windows architectures without a staged Wintun runtime. The shared start path
// reports the honest "no bundled runtime" error instead of failing to compile;
// the generator only ever requests amd64/arm64 (see supportedArchByOS).
var wintunDLLBytes []byte
