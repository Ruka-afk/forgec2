package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/payload"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
)

func TestValidateCommandArg(t *testing.T) {
	tests := []struct {
		name    string
		v       string
		maxLen  int
		wantErr bool
	}{
		{"normal", "createremotethread", 64, false},
		{"pipe delimiter", "a|b", 64, true},
		{"newline", "foo\nbar", 64, true},
		{"null byte", "foo\x00bar", 64, true},
		{"too long", strings.Repeat("x", 65), 64, true},
		{"at max length", strings.Repeat("x", 64), 64, false},
		{"empty", "", 64, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCommandArg(tc.v, tc.maxLen, "test")
			if tc.wantErr && err == nil {
				t.Errorf("expected error for %q", tc.v)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.v, err)
			}
		})
	}
}

func TestClampSleepString(t *testing.T) {
	cases := []struct {
		in, want string
		minI, minJ int
	}{
		{"5,5", "30,20", 30, 20},
		{"60,50", "60,50", 30, 20},
		{"not-a-sleep", "not-a-sleep", 30, 20},
		{"5", "5", 30, 20},
		{"5,5", "5,5", 1, 0},
	}
	for _, tc := range cases {
		if got := clampSleepString(tc.in, tc.minI, tc.minJ); got != tc.want {
			t.Errorf("clampSleepString(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHandleSetSleepHonorsMinInterval(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTasksTestServer(t)
	s.cfg = config.DefaultConfig()
	s.cfg.Implant.MinInterval = 30
	s.cfg.Implant.MinJitter = 20
	if err := s.db.Create(&db.Implant{ID: "a1", Hostname: "TEST"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	post := func(body, ctype string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/agents/a1/set_sleep", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", ctype)
		s.handleSetSleep(c)
		return w
	}

	// Form path below the floor must be clamped, not rejected.
	if w := post("sleep=5%2C5", "application/x-www-form-urlencoded"); w.Code != http.StatusOK {
		t.Fatalf("form: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	// JSON path below the floor must be clamped too.
	if w := post(`{"interval":10,"jitter":5}`, "application/json"); w.Code != http.StatusOK {
		t.Fatalf("json: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var tasks []db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "set_sleep").Order("id").Find(&tasks).Error; err != nil {
		t.Fatalf("query tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Command != "30,20" {
			t.Errorf("command = %q, want clamped %q", task.Command, "30,20")
		}
	}
}

func TestBuildImplantConfigFallsBackToDefaultWorkingHours(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newTasksTestServer(t)
	s.cfg = config.DefaultConfig()
	s.cfg.Server.BeaconKey = strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	s.regSecrets = crypto.NewRegSecretStore(make([]byte, 32))
	s.cfg.Implant.DefaultWorkingStart = "09:00"
	s.cfg.Implant.DefaultWorkingEnd = "18:00"
	s.cfg.Implant.DefaultWorkingTZ = "UTC"

	cfg, err := s.buildImplantConfig(&binaryGenForm{Architecture: "amd64"})
	if err != nil {
		t.Fatalf("buildImplantConfig: %v", err)
	}
	// Normalize applies the documented precedence explicit form > profile >
	// server default (dataDir without profiles → default profile).
	payload.NormalizeImplantConfig(&cfg, t.TempDir())
	if cfg.WorkingStart != "09:00" || cfg.WorkingEnd != "18:00" || cfg.WorkingTZ != "UTC" {
		t.Errorf("working hours = %q/%q/%q, want server default 09:00/18:00/UTC",
			cfg.WorkingStart, cfg.WorkingEnd, cfg.WorkingTZ)
	}

	// Explicit form values win over the server default.
	cfg, err = s.buildImplantConfig(&binaryGenForm{Architecture: "amd64", WorkingStart: "01:00", WorkingEnd: "02:00", WorkingTZ: "UTC"})
	if err != nil {
		t.Fatalf("buildImplantConfig: %v", err)
	}
	payload.NormalizeImplantConfig(&cfg, t.TempDir())
	if cfg.WorkingStart != "01:00" || cfg.WorkingEnd != "02:00" {
		t.Errorf("explicit working hours lost: %q/%q", cfg.WorkingStart, cfg.WorkingEnd)
	}
}

func TestHandleGetTaskStatusEnforcesAgentScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)
	for _, agent := range []db.Implant{
		{ID: "agent-a", Hostname: "HOST-A"},
		{ID: "agent-b", Hostname: "HOST-B"},
	} {
		if err := s.db.Create(&agent).Error; err != nil {
			t.Fatalf("seed agent %s: %v", agent.ID, err)
		}
	}
	task := db.Task{AgentID: "agent-a", Type: "shell", Status: "completed", Result: "ok"}
	if err := s.db.Create(&task).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	request := func(agentID string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{
			{Key: "id", Value: agentID},
			{Key: "taskId", Value: strconv.FormatUint(uint64(task.ID), 10)},
		}
		c.Request, _ = http.NewRequest(http.MethodGet, "/api/v1/agents/"+agentID+"/tasks/"+strconv.FormatUint(uint64(task.ID), 10), nil)
		s.handleGetTaskStatus(c)
		return w
	}

	if w := request("agent-a"); w.Code != http.StatusOK {
		t.Fatalf("matching agent: expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if w := request("agent-b"); w.Code != http.StatusNotFound {
		t.Fatalf("mismatched agent: expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

func newInjectTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{
		db:                testutil.SetupTestDB(t),
		cfg:               &config.Config{},
		ctx:               context.Background(),
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks: make(map[string]int),
		metrics:           &MetricsCollector{TasksTotal: prometheus.NewCounter(prometheus.CounterOpts{})},
	}
}

func doInjectRequest(s *Server, t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/inject", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "operator")
	s.handleInject(c)
	return w
}

func TestHandleInject_RejectsInvalidInput(t *testing.T) {
	validSC := base64.StdEncoding.EncodeToString([]byte{0x90, 0x90, 0x90})

	t.Run("invalid pid", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"abc"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("zero pid", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"0"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("invalid base64", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {"!!!not base64!!!"}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("empty shellcode", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {""}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("oversized shellcode", func(t *testing.T) {
		s := newInjectTestServer(t)
		big := base64.StdEncoding.EncodeToString(make([]byte, MaxShellcodeSize+1))
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "shellcode": {big}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique with pipe", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "tech": {"crt|evil"}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique too long", func(t *testing.T) {
		s := newInjectTestServer(t)
		w := doInjectRequest(s, t, url.Values{"pid": {"1234"}, "tech": {strings.Repeat("x", MaxTechniqueLength+1)}, "shellcode": {validSC}})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func TestHandleInject_CreatesTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)
	if err := s.db.Create(&db.Implant{ID: "a1", Hostname: "TEST", IP: "10.0.0.1", LastSeen: time.Now()}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	sc := []byte{0x90, 0x90, 0x90}
	scB64 := base64.StdEncoding.EncodeToString(sc)
	w := doInjectRequest(s, t, url.Values{"pid": {"4321"}, "tech": {"crt"}, "shellcode": {scB64}})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body=%s", w.Code, w.Body.String())
	}

	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "inject").First(&task).Error; err != nil {
		t.Fatalf("task not created: %v", err)
	}
	if task.Command != "4321|crt" {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.Data != scB64 {
		t.Errorf("shellcode payload not stored")
	}
}

func TestHandleSpawn_RejectsInvalidArgs(t *testing.T) {
	t.Run("target with pipe", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		s := newInjectTestServer(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/spawn", strings.NewReader(url.Values{"target": {"rundll32|evil"}}.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.Set("user_role", "operator")
		s.handleSpawn(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("technique with newline", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		s := newInjectTestServer(t)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Params = gin.Params{{Key: "id", Value: "a1"}}
		c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/spawn", strings.NewReader(url.Values{"technique": {"CreateRemoteThread\npwn"}}.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c.Set("user_role", "operator")
		s.handleSpawn(c)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})
}

func doSendCommandRequest(s *Server, t *testing.T, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	agent := db.Implant{
		ID:       "a1",
		Hostname: "TEST-HOST",
		Username: "testuser",
		OS:       "windows",
		Arch:     "amd64",
		IP:       "10.0.0.1",
	}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/agents/a1/send_command", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Set("user_role", "operator")
	s.handleSendCommand(c)
	return w
}

func TestHandleSendCommand_InvalidCallbackLeavesNoTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	// Private-IP callback URL must be rejected by validateCallbackURL.
	form := url.Values{"command": {"whoami"}, "callback_url": {"http://10.0.0.5/cb"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for private callback URL, got %d", w.Code)
	}

	// No task must have been persisted and pending count must remain 0.
	var count int64
	if err := s.db.Model(&db.Task{}).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks after rejected callback, got %d", count)
	}
	if got := s.agentPendingTasks["a1"]; got != 0 {
		t.Fatalf("pending count leaked: got %d, want 0", got)
	}
}

func TestHandleSendCommand_InvalidCallbackMethodLeavesNoTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	form := url.Values{"command": {"whoami"}, "callback_url": {"http://8.8.8.8/cb"}, "callback_method": {"FROB"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid callback method, got %d", w.Code)
	}

	var count int64
	if err := s.db.Model(&db.Task{}).Count(&count).Error; err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 tasks after rejected callback method, got %d", count)
	}
	if got := s.agentPendingTasks["a1"]; got != 0 {
		t.Fatalf("pending count leaked: got %d, want 0", got)
	}
}

func TestHandleSendCommand_ValidCreatesTaskAndPendingCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)

	form := url.Values{"command": {"whoami"}, "callback_url": {"http://8.8.8.8/cb"}, "callback_method": {"GET"}}
	w := doSendCommandRequest(s, t, form)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "shell").First(&task).Error; err != nil {
		t.Fatalf("task not created: %v", err)
	}
	if task.Command != "whoami" {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.CallbackURL != "http://8.8.8.8/cb" {
		t.Errorf("callback URL not stored: %q", task.CallbackURL)
	}
	if task.CallbackMethod != "GET" {
		t.Errorf("callback method not stored: %q", task.CallbackMethod)
	}
	if got := s.agentPendingTasks["a1"]; got != 1 {
		t.Fatalf("pending count: got %d, want 1", got)
	}
}

func TestHandleSendCommand_JSONBodyCreatesTask(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := newInjectTestServer(t)
	agent := db.Implant{
		ID:       "a1",
		Hostname: "TEST-HOST",
		Username: "testuser",
		OS:       "windows",
		Arch:     "amd64",
		IP:       "10.0.0.1",
	}
	if err := s.db.Create(&agent).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "a1"}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/agents/a1/command", strings.NewReader(`{"command":"whoami","shell":"cmd.exe"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_role", "operator")
	s.handleSendCommand(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var task db.Task
	if err := s.db.Where("agent_id = ? AND type = ?", "a1", "shell").First(&task).Error; err != nil {
		t.Fatalf("task not created: %v", err)
	}
	if task.Command != "whoami" {
		t.Errorf("unexpected command %q", task.Command)
	}
	if task.Shell != "cmd.exe" {
		t.Errorf("unexpected shell %q", task.Shell)
	}
}
