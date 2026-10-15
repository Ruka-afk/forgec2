package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/gin-gonic/gin"
)

// ciphertextMarker is the at-rest prefix EncryptLoot writes. No handler
// response may ever contain it.
const ciphertextMarker = "FC2ENC:"

// seedEncryptedTaskResult writes a task through the production result-encryption
// path (EncryptTaskFields, exactly what the beacon and ExtC2 writers call) and
// asserts the precondition that the raw column really is ciphertext. Without
// that assertion a broken hook would make every read-side check below pass
// vacuously.
func seedEncryptedTaskResult(t *testing.T, s *Server, agentID string, tenantID uint, taskType, command, result string) *db.Task {
	t.Helper()
	task := &db.Task{TenantID: tenantID, AgentID: agentID, Type: taskType, Command: command, Result: result, Status: "completed"}
	if err := task.EncryptTaskFields(); err != nil {
		t.Fatalf("encrypt %s result: %v", taskType, err)
	}
	if err := s.db.Create(task).Error; err != nil {
		t.Fatalf("seed %s task: %v", taskType, err)
	}
	var raw struct{ Result string }
	if err := s.db.Table("tasks").Select("result").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Result, ciphertextMarker) {
		t.Fatalf("precondition: %s result must be ciphertext at rest, got %q", taskType, raw.Result)
	}
	if strings.Contains(raw.Result, result) {
		t.Fatalf("precondition: %s result leaked plaintext into the raw column", taskType)
	}
	return task
}

// TestProcessTreeReturnsDecryptedOutput proves the process-tree panel works
// when results are encrypted at rest. Scanning into a bare struct skipped
// Task.AfterFind, the envelope parse then failed, and the handler answered
// "No completed process tree" for every agent that actually had one.
func TestProcessTreeReturnsDecryptedOutput(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-proc", 1)
	seedEncryptedTaskResult(t, s, "g1-proc", 1, "process_tree", "process_tree",
		"PID  COMMAND\n  4  explorer.exe\n  88 svchost.exe\n")

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	c.Params = gin.Params{{Key: "id", Value: "g1-proc"}}
	s.handleGetProcessTree(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200 — an encrypted result must not read as a missing tree", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "explorer.exe") {
		t.Fatalf("response lost the decrypted process output: %s", body)
	}
	if strings.Contains(body, ciphertextMarker) {
		t.Fatalf("response leaked ciphertext: %s", body)
	}
}

// TestProcessTreePrefersTreeOverLastPsListing keeps the ordering contract
// honest now that the row is read through the model: process_tree outranks ps,
// and a ps-only agent still gets its snapshot.
func TestProcessTreePrefersTreeOverLastPsListing(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-proc2", 1)
	seedEncryptedTaskResult(t, s, "g1-proc2", 1, "ps", "ps", "PID COMMAND\n 7 winword.exe")

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	c.Params = gin.Params{{Key: "id", Value: "g1-proc2"}}
	s.handleGetProcessTree(c)
	if w.Code != http.StatusOK {
		t.Fatalf("ps fallback status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "winword.exe") || strings.Contains(w.Body.String(), ciphertextMarker) {
		t.Fatalf("ps fallback not decrypted: %s", w.Body.String())
	}

	seedEncryptedTaskResult(t, s, "g1-proc2", 1, "process_tree", "process_tree", "PID COMMAND\n 9 lsass.exe")
	c2, w2 := tenantScopedAdminContext(s, t, "g1-admin", 1)
	c2.Params = gin.Params{{Key: "id", Value: "g1-proc2"}}
	s.handleGetProcessTree(c2)
	if w2.Code != http.StatusOK || !strings.Contains(w2.Body.String(), "lsass.exe") {
		t.Fatalf("process_tree must outrank ps: status=%d body=%s", w2.Code, w2.Body.String())
	}
}

// TestProcessTreeStill404sWhenNoCompletedTree proves the 404 path is reserved
// for genuinely missing data, not used as a ciphertext side effect.
func TestProcessTreeStill404sWhenNoCompletedTree(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-proc3", 1)

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	c.Params = gin.Params{{Key: "id", Value: "g1-proc3"}}
	s.handleGetProcessTree(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404 for an agent with no completed tree", w.Code)
	}
}

// TestTimelineDetailsAreDecrypted proves the unified timeline feed shows command
// output, not FC2ENC: blobs. The task query scans the model so AfterFind runs.
func TestTimelineDetailsAreDecrypted(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-tl", 1)
	const out = "sekizi Access granted for user CORP\\jsmith"
	seedEncryptedTaskResult(t, s, "g1-tl", 1, "whoami", "whoami", out)

	c, _ := tenantScopedAdminContext(s, t, "g1-admin", 1)
	events := s.buildTimelineEvents(c, "task", "", "", "", "")

	var found bool
	for _, e := range events {
		if e.Type != "task" {
			continue
		}
		found = true
		if strings.Contains(e.Details, ciphertextMarker) {
			t.Fatalf("timeline Details carried ciphertext: %q", e.Details)
		}
		if e.Details != out {
			t.Fatalf("timeline Details = %q, want %q", e.Details, out)
		}
		if e.Timestamp.IsZero() {
			t.Fatal("timeline event lost its timestamp (created_at must map to the event)")
		}
	}
	if !found {
		t.Fatal("the task never reached the timeline feed")
	}
}

// TestPrivescResultsAreDecrypted proves /api/privesc/results serves plaintext.
// The handler used to scan into its own JSON view struct, which bypasses
// AfterFind, and handed the UI raw ciphertext.
func TestPrivescResultsAreDecrypted(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-privesc", 1)
	const out = "SeImpersonatePrivilege: enabled\nCurrentUser: CORP\\svc-backup"
	seedEncryptedTaskResult(t, s, "g1-privesc", 1, "privesc_check", "privesc_check", out)

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	s.handleAPIPrivesc(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	var payload struct {
		Results []struct {
			ID     uint   `json:"id"`
			Result string `json:"result"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results length = %d, want 1", len(payload.Results))
	}
	if strings.Contains(payload.Results[0].Result, ciphertextMarker) {
		t.Fatalf("privesc result served ciphertext: %q", payload.Results[0].Result)
	}
	if payload.Results[0].Result != out {
		t.Fatalf("privesc result = %q, want %q", payload.Results[0].Result, out)
	}
}

// TestExtractIOCsReadsDecryptedTaskResults is the data-loss guard: the indicator
// regexes ran over ciphertext, so every IOC that appeared only in command
// output was silently dropped while the response still reported success and a
// plausible task count.
func TestExtractIOCsReadsDecryptedTaskResults(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-ioc", 1)
	seedEncryptedTaskResult(t, s, "g1-ioc", 1, "download_url", "download_url",
		"connecting to 203.0.113.45:8443\nsaved to C:\\temp\\p.exe")

	iocs, scanned, err := s.extractIOCs(30, false, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	if scanned != 1 {
		t.Fatalf("scanned = %d, want 1", scanned)
	}
	var hit string
	for _, e := range iocs {
		if strings.Contains(e.Value, "203.0.113.45") {
			hit = e.Value
			break
		}
	}
	if hit == "" {
		var got []string
		for _, e := range iocs {
			got = append(got, e.Value)
		}
		t.Fatalf("the IP in the encrypted result never became an IOC; got %v", got)
	}
}

// TestExtractIOCsDecryptsSensitiveTaskCommands covers the Command side: the
// hook only decrypts Command for SensitiveTaskTypes, which is why the IOC query
// must also select `type`.
func TestExtractIOCsDecryptsSensitiveTaskCommands(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-ioc2", 1)

	// cred_check is at-rest sensitive, so Command rides ciphertext.
	task := &db.Task{
		TenantID: 1, AgentID: "g1-ioc2", Type: "cred_check",
		Command: "lateral to 198.51.100.23", Result: "done", Status: "completed",
	}
	if err := task.EncryptTaskFields(); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := s.db.Create(task).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var raw struct{ Command string }
	if err := s.db.Table("tasks").Select("command").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Command, ciphertextMarker) {
		t.Fatalf("precondition: cred_check command must be ciphertext, got %q", raw.Command)
	}

	iocs, _, err := s.extractIOCs(30, false, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	for _, e := range iocs {
		if strings.Contains(e.Value, ciphertextMarker) {
			t.Fatalf("an indicator was built from ciphertext: %q", e.Value)
		}
		if strings.Contains(e.Value, "198.51.100.23") {
			return
		}
	}
	t.Fatal("the host in the encrypted command never became an IOC")
}

// TestHandoverExportTruncatesAfterDecrypting pins the fix for the inverted
// order in the handover bundle: SUBSTR ran on ciphertext, so the 4 KiB cut
// produced an undecryptable blob that AfterFind passed through and the operator
// received garbage where the task output should be.
func TestHandoverExportTruncatesAfterDecrypting(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-handover", 1)

	const head = "HEAD-OF-ENCRYPTED-RESULT"
	body := head + strings.Repeat("A", 8*1024) + "TAIL-OF-ENCRYPTED-RESULT"
	seedEncryptedTaskResult(t, s, "g1-handover", 1, "shell", "type C:\\notes.txt", body)

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	s.handleHandoverExport(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}
	line := handoverTaskLine(t, w.Body.Bytes())
	if strings.Contains(line, ciphertextMarker) {
		t.Fatalf("tasks.jsonl shipped a ciphertext blob: %q", firstN(line, 120))
	}
	if !strings.Contains(line, head) {
		t.Fatalf("tasks.jsonl lost the head of the decrypted result: %q", firstN(line, 120))
	}
	if strings.Contains(line, "TAIL-OF-ENCRYPTED-RESULT") {
		t.Fatal("the result was not truncated to the documented cap")
	}
	var parsed struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &parsed); err != nil {
		t.Fatalf("tasks.jsonl line is not valid JSON: %v", err)
	}
	// truncateStr keeps HandoverResultMaxLen bytes and appends an ellipsis, so
	// the cap applies to the retained payload, not to the final string.
	if len(parsed.Result) > HandoverResultMaxLen+len("...") {
		t.Fatalf("result length = %d, want <= %d plus the ellipsis", len(parsed.Result), HandoverResultMaxLen)
	}
}

// TestHandoverExportKeepsShortResultsIntact is the regression guard in the other
// direction: the chunked rewrite must not drop or mangle results that were
// already under the cap, and must emit every seeded task exactly once.
func TestHandoverExportKeepsShortResultsIntact(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-handover2", 1)

	const want = "short and plaintext"
	for i := 0; i < 5; i++ {
		seedEncryptedTaskResult(t, s, "g1-handover2", 1, "shell", "whoami", want)
	}

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	s.handleHandoverExport(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", w.Code)
	}

	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	var lines int
	for _, f := range zr.File {
		if f.Name != "tasks.jsonl" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open tasks.jsonl: %v", err)
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(rc)
		rc.Close()
		for _, l := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
			if l == "" {
				continue
			}
			lines++
			var parsed struct {
				Result string `json:"result"`
			}
			if err := json.Unmarshal([]byte(l), &parsed); err != nil {
				t.Fatalf("line %d is not valid JSON: %v", lines, err)
			}
			if parsed.Result != want {
				t.Fatalf("line %d result = %q, want %q", lines, parsed.Result, want)
			}
		}
	}
	if lines != 5 {
		t.Fatalf("tasks.jsonl carried %d rows, want 5", lines)
	}
}

// TestBareStructScanBypassesTaskDecryption is the control for every assertion
// above. Same table, same column, same row, two destinations: scanning into a
// bare struct yields ciphertext, scanning into db.Task yields plaintext. GORM
// only runs Task.AfterFind for the model destination, which is why every read
// path over an encrypted column has to use it.
func TestBareStructScanBypassesTaskDecryption(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-control", 1)
	const out = "control payload"
	task := seedEncryptedTaskResult(t, s, "g1-control", 1, "shell", "whoami", out)

	var bare struct{ Result string }
	if err := s.db.Table("tasks").Select("result").Where("id = ?", task.ID).Find(&bare).Error; err != nil {
		t.Fatalf("bare scan: %v", err)
	}
	if !strings.HasPrefix(bare.Result, ciphertextMarker) {
		t.Fatalf("control: a bare struct scan should hand back ciphertext, got %q", bare.Result)
	}

	var model db.Task
	if err := s.db.Table("tasks").Select("result").Where("id = ?", task.ID).Find(&model).Error; err != nil {
		t.Fatalf("model scan: %v", err)
	}
	if model.Result != out {
		t.Fatalf("control: the model scan must decrypt, got %q", firstN(model.Result, 80))
	}
}

// TestAgentTimelineTitlesAreDecrypted covers the Command side of the same
// defect on the per-agent timeline: for a SensitiveTaskTypes entry the command
// rides ciphertext at rest, and the bare-struct scan used it verbatim as the
// event title.
func TestAgentTimelineTitlesAreDecrypted(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-agttl", 1)

	// cred_check is at-rest sensitive: Command must be ciphertext.
	const cmd = "mimikatz sekizi::privileges::debug CORP\\jsmith"
	task := &db.Task{TenantID: 1, AgentID: "g1-agttl", Type: "cred_check", Command: cmd, Result: "ok", Status: "completed"}
	if err := s.db.Create(task).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var raw struct{ Command string }
	if err := s.db.Table("tasks").Select("command").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Command, ciphertextMarker) {
		t.Fatalf("precondition: cred_check command must be ciphertext, got %q", raw.Command)
	}

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	c.Params = gin.Params{{Key: "id", Value: "g1-agttl"}}
	s.handleAgentTimeline(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, ciphertextMarker) {
		t.Fatalf("the agent timeline leaked a ciphertext command: %s", firstN(body, 200))
	}
	if !strings.Contains(body, "sekizi::privileges::debug") {
		t.Fatalf("the agent timeline lost the decrypted command: %s", firstN(body, 200))
	}
}

// TestAPITimelineActionsAreDecrypted is the /api/timeline counterpart: the
// event's Action string embeds Command verbatim.
func TestAPITimelineActionsAreDecrypted(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-apitl", 1)

	// mimikatz is at-rest sensitive; kerberoast deliberately stays searchable
	// in the DB, so it would not exercise the hook.
	const cmd = "mimikatz sekizi::logonpasswords CORP\\jsmith"
	task := &db.Task{TenantID: 1, AgentID: "g1-apitl", Type: "mimikatz", Command: cmd, Result: "done", Status: "completed"}
	if err := s.db.Create(task).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	var raw struct{ Command string }
	if err := s.db.Table("tasks").Select("command").Where("id = ?", task.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw scan: %v", err)
	}
	if !strings.HasPrefix(raw.Command, ciphertextMarker) {
		t.Fatalf("precondition: kerberoast command must be ciphertext, got %q", raw.Command)
	}

	c, w := tenantScopedAdminContext(s, t, "g1-admin", 1)
	s.handleAPITimelineData(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, ciphertextMarker) {
		t.Fatalf("/api/timeline leaked a ciphertext command: %s", firstN(body, 200))
	}
	if !strings.Contains(body, "mimikatz sekizi::logonpasswords") {
		t.Fatalf("/api/timeline lost the decrypted command: %s", firstN(body, 200))
	}
}

// handoverTaskLine pulls the single tasks.jsonl row out of the bundle.
func handoverTaskLine(t *testing.T, bundle []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "tasks.jsonl" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open tasks.jsonl: %v", err)
		}
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(rc)
		rc.Close()
		for _, l := range strings.Split(strings.TrimRight(buf.String(), "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				return l
			}
		}
	}
	t.Fatal("tasks.jsonl is empty")
	return ""
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
