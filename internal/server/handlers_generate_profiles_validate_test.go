package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/gin-gonic/gin"
)

// validateProfileWarnings posts a v2 JSON profile to handleValidateProfile
// and returns the warnings list.
func validateProfileWarnings(t *testing.T, profileJSON string) []string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	s := &Server{cfg: &config.Config{}}
	body, _ := json.Marshal(map[string]string{
		"name":    "t",
		"format":  "json",
		"content": profileJSON,
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/generate/profile/validate", strings.NewReader(string(body)))
	c.Request.Header.Set("Content-Type", "application/json")
	s.handleValidateProfile(c)
	if w.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Warnings []string `json:"warnings"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode validate response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("validate success=false: %s", w.Body.String())
	}
	if resp.Data.Warnings == nil {
		return []string{}
	}
	return resp.Data.Warnings
}

func warningsContain(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

// TestValidateProfileLossyCase proves the validate endpoint catches lossy
// server_output chains: "case" round-trips lowercase input but corrupts
// mixed-case envelopes, so it must warn.
func TestValidateProfileLossyCase(t *testing.T) {
	ws := validateProfileWarnings(t, `{"name":"t","server_output":[{"type":"case"}]}`)
	if !warningsContain(ws, "lossy for mixed-case") {
		t.Fatalf("case chain should warn lossy, got %v", ws)
	}
}

// TestValidateProfileCImplantWarning proves any non-empty server_output chain
// warns that the C implant cannot decode it.
func TestValidateProfileCImplantWarning(t *testing.T) {
	ws := validateProfileWarnings(t, `{"name":"t","server_output":[{"type":"base64"}]}`)
	if !warningsContain(ws, "C implant") {
		t.Fatalf("base64 chain should warn C-implant incompatibility, got %v", ws)
	}
	if warningsContain(ws, "lossy for mixed-case") {
		t.Fatalf("base64 chain must not warn lossy, got %v", ws)
	}
}

// TestValidateProfileCleanChain proves a lossless chain yields only the
// C-implant notice and no round-trip warnings.
func TestValidateProfileCleanChain(t *testing.T) {
	ws := validateProfileWarnings(t, `{"name":"t","server_output":[{"type":"base64"},{"type":"xor","value":"k"}]}`)
	for _, w := range ws {
		if strings.Contains(w, "round-trip") || strings.Contains(w, "lossy") || strings.Contains(w, "encode failed") {
			t.Fatalf("clean chain should not warn round-trip, got %v", ws)
		}
	}
}

// TestValidateProfileNoServerOutput proves profiles without server_output
// stay warning-free on this axis.
func TestValidateProfileNoServerOutput(t *testing.T) {
	ws := validateProfileWarnings(t, `{"name":"plain"}`)
	if warningsContain(ws, "server_output") || warningsContain(ws, "C implant") {
		t.Fatalf("plain profile should not warn server_output, got %v", ws)
	}
}
