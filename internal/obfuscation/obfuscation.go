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


