//go:build windows && wintun && amd64

package main

import _ "embed"

// wintunDLLBytes carries the signed amd64 Wintun runtime (wintun 0.14.1)
// staged by scripts/fetch-wintun.ps1. The generator adds the wintun build tag
// only when this asset is present in the build tree; builds without it keep
// the honest stub in tun_stub.go.
//
//go:embed assets/wintun/amd64/wintun.dll
var wintunDLLBytes []byte
