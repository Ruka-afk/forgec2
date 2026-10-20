package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func makeTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, image.White)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestHandleFileThumbQueuesBoundedBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	srv.cfg.Server.DataDir = t.TempDir()
	agent := seedImplant(t, database)

	paths, _ := json.Marshal([]string{`C:\Users\a\one.jpg`, `C:\Users\a\two.mp4`})
	form := url.Values{"paths": {string(paths)}}
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleFileThumb(c)
	assertStatus(t, w, http.StatusOK)
	if assertSuccessJSON(t, w)["task_id"] == nil {
		t.Fatalf("missing task_id: %s", w.Body.String())
	}

	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_thumb").First(&task).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Command != "2 file(s)" {
		t.Errorf("command = %q", task.Command)
	}
	var opts struct {
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
		t.Fatalf("decode task data: %v", err)
	}
	if len(opts.Paths) != 2 || opts.Paths[0] != `C:\Users\a\one.jpg` {
		t.Errorf("paths = %#v", opts.Paths)
	}
}

func TestHandleFileThumbValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	srv.cfg.Server.DataDir = t.TempDir()
	agent := seedImplant(t, database)

	cases := []struct {
		name string
		form url.Values
	}{
		{name: "missing paths", form: nil},
		{name: "empty path", form: url.Values{"paths": {`["  "]`}}},
		{name: "nul byte", form: url.Values{"paths": {`["C:\\a\u0000.jpg"]`}}},
		{name: "too many", form: url.Values{"paths": {`["a","b","c","d","e","f","g","h","i","j","k","l","m"]`}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newFormContext(http.MethodPost, "/", &tc.form)
			c.Params = gin.Params{{Key: "id", Value: agent.ID}}
			srv.handleFileThumb(c)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}
}

func TestStoreFileThumbsCachesAndServesJPEG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	srv.cfg.Server.DataDir = t.TempDir()
	agent := seedImplant(t, database)

	jpegBytes := makeTestJPEG(t)
	raw, _ := json.Marshal(fileThumbWireResult{Items: []fileThumbWireItem{{
		Path:       `C:\Users\a\one.jpg`,
		OK:         true,
		MIME:       "image/jpeg",
		Width:      4,
		Height:     3,
		SourceSize: 123,
		Modified:   "2026-10-20T10:00:00Z",
		Data:       base64.StdEncoding.EncodeToString(jpegBytes),
	}}})
	stored := srv.storeFileThumbs(agent.ID, string(raw))
	if bytes.Contains([]byte(stored), []byte(base64.StdEncoding.EncodeToString(jpegBytes))) {
		t.Fatalf("stored task result still contains base64 image: %s", stored)
	}
	var doc fileThumbWireResult
	if err := json.Unmarshal([]byte(stored), &doc); err != nil {
		t.Fatalf("decode stored result: %v", err)
	}
	if len(doc.Items) != 1 || !doc.Items[0].OK || !validFileThumbID(doc.Items[0].ThumbID) {
		t.Fatalf("stored item = %#v", doc.Items)
	}

	w, c := newFormContext(http.MethodGet, "/", nil)
	c.Params = gin.Params{
		{Key: "id", Value: agent.ID},
		{Key: "thumb", Value: doc.Items[0].ThumbID},
	}
	srv.handleFileThumbGet(c)
	assertStatus(t, w, http.StatusOK)
	if got := w.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("content-type = %q", got)
	}
	if !bytes.Equal(w.Body.Bytes(), jpegBytes) {
		t.Errorf("served thumbnail does not match source")
	}

	t.Run("invalid id", func(t *testing.T) {
		w, c := newFormContext(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}, {Key: "thumb", Value: "not-hex"}}
		srv.handleFileThumbGet(c)
		assertStatus(t, w, http.StatusBadRequest)
	})
	t.Run("missing id", func(t *testing.T) {
		w, c := newFormContext(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}, {Key: "thumb", Value: strings.Repeat("a", 64)}}
		srv.handleFileThumbGet(c)
		assertStatus(t, w, http.StatusNotFound)
	})
}

func TestStoreFileThumbsRejectsNonJPEG(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	srv.cfg.Server.DataDir = t.TempDir()
	agent := seedImplant(t, database)

	cases := []fileThumbWireItem{
		{Path: "x.png", OK: true, Data: base64.StdEncoding.EncodeToString([]byte("not jpeg"))},
		{Path: "y.png", OK: true},
	}
	for _, item := range cases {
		raw, _ := json.Marshal(fileThumbWireResult{Items: []fileThumbWireItem{item}})
		var doc fileThumbWireResult
		if err := json.Unmarshal([]byte(srv.storeFileThumbs(agent.ID, string(raw))), &doc); err != nil {
			t.Fatalf("decode stored result: %v", err)
		}
		if len(doc.Items) != 1 || doc.Items[0].OK || doc.Items[0].Error == "" {
			t.Fatalf("item = %#v", doc.Items)
		}
	}
}

func TestCleanupOldThumbs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	srv.cfg.Server.DataDir = t.TempDir()
	agent := seedImplant(t, database)
	base := srv.fileThumbBase(agent.ID)
	if base == "" {
		t.Fatal("empty thumb base")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldPath := filepath.Join(base, "old.jpg")
	if err := os.WriteFile(oldPath, []byte("old"), 0600); err != nil {
		t.Fatalf("write old: %v", err)
	}
	old := time.Now().Add(-fileThumbTTL - time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	freshPath := filepath.Join(base, "fresh.jpg")
	if err := os.WriteFile(freshPath, []byte("fresh"), 0600); err != nil {
		t.Fatalf("write fresh: %v", err)
	}
	srv.cleanupOldThumbs()
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old thumbnail still exists: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Errorf("fresh thumbnail removed: %v", err)
	}
}
