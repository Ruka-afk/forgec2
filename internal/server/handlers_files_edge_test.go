package server

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSplitPushChunks(t *testing.T) {
	// Small file: single chunk at offset 0.
	small := []byte("hello")
	ch := splitPushChunks(small)
	if len(ch) != 1 || ch[0].offset != 0 {
		t.Fatalf("small file must yield 1 chunk at offset 0, got %+v", ch)
	}
	if got, _ := base64.StdEncoding.DecodeString(ch[0].data); string(got) != "hello" {
		t.Fatalf("chunk round-trip mismatch: %q", got)
	}

	// Empty file: single empty chunk (preserves legacy single-task path).
	if ch := splitPushChunks(nil); len(ch) != 1 || ch[0].offset != 0 {
		t.Fatalf("empty file must yield 1 chunk, got %+v", ch)
	}

	// Large file: ceil(n/cap) chunks, contiguous offsets, full round-trip.
	n := 2*MaxTransferChunkSize + 12345
	raw := make([]byte, n)
	for i := range raw {
		raw[i] = byte(i * 31)
	}
	chunks := splitPushChunks(raw)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	var reassembled []byte
	for i, c := range chunks {
		if c.offset != int64(i*MaxTransferChunkSize) {
			t.Fatalf("chunk %d offset = %d, want %d", i, c.offset, i*MaxTransferChunkSize)
		}
		dec, err := base64.StdEncoding.DecodeString(c.data)
		if err != nil {
			t.Fatalf("chunk %d decode: %v", i, err)
		}
		if len(dec) > MaxTransferChunkSize {
			t.Fatalf("chunk %d size %d exceeds cap %d", i, len(dec), MaxTransferChunkSize)
		}
		reassembled = append(reassembled, dec...)
	}
	if string(reassembled) != string(raw) {
		t.Fatal("chunk reassembly mismatch")
	}
}

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
