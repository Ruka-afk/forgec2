package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

type testErr string

func (e testErr) Error() string { return string(e) }

func errForTest(msg string) error { return testErr(msg) }

func seedIssueAgent(t *testing.T, s *Server, id string) {
	t.Helper()
	if err := s.db.Create(&db.Implant{ID: id}).Error; err != nil {
		t.Fatalf("seed implant: %v", err)
	}
}

func issueTestContext(id string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodPost, "/agents/"+id+"/ps", nil)
	c.Params = gin.Params{{Key: "id", Value: id}}
	return c, w
}

func TestClientErrorStatus_Mapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		ok     bool
	}{
		{"pending limit", errForTest("agent x has 5 pending tasks (limit 5)"), http.StatusTooManyRequests, true},
		{"chrome affinity", errForTest("task requires a chrome-tagged agent"), http.StatusForbidden, true},
		{"chrome unsupported", errForTest("op is not supported on chrome extension agents"), http.StatusForbidden, true},
		{"lportfwd disabled", errForTest("lportfwd is disabled by server configuration (server.lportfwd_enabled)"), http.StatusForbidden, true},
		{"opsec block", errForTest("blocked by adaptive opsec: mimikatz is not allowed on a critical-threat host"), http.StatusForbidden, true},
		{"unknown type", errForTest("unknown task type: bogus"), http.StatusBadRequest, true},
		{"missing command", errForTest("task type shell requires 'command' parameter"), http.StatusBadRequest, true},
		{"missing shell", errForTest("task type shell requires 'shell' parameter"), http.StatusBadRequest, true},
		{"missing path", errForTest("task type upload requires 'path' parameter"), http.StatusBadRequest, true},
		{"missing data", errForTest("task type upload requires 'data' parameter"), http.StatusBadRequest, true},
		{"too long", errForTest("command too long (max 8192 characters)"), http.StatusBadRequest, true},
		{"nil", nil, 0, false},
		{"db error", errForTest("FOREIGN KEY constraint failed"), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, ok := clientErrorStatus(tc.err)
			if ok != tc.ok || status != tc.status {
				t.Fatalf("clientErrorStatus(%v) = (%d, %v), want (%d, %v)", tc.err, status, ok, tc.status, tc.ok)
			}
		})
	}
}

func TestRespondTaskError_StatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"conflict", errForTest("agent conflict: bob is being actively operated by alice"), http.StatusConflict},
		{"validation", errForTest("unknown task type: bogus"), http.StatusBadRequest},
		{"policy", errForTest("blocked by adaptive opsec: x is not allowed on a critical-threat host"), http.StatusForbidden},
		{"limit", errForTest("agent x has 5 pending tasks (limit 5)"), http.StatusTooManyRequests},
		{"internal", errForTest("FOREIGN KEY constraint failed"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(http.MethodPost, "/x", nil)
			respondTaskError(c, tc.err)
			if w.Code != tc.status {
				t.Fatalf("expected %d, got %d; body=%s", tc.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestIssueAgentTask_SuccessWritesNothing(t *testing.T) {
	s := newTasksTestServer(t)
	seedIssueAgent(t, s, "agent-issue-ok")
	c, w := issueTestContext("agent-issue-ok")

	task := s.issueAgentTask(c, "agent-issue-ok", TaskSpec{Type: "ps"})
	if task == nil {
		t.Fatalf("expected task, got nil; body=%s", w.Body.String())
	}
	if task.Type != "ps" || task.AgentID != "agent-issue-ok" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("issueAgentTask must not write a success response; body=%s", w.Body.String())
	}
}

func TestIssueAgentTask_UnknownAgent404(t *testing.T) {
	s := newTasksTestServer(t)
	c, w := issueTestContext("agent-issue-missing")

	if task := s.issueAgentTask(c, "agent-issue-missing", TaskSpec{Type: "ps"}); task != nil {
		t.Fatal("expected nil task for unknown agent")
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestIssueAgentTask_UnknownType400(t *testing.T) {
	s := newTasksTestServer(t)
	seedIssueAgent(t, s, "agent-issue-badtype")
	c, w := issueTestContext("agent-issue-badtype")

	if task := s.issueAgentTask(c, "agent-issue-badtype", TaskSpec{Type: "no_such_task_xyz"}); task != nil {
		t.Fatal("expected nil task for unknown type")
	}
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body=%s", w.Code, w.Body.String())
	}
}

func TestIssueAgentTask_EnforcesSoftLock(t *testing.T) {
	s := newTasksTestServer(t)
	seedIssueAgent(t, s, "agent-issue-lock")
	s.operatorSessions = &operatorSessionTracker{sessions: make(map[uint]*WSOperatorSession)}
	s.operatorSessions.sessions[99] = newPresenceSession(99, "other_operator", "agent-issue-lock", time.Now())

	c, w := issueTestContext("agent-issue-lock")
	c.Set("user_id", uint(1))

	// Another operator is viewing: the caller identity must flow through
	// issueAgentTask into the soft-lock gate (previously dropped at call sites
	// that forgot callerOpts).
	if task := s.issueAgentTask(c, "agent-issue-lock", TaskSpec{Type: "ps"}); task != nil {
		t.Fatal("expected soft-lock to block the task")
	}
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d; body=%s", w.Code, w.Body.String())
	}

	// Same operator viewing their own agent: allowed.
	c2, w2 := issueTestContext("agent-issue-lock")
	c2.Set("user_id", uint(99))
	if task := s.issueAgentTask(c2, "agent-issue-lock", TaskSpec{Type: "ps"}); task == nil {
		t.Fatalf("same-operator issue should succeed; body=%s", w2.Body.String())
	}
}
