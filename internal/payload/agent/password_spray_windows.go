//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

// trySprayAuth: logonMail uses LOGON32_LOGON_INTERACTIVE (2) and
// LOGON32_PROVIDER_DEFAULT (0) — declared once in token_windows.go.

func trySprayAuth(domain, user, password, dc string) (string, string) {
	usernameUTF16, _ := syscall.UTF16PtrFromString(user)
	domainUTF16, _ := syscall.UTF16PtrFromString(domain)
	passwordUTF16, _ := syscall.UTF16PtrFromString(password)

	var token syscall.Token
	ret, _, le := procLogonUserW.Call(
		uintptr(unsafe.Pointer(usernameUTF16)),
		uintptr(unsafe.Pointer(domainUTF16)),
		uintptr(unsafe.Pointer(passwordUTF16)),
		uintptr(logon32LogonInteractive),
		uintptr(logon32ProviderDefault),
		uintptr(unsafe.Pointer(&token)),
	)

	if ret != 0 {
		token.Close()
		return "valid", ""
	}

	var errCode uint32
	var sysErr syscall.Errno
	if errors.As(le, &sysErr) {
		errCode = uint32(sysErr)
	} else if le != nil {
		return "error", le.Error()
	} else {
		return "error", "unknown logon failure"
	}

	switch errCode {
	case 1326, 1327, 1328:
		return "invalid", ""
	case 1909:
		return "locked", "account locked"
	case 1317:
		return "invalid", "account disabled"
	case 1330:
		return "valid", "password expired"
	case 1907:
		return "valid", "password must change"
	case 1314:
		return "error", "insufficient privileges"
	default:
		return "error", fmt.Sprintf("LogonUser error %d", errCode)
	}
}
