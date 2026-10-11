package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// newWorkflowTestServer builds a server for the workflow/group API tests.
func newWorkflowTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Server{db: testutil.SetupTestDB(t), cfg: &config.Config{}}
}

// wfJSONCtx posts a JSON body to a handler under test.
func wfJSONCtx(body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body != "" {
		c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(body)))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	}
	c.Set("user", "wf-op")
	c.Set("user_role", "operator")
	return c, w
}

// TestExecutionStatusPriority pins the aggregate-status derivation: abort
// dominates everything, then running/pending, then failed, then completed.
// A mis-ordered branch would report a failed pipeline as completed.
func TestExecutionStatusPriority(t *testing.T) {
	cases := []struct {
		name     string
		statuses map[string]bool
		want     string
	}{
		{"abort beats all", map[string]bool{"aborted": true, "failed": true, "completed": true}, "aborted"},
		{"running beats failed", map[string]bool{"running": true, "failed": true}, "running"},
		{"pending counts as running", map[string]bool{"pending": true}, "running"},
		{"failed over completed", map[string]bool{"failed": true, "completed": true}, "failed"},
		{"completed only", map[string]bool{"completed": true}, "completed"},
		{"empty set", map[string]bool{}, "completed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := executionStatus(tc.statuses); got != tc.want {
				t.Fatalf("executionStatus(%v) = %q, want %q", tc.statuses, got, tc.want)
			}
		})
	}
}

// TestWorkflowCreateRequiresNameAndDefaultsScope proves the create guards.
func TestWorkflowCreateRequiresNameAndDefaultsScope(t *testing.T) {
	s := newWorkflowTestServer(t)

	c, w := wfJSONCtx(`{"name":""}`)
	s.handleAPICreateWorkflow(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 for a missing name", w.Code)
	}

	c2, w2 := wfJSONCtx(`{"name":"recon-wf"}`)
	s.handleAPICreateWorkflow(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w2.Code, w2.Body.String())
	}
	var stored db.Workflow
	if err := s.db.First(&stored).Error; err != nil {
		t.Fatalf("workflow not persisted: %v", err)
	}
	if stored.ScopeType != "all" {
		t.Fatalf("scope_type = %q, want the default all", stored.ScopeType)
	}
	if !stored.Enabled {
		t.Fatal("a new workflow must default to enabled")
	}
}

// TestWorkflowCreateAppliesStepDefaults proves step ordering and the shell /
// timeout defaults are stamped on create.
func TestWorkflowCreateAppliesStepDefaults(t *testing.T) {
	s := newWorkflowTestServer(t)

	body := `{"name":"stepped","steps":[{"task_type":"shell","command":"whoami"},{"task_type":"ps","shell":"powershell"}]}`
	c, w := wfJSONCtx(body)
	s.handleAPICreateWorkflow(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var wf db.Workflow
	if err := s.db.Preload("Steps").First(&wf).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(wf.Steps) != 2 {
		t.Fatalf("steps persisted = %d, want 2", len(wf.Steps))
	}
	if wf.Steps[0].StepOrder != 1 || wf.Steps[1].StepOrder != 2 {
		t.Fatalf("step order = %d/%d, want 1/2", wf.Steps[0].StepOrder, wf.Steps[1].StepOrder)
	}
	if wf.Steps[0].Shell != "cmd" {
		t.Fatalf("shell default = %q, want cmd", wf.Steps[0].Shell)
	}
	if wf.Steps[0].TimeoutSec != 60 {
		t.Fatalf("timeout default = %d, want 60", wf.Steps[0].TimeoutSec)
	}
	if wf.Steps[1].Shell != "powershell" {
		t.Fatalf("explicit shell lost: %q", wf.Steps[1].Shell)
	}
}

// TestWorkflowDetailAndToggle proves detail lookup (404 for missing) and the
// toggle flips enabled state.
func TestWorkflowDetailAndToggle(t *testing.T) {
	s := newWorkflowTestServer(t)
	seed := db.Workflow{ID: "wf-1", Name: "wf-one", Enabled: true, ScopeType: "all"}
	if err := s.db.Create(&seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := wfJSONCtx("")
	c.Params = gin.Params{{Key: "id", Value: "nope"}}
	s.handleAPIWorkflowsDetail(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("detail status=%d, want 404 for a missing workflow", w.Code)
	}

	c2, w2 := wfJSONCtx("")
	c2.Params = gin.Params{{Key: "id", Value: "wf-1"}}
	s.handleAPIWorkflowsDetail(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("detail status=%d, want 200", w2.Code)
	}

	c3, w3 := wfJSONCtx("")
	c3.Params = gin.Params{{Key: "id", Value: "wf-1"}}
	s.handleAPIWorkflowsToggle(c3)
	if w3.Code != http.StatusOK {
		t.Fatalf("toggle status=%d body=%s", w3.Code, w3.Body.String())
	}
	var reloaded db.Workflow
	if err := s.db.First(&reloaded, "id = ?", "wf-1").Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Enabled {
		t.Fatal("toggle did not flip enabled off")
	}

	c4, w4 := wfJSONCtx("")
	c4.Params = gin.Params{{Key: "id", Value: "missing"}}
	s.handleAPIWorkflowsToggle(c4)
	if w4.Code != http.StatusNotFound {
		t.Fatalf("toggle of a missing workflow: status=%d, want 404", w4.Code)
	}
}

// TestWorkflowExecuteEmptyScopeReportsZero proves a scoped workflow with no
// scope ids short-circuits to zero instead of silently targeting every agent.
func TestWorkflowExecuteEmptyScopeReportsZero(t *testing.T) {
	s := newWorkflowTestServer(t)
	if err := s.db.Create(&db.Workflow{ID: "wf-scope", Name: "scoped", Enabled: true, ScopeType: "agents", ScopeIDs: ""}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A live recent agent exists: if the empty scope fell through to the
	// "recent agents" default the task count would be non-zero.
	if err := s.db.Create(&db.Implant{ID: "wf-live", Hostname: "live", OS: "Windows"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	c, w := wfJSONCtx("")
	c.Params = gin.Params{{Key: "id", Value: "wf-scope"}}
	s.handleAPIWorkflowsExecute(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		TaskCount   int `json:"task_count"`
		AgentsCount int `json:"agents_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.TaskCount != 0 || payload.AgentsCount != 0 {
		t.Fatalf("empty scope executed on %d agent(s) / %d task(s), want 0/0",
			payload.AgentsCount, payload.TaskCount)
	}
}

// TestWorkflowExecuteMissingIs404 proves the execute path refuses a missing
// workflow.
func TestWorkflowExecuteMissingIs404(t *testing.T) {
	s := newWorkflowTestServer(t)

	c, w := wfJSONCtx("")
	c.Params = gin.Params{{Key: "id", Value: "ghost"}}
	s.handleAPIWorkflowsExecute(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", w.Code)
	}
}

// TestWorkflowDeleteAndExecutions404 proves delete removes the row and the
// execution endpoints answer 404 for unknown ids.
func TestWorkflowDeleteAndExecutions404(t *testing.T) {
	s := newWorkflowTestServer(t)
	if err := s.db.Create(&db.Workflow{ID: "wf-del", Name: "doomed", ScopeType: "all"}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	c, w := wfJSONCtx("")
	c.Params = gin.Params{{Key: "id", Value: "wf-del"}}
	s.handleAPIDeleteWorkflow(c)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status=%d, want 200", w.Code)
	}
	var n int64
	if err := s.db.Model(&db.Workflow{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("workflow survived delete: %d remain", n)
	}

	c2, w2 := wfJSONCtx("")
	c2.Params = gin.Params{{Key: "id", Value: "wf-del"}}
	s.handleListWorkflowExecutions(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("executions list status=%d, want 200 (empty list is valid here)", w2.Code)
	}

	c3, w3 := wfJSONCtx("")
	c3.Params = gin.Params{{Key: "executionId", Value: "exec-ghost"}}
	s.handleGetWorkflowExecution(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("execution detail status=%d, want 404", w3.Code)
	}
}

// Deleting a group that still has agents assigned is refused (the agents
// would silently lose their grouping), and so is one that still has children.
func TestGroupDeleteRefusesNonEmpty(t *testing.T) {
	s := newWorkflowTestServer(t)
	if err := s.db.Create(&db.Implant{ID: "grp-guard", Hostname: "grp-guard", OS: "Linux"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := s.db.Create(&db.AgentGroup{ID: "grp-busy", Name: "busy"}).Error; err != nil {
		t.Fatalf("seed group: %v", err)
	}
	if err := s.db.Create(&db.AgentGroupAssignment{AgentGroupID: "grp-busy", ImplantID: "grp-guard"}).Error; err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	c, w := wfJSONCtx("")
	c.Params = gin.Params{{Key: "id", Value: "grp-busy"}}
	s.handleAPIDeleteGroup(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400 for a group with agents", w.Code, w.Body.String())
	}
	var n int64
	if err := s.db.Model(&db.AgentGroup{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("refused delete still removed the group: %d remain", n)
	}

	// A group with only a child is also protected.
	parentID := "grp-parent"
	if err := s.db.Create(&db.AgentGroup{ID: parentID, Name: "parent"}).Error; err != nil {
		t.Fatalf("seed parent: %v", err)
	}
	if err := s.db.Create(&db.AgentGroup{ID: "grp-kid", Name: "kid", ParentID: &parentID}).Error; err != nil {
		t.Fatalf("seed kid: %v", err)
	}
	c2, w2 := wfJSONCtx("")
	c2.Params = gin.Params{{Key: "id", Value: "grp-parent"}}
	s.handleAPIDeleteGroup(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400 for a group with children", w2.Code)
	}
}

// TestGroupCRUDAndAggregation proves group create (default color), the agent
// and child counts on list, and that an emptied group deletes cleanly.
func TestGroupCRUDAndAggregation(t *testing.T) {
	s := newWorkflowTestServer(t)

	c, w := wfJSONCtx(`{"name":"parent"}`)
	s.handleAPICreateGroup(c)
	if w.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.ID == "" {
		t.Fatal("create returned no group id")
	}
	var parent db.AgentGroup
	if err := s.db.First(&parent, payload.ID).Error; err != nil {
		t.Fatalf("group not persisted: %v", err)
	}
	if parent.Color != "#2ecc71" {
		t.Fatalf("color = %q, want the default #2ecc71", parent.Color)
	}

	// A child group plus one assigned agent populate the aggregate counts.
	if err := s.db.Create(&db.AgentGroup{ID: "grp-child", Name: "child", ParentID: &payload.ID}).Error; err != nil {
		t.Fatalf("seed child: %v", err)
	}
	if err := s.db.Create(&db.Implant{ID: "grp-agent", Hostname: "grp-agent", OS: "Linux"}).Error; err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	if err := s.db.Create(&db.AgentGroupAssignment{AgentGroupID: payload.ID, ImplantID: "grp-agent"}).Error; err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	c2, w2 := wfJSONCtx("")
	s.handleAPIGroups(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", w2.Code, w2.Body.String())
	}
	var listed struct {
		Groups []struct {
			Name       string `json:"name"`
			AgentCount int    `json:"agent_count"`
			ChildCount int    `json:"child_count"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	var found bool
	for _, g := range listed.Groups {
		if g.Name == "parent" {
			found = true
			if g.AgentCount != 1 {
				t.Fatalf("agent_count = %d, want 1", g.AgentCount)
			}
			if g.ChildCount != 1 {
				t.Fatalf("child_count = %d, want 1", g.ChildCount)
			}
		}
	}
	if !found {
		t.Fatal("created group missing from the list")
	}

	// The delete guard is proven by TestGroupDeleteRefusesNonEmpty; here the
	// group is emptied first (agent assignment + child removed) so the happy
	// path is exercised — both rows are gone, leaving zero groups.
	if err := s.db.Where("agent_group_id = ?", payload.ID).Delete(&db.AgentGroupAssignment{}).Error; err != nil {
		t.Fatalf("clear assignment: %v", err)
	}
	if err := s.db.Where("parent_id = ?", payload.ID).Delete(&db.AgentGroup{}).Error; err != nil {
		t.Fatalf("clear child: %v", err)
	}

	c3, w3 := wfJSONCtx("")
	c3.Params = gin.Params{{Key: "id", Value: payload.ID}}
	s.handleAPIDeleteGroup(c3)
	if w3.Code != http.StatusOK {
		t.Fatalf("delete status=%d, want 200", w3.Code)
	}
	var n int64
	if err := s.db.Model(&db.AgentGroup{}).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("groups remaining = %d, want 0 after the cleanup and delete", n)
	}
}

// TestGroupCreateRejectsMissingName and update 404 guard.
func TestGroupUpdateMissingIs404(t *testing.T) {
	s := newWorkflowTestServer(t)

	c, w := wfJSONCtx(`{"name":""}`)
	s.handleAPICreateGroup(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("create without name: status=%d, want 400", w.Code)
	}

	c2, w2 := wfJSONCtx(`{"name":"x"}`)
	c2.Params = gin.Params{{Key: "id", Value: "ghost"}}
	s.handleAPIUpdateGroup(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("update missing group: status=%d, want 404", w2.Code)
	}
}
