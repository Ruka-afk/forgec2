package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestHandleAPISendChatMessage_SizeLimit(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t)}

	send := func(msg string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/chat/send",
			strings.NewReader(`{"message":`+strconv.Quote(msg)+`,"channel":"general"}`))
		s.handleAPISendChatMessage(c)
		return w
	}

	if w := send(strings.Repeat("a", MaxChatMessageBytes)); w.Code != http.StatusOK {
		t.Fatalf("boundary message (exactly max) rejected: %d %s", w.Code, w.Body.String())
	}
	if w := send(strings.Repeat("a", MaxChatMessageBytes+1)); w.Code != http.StatusBadRequest {
		t.Fatalf("oversized message accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestHandleSocks_RejectsInvalidPort(t *testing.T) {
	s := &Server{db: testutil.SetupTestDB(t)}

	for _, port := range []string{"0", "-1", "65536", "abc", "1e3"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest(http.MethodPost, "/agents/a1/socks",
			strings.NewReader("port="+port))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.Set("user_role", "operator")
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		s.handleSocks(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("port %q: expected 400, got %d; body=%s", port, w.Code, w.Body.String())
		}
	}
}
