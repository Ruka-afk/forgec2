package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/forgec2/forgec2/pkg/protocol"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func newCampaignSeedTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	return &Server{
		db:                testutil.SetupTestDB(t),
		cfg:               cfg,
		wsClients:         make(map[*websocket.Conn]*wsClientConn),
		agentPendingTasks: make(map[string]int),
	}
}

func campaignSeedRequest(t *testing.T, s *Server, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/campaigns/"+id+"/killchain", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "alice")
	s.handleCampaignKillChain(c)
	return w
}

func seedCampaignWithAgents(t *testing.T, s *Server, campaignID string, agentIDs ...string) {
	t.Helper()
	campaign := &db.Campaign{ID: campaignID, Name: "test-campaign"}
	if err := s.db.Create(campaign).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	var implants []db.Implant
	for _, id := range agentIDs {
		im := db.Implant{ID: id, Hostname: "host-" + id, OS: "Windows"}
		if err := s.db.Create(&im).Error; err != nil {
			t.Fatalf("seed agent: %v", err)
		}
		implants = append(implants, im)
	}
	// Link through the many2many association, which is how the campaign
	// handlers maintain campaign_agents. The explicit join struct was removed;
	// this asserts GORM still creates and populates the table without one.
	if len(implants) > 0 {
		if err := s.db.Model(campaign).Association("Agents").Append(implants); err != nil {
			t.Fatalf("link campaign agents: %v", err)
		}
	}
}

// TestCampaignKillChainSeedsTasks verifies the endpoint no longer fakes success:
// it must materialize a pending task per template step per campaign agent.
func TestCampaignKillChainSeedsTasks(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1", "a2")

	w := campaignSeedRequest(t, s, "c1", `{"template":"Standard Intrusion"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		TasksCreated int `json:"tasks_created"`
		Agents       int `json:"agents"`
	}
	unmarshalData(t, w.Body.Bytes(), &resp)
	if resp.TasksCreated != 14 || resp.Agents != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}

	var tasks []db.Task
	if err := s.db.Where("agent_id IN ?", []string{"a1", "a2"}).Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 14 {
		t.Fatalf("got %d tasks, want 14", len(tasks))
	}
	// Every seeded task must be pending and attributable to the operator.
	for _, tk := range tasks {
		if tk.Status != "pending" {
			t.Errorf("task %d (agent %s) status = %q, want pending", tk.ID, tk.AgentID, tk.Status)
		}
		if tk.CreatedBy != "alice" {
			t.Errorf("task %d created_by = %q, want alice", tk.ID, tk.CreatedBy)
		}
	}
	// The shell steps must carry their template command, not silence.
	shellCount := 0
	for _, tk := range tasks {
		if tk.Type == "shell" {
			shellCount++
			if tk.Command == "" {
				t.Errorf("shell task %d has empty command", tk.ID)
			}
		}
	}
	if shellCount != 6 {
		t.Errorf("shell tasks = %d, want 6 (3 per agent)", shellCount)
	}
}

// TestCampaignKillChainRejectsBadInput pins the honest failure paths: the
// endpoint must not report success when it cannot create any tasks.
func TestCampaignKillChainRejectsBadInput(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1")
	seedCampaignWithAgents(t, s, "empty")

	cases := []struct {
		name string
		id   string
		body string
	}{
		{"no body", "c1", ""},
		{"malformed body", "c1", `{"nope":1}`},
		{"unknown template", "c1", `{"template":"Not Real"}`},
		{"campaign without agents", "empty", `{"template":"Standard Intrusion"}`},
		{"missing campaign", "nope", `{"template":"Standard Intrusion"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := campaignSeedRequest(t, s, tc.id, tc.body)
			if w.Code == http.StatusOK {
				t.Fatalf("expected non-OK status for %q, got 200: %s", tc.name, w.Body.String())
			}
		})
	}

	var count int64
	s.db.Model(&db.Task{}).Count(&count)
	if count != 0 {
		t.Fatalf("no tasks should be created on failed seeds, got %d", count)
	}
}

// TestCampaignKillChainHonoursPendingCeiling ensures seeding cannot bypass the
// per-agent pending-task limit that blocks normal task creation.
func TestCampaignKillChainHonoursPendingCeiling(t *testing.T) {
	s := newCampaignSeedTestServer(t)
	seedCampaignWithAgents(t, s, "c1", "a1")
	s.agentPendingTasksMu.Lock()
	s.agentPendingTasks["a1"] = MaxPendingTasksPerAgent
	s.agentPendingTasksMu.Unlock()

	w := campaignSeedRequest(t, s, "c1", `{"template":"Standard Intrusion"}`)
	if w.Code == http.StatusOK {
		t.Fatalf("expected non-OK status when agent is at ceiling: %s", w.Body.String())
	}
	var count int64
	s.db.Model(&db.Task{}).Count(&count)
	if count != 0 {
		t.Fatalf("no tasks should be created when all agents are at ceiling, got %d", count)
	}
}

// ── Tenant isolation ────────────────────────────────────────────────────────

// campaignCtx builds a tenant-scoped admin context for the campaign API.
func campaignCtx(s *Server, t *testing.T, username string, tenantID uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	return tenantScopedAdminContext(s, t, username, tenantID)
}

// campaignCtxWithParams additionally pins the :id path parameter.
func campaignCtxWithParams(s *Server, t *testing.T, username string, tenantID uint, id string, method, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, w := campaignCtx(s, t, username, tenantID)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Request, _ = http.NewRequest(method, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func seedCampaign(t *testing.T, s *Server, id, name string, tenantID uint) db.Campaign {
	t.Helper()
	camp := db.Campaign{ID: id, Name: name, Status: "active", TenantID: tenantID}
	if err := s.db.Create(&camp).Error; err != nil {
		t.Fatalf("seed campaign: %v", err)
	}
	return camp
}

func attachCampaignAgent(t *testing.T, s *Server, campaignID, agentID string) {
	t.Helper()
	var camp db.Campaign
	if err := s.db.First(&camp, "id = ?", campaignID).Error; err != nil {
		t.Fatalf("load campaign: %v", err)
	}
	var agent db.Implant
	if err := s.db.First(&agent, "id = ?", agentID).Error; err != nil {
		t.Fatalf("load agent: %v", err)
	}
	if err := s.db.Model(&camp).Association("Agents").Append(&agent); err != nil {
		t.Fatalf("attach agent: %v", err)
	}
}

// TestCampaignListIsTenantScoped proves the campaign list returns only the
// caller's own campaigns. Campaign rows used to carry no tenant at all, so
// every tenant saw every campaign; legacy (tenant 0) rows now follow the same
// rule as agents and macros — visible to unscoped operators only.
func TestCampaignListIsTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-mine", "mine", 1)
	seedCampaign(t, s, "c-theirs", "theirs", 2)
	seedCampaign(t, s, "c-legacy", "legacy", 0)

	c, w := campaignCtx(s, t, "camp-list", 1)
	s.handleCampaignsList(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Data []db.Campaign `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	seen := map[string]bool{}
	for _, camp := range body.Data {
		seen[camp.ID] = true
	}
	if _, leaked := seen["c-theirs"]; leaked {
		t.Fatalf("another tenant's campaign leaked into the list: %v", seen)
	}
	if _, leaked := seen["c-legacy"]; leaked {
		t.Fatalf("legacy (tenant 0) campaign visible to a scoped operator: %v", seen)
	}
	if !seen["c-mine"] {
		t.Fatalf("own campaign missing from the list: %v", seen)
	}

	// An unscoped (tenant 0) operator keeps the global view.
	c2, w2 := campaignCtx(s, t, "camp-global", 0)
	s.handleCampaignsList(c2)
	var global struct {
		Data []db.Campaign `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &global); err != nil {
		t.Fatalf("decode: %v (%s)", err, w2.Body.String())
	}
	if len(global.Data) != 3 {
		t.Fatalf("unscoped operator sees %d campaigns, want all 3", len(global.Data))
	}
}

// TestCampaignGetIsTenantScoped proves read, update and delete all refuse a
// foreign campaign and leave it untouched.
func TestCampaignGetIsTenantScoped(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-theirs", "theirs", 2)

	c, w := campaignCtxWithParams(s, t, "camp-get", 1, "c-theirs", http.MethodGet, "")
	s.handleCampaignGet(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get: status = %d, want 404", w.Code)
	}

	c2, w2 := campaignCtxWithParams(s, t, "camp-get", 1, "c-theirs", http.MethodPost, `{"status":"archived"}`)
	s.handleCampaignUpdate(c2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("update: status = %d, want 404", w2.Code)
	}

	c3, w3 := campaignCtxWithParams(s, t, "camp-get", 1, "c-theirs", http.MethodDelete, "")
	s.handleCampaignDelete(c3)
	if w3.Code != http.StatusNotFound {
		t.Fatalf("delete: status = %d, want 404", w3.Code)
	}

	var survivor db.Campaign
	if err := s.db.First(&survivor, "id = ?", "c-theirs").Error; err != nil {
		t.Fatalf("campaign was destroyed by a refused request: %v", err)
	}
	if survivor.Status != "active" {
		t.Fatalf("status = %q, want the refused update to be a no-op", survivor.Status)
	}
}

// TestCampaignCreateStampsTenant proves a new campaign is owned by the creator
// rather than being globally visible.
func TestCampaignCreateStampsTenant(t *testing.T) {
	s := mustTenantServer(t)

	c, w := campaignCtxWithParams(s, t, "camp-create", 7, "", http.MethodPost, `{"name":"op seven","description":"d"}`)
	s.handleCampaignCreate(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var body struct {
		ID       string      `json:"id"`
		Campaign db.Campaign `json:"campaign"`
	}
	unmarshalData(t, w.Body.Bytes(), &body)
	if body.ID == "" {
		t.Fatal("create returned no id")
	}
	var stored db.Campaign
	if err := s.db.First(&stored, "id = ?", body.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.TenantID != 7 {
		t.Fatalf("TenantID = %d, want the creator's tenant 7", stored.TenantID)
	}
	if stored.Name != "op seven" || stored.Status != "active" {
		t.Fatalf("stored campaign = %+v", stored)
	}

	// A missing name is still refused.
	c2, w2 := campaignCtxWithParams(s, t, "camp-create", 7, "", http.MethodPost, `{"description":"no name"}`)
	s.handleCampaignCreate(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("missing name: status = %d, want 400", w2.Code)
	}
}

// ── Kill-chain tenant stamping ──────────────────────────────────────────────

// TestCampaignKillChainStampsTenantAndCreator is the core regression: seeded
// tasks used to be inserted with TenantID 0, so they were readable by every
// tenant and looked like legacy rows to scoped operators.
func TestCampaignKillChainStampsTenantAndCreator(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-1", "op seven", 7)
	seedTenantAgent(t, s, "kc-agent", 7)
	attachCampaignAgent(t, s, "c-1", "kc-agent")

	c, w := campaignCtxWithParams(s, t, "kc-op", 7, "c-1", http.MethodPost, `{"template":"Stealth Operations"}`)
	s.handleCampaignKillChain(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}

	var body struct {
		TasksCreated  int    `json:"tasks_created"`
		Agents        int    `json:"agents"`
		AgentsSkipped int    `json:"agents_skipped"`
		Template      string `json:"template"`
	}
	unmarshalData(t, w.Body.Bytes(), &body)
	if body.Template != "Stealth Operations" || body.Agents != 1 {
		t.Fatalf("response = %+v", body)
	}

	var tasks []db.Task
	if err := s.db.Where("agent_id = ?", "kc-agent").Order("id").Find(&tasks).Error; err != nil {
		t.Fatalf("find tasks: %v", err)
	}
	if len(tasks) != body.TasksCreated || len(tasks) != 4 {
		t.Fatalf("stored %d tasks, want the 4 the template declares", len(tasks))
	}
	for i, task := range tasks {
		if task.TenantID != 7 {
			t.Fatalf("task %d TenantID = %d, want the campaign's tenant 7", i, task.TenantID)
		}
		if task.CreatedBy != "kc-op" {
			t.Fatalf("task %d CreatedBy = %q, want the seeding operator", i, task.CreatedBy)
		}
		if task.Status == "" {
			t.Fatalf("task %d has no initial status", i)
		}
	}
	// The in-memory pending counters must match what was actually queued.
	if got := s.agentPendingTasks["kc-agent"]; got != len(tasks) {
		t.Fatalf("pending counter = %d, want %d", got, len(tasks))
	}
}

// TestCampaignKillChainCrossTenant404 proves a foreign campaign cannot be used
// to seed tasks — the endpoint that mints work on real hosts.
func TestCampaignKillChainCrossTenant404(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-x", "theirs", 2)
	seedTenantAgent(t, s, "kc-x", 2)
	attachCampaignAgent(t, s, "c-x", "kc-x")

	c, w := campaignCtxWithParams(s, t, "kc-attacker", 1, "c-x", http.MethodPost, `{"template":"Stealth Operations"}`)
	s.handleCampaignKillChain(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for another tenant's campaign", w.Code)
	}
	var tasks int64
	if err := s.db.Model(&db.Task{}).Where("agent_id = ?", "kc-x").Count(&tasks).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if tasks != 0 {
		t.Fatalf("cross-tenant kill chain seeded %d tasks into another tenant's agent", tasks)
	}
	if got := s.agentPendingTasks["kc-x"]; got != 0 {
		t.Fatalf("cross-tenant seeding moved the pending counter to %d", got)
	}
}

// TestCampaignKillChainAuditsSeeding proves the bulk insert leaves an audit
// trail naming the operator — it previously only wrote a server log line.
func TestCampaignKillChainAuditsSeeding(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-audit", "audited", 4)
	seedTenantAgent(t, s, "kc-audit", 4)
	attachCampaignAgent(t, s, "c-audit", "kc-audit")

	c, w := campaignCtxWithParams(s, t, "kc-auditor", 4, "c-audit", http.MethodPost, `{"template":"Stealth Operations"}`)
	s.handleCampaignKillChain(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var entry db.AuditLog
	if err := s.db.Where("action = ?", "campaign_killchain_seed").First(&entry).Error; err != nil {
		t.Fatalf("no seeding audit entry: %v", err)
	}
	if entry.User != "kc-auditor" || entry.Resource != "campaign" || entry.AgentID != "c-audit" || !entry.Success {
		t.Fatalf("audit entry = %+v", entry)
	}
}

// TestCampaignKillChainNoAgentsRefused pins the "campaign has no agents"
// pre-condition with the tenant-scoped handler (the unscoped helper above covers
// the other refusal shapes).
func TestCampaignKillChainNoAgentsRefused(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-empty", "empty", 1)

	c, w := campaignCtxWithParams(s, t, "kc-empty", 1, "c-empty", http.MethodPost, `{"template":"Stealth Operations"}`)
	s.handleCampaignKillChain(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "no agents") {
		t.Fatalf("body = %s, want the no-agents message", w.Body.String())
	}
}

// ── MITRE timeline scoping ──────────────────────────────────────────────────

// TestMitreTimelineIgnoresForeignCampaign proves a foreign campaign_id does not
// widen the timeline: the caller sees their own tasks, never the other
// tenant's phase breakdown.
func TestMitreTimelineIgnoresForeignCampaign(t *testing.T) {
	s := mustTenantServer(t)
	seedCampaign(t, s, "c-t2", "theirs", 2)
	seedTenantAgent(t, s, "tl-x", 2)
	attachCampaignAgent(t, s, "c-t2", "tl-x")
	if err := s.db.Create(&db.Task{
		TenantID: 2, AgentID: "tl-x", Type: "ps", Command: "x", Status: "completed",
	}).Error; err != nil {
		t.Fatalf("seed foreign task: %v", err)
	}

	c, w := campaignCtx(s, t, "tl-op", 1)
	c.Request, _ = http.NewRequest(http.MethodGet, "/mitre/timeline?campaign_id=c-t2", nil)
	s.handleMitreTimeline(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "tl-x") {
		t.Fatalf("timeline leaked another tenant's agent: %s", w.Body.String())
	}
}

// ── Pure functions ──────────────────────────────────────────────────────────

// TestSplitStepParams pins the template param → task column mapping and proves
// unknown keys are ignored instead of landing in a command field.
func TestSplitStepParams(t *testing.T) {
	cases := []struct {
		name        string
		params      map[string]string
		cmd, p, dat string
	}{
		{"nil params", nil, "", "", ""},
		{"command only", map[string]string{"command": "whoami"}, "whoami", "", ""},
		{"path only", map[string]string{"path": "C:\\tmp"}, "", "C:\\tmp", ""},
		{"data only", map[string]string{"data": "payload"}, "", "", "payload"},
		{
			name:   "all three",
			params: map[string]string{"command": "c", "path": "p", "data": "d"},
			cmd:    "c", p: "p", dat: "d",
		},
		{"unknown keys ignored", map[string]string{"wait": "5", "phase": "Recon", "command": "keep"}, "keep", "", ""},
		{"empty values kept as empty", map[string]string{"command": "", "data": "d"}, "", "", "d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd, p, dat := splitStepParams(tc.params)
			if cmd != tc.cmd || p != tc.p || dat != tc.dat {
				t.Fatalf("splitStepParams = (%q,%q,%q), want (%q,%q,%q)", cmd, p, dat, tc.cmd, tc.p, tc.dat)
			}
		})
	}
}

// TestFindKillChainTemplate pins exact-name lookup and the unknown miss.
func TestFindKillChainTemplate(t *testing.T) {
	if tpl := findKillChainTemplate("Stealth Operations"); tpl == nil {
		t.Fatal("built-in template not found by name")
	}
	if tpl := findKillChainTemplate("stealth operations"); tpl != nil {
		t.Fatal("lookup is exact-match; a mismatched name must not seed the wrong chain")
	}
	if tpl := findKillChainTemplate("does-not-exist"); tpl != nil {
		t.Fatal("unknown template resolved to a value")
	}
	if tpl := findKillChainTemplate(""); tpl != nil {
		t.Fatal("empty template name resolved to a value")
	}
}

// TestBuiltinKillChainTemplatesAreExecutable proves every built-in template
// step names a task type the protocol actually registers: seeding a ghost type
// would fabricate queue entries that can never run.
func TestBuiltinKillChainTemplatesAreExecutable(t *testing.T) {
	templates := builtinKillChainTemplates()
	if len(templates) == 0 {
		t.Fatal("no built-in kill chain templates")
	}
	for _, tpl := range templates {
		if tpl.Name == "" || len(tpl.Steps) == 0 {
			t.Fatalf("template %q has no steps", tpl.Name)
		}
		for _, st := range tpl.Steps {
			if st.TaskType == "" {
				t.Fatalf("template %q has a step with no task type", tpl.Name)
			}
			if !IsKnownTaskType(st.TaskType) && !protocol.ValidTaskType(st.TaskType) {
				t.Fatalf("template %q references unknown task type %q", tpl.Name, st.TaskType)
			}
		}
	}
}

// TestTaskPhase pins the MITRE phase mapping the timeline and stats derive
// from, including the default: an unmapped task type is reported as
// "Execution" (the catch-all bucket the campaign chart renders), not dropped.
// Credential-theft types are pinned explicitly because they used to fall
// through to that catch-all and were shown under the wrong phase.
func TestTaskPhase(t *testing.T) {
	cases := []struct{ taskType, want string }{
		{"ps", "Discovery"},
		{"shell", "Execution"},
		{"keylogger_start", "Collection"},
		{"keylogger_dump", "Collection"},
		{"clipboard_get", "Collection"},
		{"screenshot", "Collection"},
		{"screen_stream_start", "Collection"},
		{"SCREEN_STREAM_STOP", "Collection"},
		{"REG_GET", "Persistence"},
		{"recon", "Reconnaissance"},
		{"portscan", "Reconnaissance"},
		{"password_spray", "Credential Access"},
		{"token_steal", "Credential Access"},
		{"token_make", "Credential Access"},
		{"cred_check", "Credential Access"},
		{"dcsync", "Credential Access"},
		{"hashdump", "Credential Access"},
		{"lsa_dump", "Credential Access"},
		{"lateral", "Lateral Movement"},
		{"lateral_list", "Lateral Movement"},
		{"beacon_now", "Command and Control"},
		{"profile_rotate", "Command and Control"},
		{"upload", "Exfiltration"},
		{"killproc", "Defense Evasion"},
		{"reboot", "Execution"},
		{"definitely_not_a_task", "Execution"},
		{"", "Execution"},
	}
	for _, tc := range cases {
		if got := taskPhase(tc.taskType); got != tc.want {
			t.Fatalf("taskPhase(%q) = %q, want %q", tc.taskType, got, tc.want)
		}
	}
}

// TestBuildPhaseTimeline proves the timeline reports each phase once with its
// earliest sighting and full count, and that it is stable for empty input.
func TestBuildPhaseTimeline(t *testing.T) {
	if got := buildPhaseTimeline(nil); len(got) != 0 {
		t.Fatalf("empty input produced %d phases", len(got))
	}
	base := time.Now().Add(-time.Hour)
	tasks := []db.Task{
		{Type: "ps", CreatedAt: base.Add(10 * time.Minute)},
		{Type: "ps", CreatedAt: base.Add(20 * time.Minute)},
		{Type: "shell", CreatedAt: base.Add(30 * time.Minute)},
		{Type: "ps", CreatedAt: base}, // earliest Discovery sighting
	}
	got := buildPhaseTimeline(tasks)
	if len(got) != 2 {
		t.Fatalf("got %d phases, want 2: %+v", len(got), got)
	}
	byPhase := map[string]map[string]interface{}{}
	for _, p := range got {
		byPhase[p["phase"].(string)] = p
	}
	disc, ok := byPhase["Discovery"]
	if !ok {
		t.Fatalf("Discovery phase missing: %+v", byPhase)
	}
	if disc["task_count"] != 3 {
		t.Fatalf("Discovery task_count = %v, want 3", disc["task_count"])
	}
	if disc["first_seen"] != base.Format(time.RFC3339) {
		t.Fatalf("Discovery first_seen = %v, want the earliest sighting", disc["first_seen"])
	}
	if _, ok := byPhase["Execution"]; !ok {
		t.Fatalf("Execution phase missing: %+v", byPhase)
	}
}

// TestComputeCampaignStats proves the aggregate counts the campaign detail
// endpoint returns, including that agents with no tasks still appear with a
// zero count rather than vanishing from the breakdown.
func TestComputeCampaignStats(t *testing.T) {
	agents := []db.Implant{
		{ID: "a1", Hostname: "H1", Username: "corp\\u1", IP: "10.0.0.1"},
		{ID: "a2", Hostname: "H2", Username: "corp\\u2", IP: "10.0.0.2"},
	}
	base := time.Now().Add(-time.Hour)
	tasks := []db.Task{
		{AgentID: "a1", Type: "ps", Status: "completed", CreatedAt: base},
		{AgentID: "a1", Type: "shell", Status: "failed", CreatedAt: base.Add(time.Minute)},
		{AgentID: "a1", Type: "shell", Status: "pending", CreatedAt: base.Add(2 * time.Minute)},
	}
	stats := computeCampaignStats(agents, tasks)

	if stats["total_agents"] != 2 || stats["total_tasks"] != 3 {
		t.Fatalf("counts = %v/%v, want 2/3", stats["total_agents"], stats["total_tasks"])
	}
	if stats["completed_tasks"] != 1 || stats["failed_tasks"] != 1 {
		t.Fatalf("completed/failed = %v/%v, want 1/1", stats["completed_tasks"], stats["failed_tasks"])
	}
	summary, ok := stats["kill_chain_summary"].(map[string]int)
	if !ok {
		t.Fatalf("kill_chain_summary = %T", stats["kill_chain_summary"])
	}
	if summary["Discovery"] != 1 || summary["Execution"] != 2 {
		t.Fatalf("phase summary = %v", summary)
	}
	breakdown, ok := stats["agent_breakdown"].([]map[string]interface{})
	if !ok || len(breakdown) != 2 {
		t.Fatalf("agent_breakdown = %v", stats["agent_breakdown"])
	}
	for _, entry := range breakdown {
		id := entry["agent_id"].(string)
		want := 3
		if id == "a2" {
			want = 0
		}
		if entry["task_count"] != want {
			t.Fatalf("agent %s task_count = %v, want %d", id, entry["task_count"], want)
		}
	}
}
