package obfuscation

import (
	"encoding/base64"
	"fmt"
)

// GenerateCommandLineOneLiner creates a complete cmd/powershell one-liner ready for execution
// Uses [ScriptBlock]::Create to avoid command line length limitations
func GenerateCommandLineOneLiner(code string) string {
	// Use UTF-8 Base64 encoding and ScriptBlock::Create to avoid length limits
	b64 := base64.StdEncoding.EncodeToString([]byte(code))
	
	// Use [ScriptBlock]::Create method to bypass command line length limitation
	// This method loads the script from base64 decoded string in memory
	return fmt.Sprintf(`powershell -nop -w hidden -c ([ScriptBlock]::Create([System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s')))).Invoke()`, b64)
}

// encodeUTF16LE converts a string to UTF-16LE byte slice (no BOM for PowerShell -enc)
func encodeUTF16LE(s string) []byte {
	encoded := make([]byte, 0, len(s)*2)
	
	for _, r := range s {
		// Little-endian: low byte first, high byte second
		encoded = append(encoded, byte(r), byte(r>>8))
	}
	
	return encoded
}
