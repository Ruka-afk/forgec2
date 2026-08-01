package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleUploadFile_PathTraversalPrevention(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("safeJoin prevents traversal", func(t *testing.T) {
		result := safeJoin(`C:\Base`, `..\..\Windows\System32\config\SAM`)
		if result != "" {
			t.Errorf("path traversal should return empty string, got %q", result)
		}
	})

	t.Run("safeJoin allows normal paths", func(t *testing.T) {
		result := safeJoin(`C:\Base`, `Users\Admin\file.txt`)
		if !strings.HasPrefix(result, `C:\Base`) {
			t.Errorf("expected path under C:\\Base, got %q", result)
		}
	})
}

func TestHandleDownload_InvalidURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := &Server{db: database, agentPendingTasks: make(map[string]int)}
	agent := seedImplant(t, database)

	t.Run("missing url returns 400", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\temp\file.bin`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleDownload(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d; body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("missing path returns 400", func(t *testing.T) {
		form := url.Values{}
		form.Set("url", "https://example.com/file.bin")
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleDownload(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d; body=%s", w.Code, w.Body.String())
		}
	})
}

func TestHandleFileRead_AgentNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := &Server{db: database, agentPendingTasks: make(map[string]int)}

	form := url.Values{}
	form.Set("path", `C:\test.txt`)
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent-agent"}}
	srv.handleFileRead(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestHandleFileDelete_AgentNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := &Server{db: database, agentPendingTasks: make(map[string]int)}

	form := url.Values{}
	form.Set("path", `C:\temp\file.txt`)
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent-agent"}}
	srv.handleFileDelete(c)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
}
