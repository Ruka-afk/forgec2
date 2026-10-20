package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

func newJSONFileScanContext(t *testing.T, body string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("user_role", "operator")
	return w, c
}

func TestHandleFileScanQueuesStructuredTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	body := `{"path":"C:\\Users\\Public","pattern":"*.txt,*.log","rules":"email,bank_card,aws_key","include_hidden":true,"max_depth":99,"max_files":99999,"max_file_bytes":99999999,"max_total_bytes":99999999,"max_findings":99999,"timeout_seconds":999}`
	w, c := newJSONFileScanContext(t, body)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleFileScan(c)
	assertStatus(t, w, http.StatusOK)
	if assertSuccessJSON(t, w)["task_id"] == nil {
		t.Fatalf("missing task_id: %s", w.Body.String())
	}

	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_scan").First(&task).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Path != `C:\Users\Public` || task.Command != "*.txt,*.log" {
		t.Fatalf("task path/command = %q / %q", task.Path, task.Command)
	}
	var opts struct {
		Patterns       []string `json:"patterns"`
		Rules          []string `json:"rules"`
		IncludeHidden  bool     `json:"include_hidden"`
		MaxDepth       int      `json:"max_depth"`
		MaxFiles       int      `json:"max_files"`
		MaxFileBytes   int64    `json:"max_file_bytes"`
		MaxTotalBytes  int64    `json:"max_total_bytes"`
		MaxFindings    int      `json:"max_findings"`
		TimeoutSeconds int      `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
		t.Fatalf("decode opts: %v; data=%s", err, task.Data)
	}
	if len(opts.Patterns) != 2 || opts.Patterns[0] != "*.txt" {
		t.Errorf("patterns = %#v", opts.Patterns)
	}
	if len(opts.Rules) != 3 || opts.Rules[0] != "email" || opts.Rules[1] != "bank_card" || opts.Rules[2] != "aws_key" {
		t.Errorf("rules = %#v", opts.Rules)
	}
	if !opts.IncludeHidden {
		t.Error("include_hidden = false")
	}
	if opts.MaxDepth != fileScanHardMaxDepth || opts.MaxFiles != fileScanHardMaxFiles ||
		opts.MaxFileBytes != fileScanHardMaxFileBytes || opts.MaxTotalBytes != fileScanHardMaxTotalBytes ||
		opts.MaxFindings != fileScanHardMaxFindings || opts.TimeoutSeconds != fileScanHardTimeout {
		t.Errorf("limits not clamped: %+v", opts)
	}
}

func TestHandleFileScanDefaultsAndAllRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	form := url.Values{"path": {`C:\Users\Public`}, "rules": {"all"}}
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleFileScan(c)
	assertStatus(t, w, http.StatusOK)

	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_scan").Order("id desc").First(&task).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if task.Command != "*" {
		t.Errorf("default command = %q, want *", task.Command)
	}
	var opts struct {
		Rules          []string `json:"rules"`
		MaxFiles       int      `json:"max_files"`
		MaxFileBytes   int64    `json:"max_file_bytes"`
		MaxTotalBytes  int64    `json:"max_total_bytes"`
		MaxFindings    int      `json:"max_findings"`
		TimeoutSeconds int      `json:"timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
		t.Fatalf("decode opts: %v", err)
	}
	if len(opts.Rules) != len(fileScanAllowedRules) {
		t.Errorf("all rules = %#v", opts.Rules)
	}
	if opts.MaxFiles != fileScanDefaultMaxFiles || opts.MaxFileBytes != fileScanDefaultMaxFileBytes ||
		opts.MaxTotalBytes != fileScanDefaultMaxTotalBytes || opts.MaxFindings != fileScanDefaultMaxFindings ||
		opts.TimeoutSeconds != fileScanDefaultTimeout {
		t.Errorf("defaults = %+v", opts)
	}
}

func TestHandleFileScanDefaultsToCoreRules(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	form := url.Values{"path": {`/home/operator`}}
	w, c := newFormContext(http.MethodPost, "/", &form)
	c.Params = gin.Params{{Key: "id", Value: agent.ID}}
	srv.handleFileScan(c)
	assertStatus(t, w, http.StatusOK)
	var task db.Task
	if err := database.Where("agent_id = ? AND type = ?", agent.ID, "file_scan").Order("id desc").First(&task).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	var opts struct {
		Rules []string `json:"rules"`
	}
	if err := json.Unmarshal([]byte(task.Data), &opts); err != nil {
		t.Fatalf("decode opts: %v", err)
	}
	if len(opts.Rules) != 3 || opts.Rules[0] != "email" || opts.Rules[1] != "phone" || opts.Rules[2] != "bank_card" {
		t.Errorf("default rules = %#v", opts.Rules)
	}
}

func TestHandleFileScanValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	database := newContractDB(t)
	srv := newTestFileServer(t, database)
	agent := seedImplant(t, database)

	cases := []struct {
		name string
		form url.Values
	}{
		{name: "missing path", form: nil},
		{name: "unknown rule", form: url.Values{"path": {`C:\Users`}, "rules": {"email,nope"}}},
		{name: "negative max files", form: url.Values{"path": {`C:\Users`}, "max_files": {"-1"}}},
		{name: "negative timeout", form: url.Values{"path": {`C:\Users`}, "timeout_seconds": {"-1"}}},
		{name: "nul path", form: url.Values{"path": {"C:\\Users\x00Public"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, c := newFormContext(http.MethodPost, "/", &tc.form)
			c.Params = gin.Params{{Key: "id", Value: agent.ID}}
			srv.handleFileScan(c)
			assertStatus(t, w, http.StatusBadRequest)
		})
	}

	t.Run("unknown agent", func(t *testing.T) {
		form := url.Values{"path": {`C:\Users`}}
		w, c := newFormContext(http.MethodPost, "/", &form)
		c.Params = gin.Params{{Key: "id", Value: "missing"}}
		srv.handleFileScan(c)
		assertStatus(t, w, http.StatusNotFound)
	})
}
