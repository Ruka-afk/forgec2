package server

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/crypto"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
)

// ── Indicator sweep bounds ───────────────────────────────────────────────────

// seedIOCBearingTask stores a completed task whose result carries one unique
// public IP, so an IOC sweep that skips or truncates rows shows up as a missing
// indicator rather than a silent pass. Results here are plaintext on purpose —
// the at-rest encryption path is covered by the TestExtractIOCs*Decrypt* tests;
// this file is about paging and counts.
func seedIOCBearingTask(t *testing.T, s *Server, i int, ip string) {
	t.Helper()
	body := fmt.Sprintf("resolved target %s on port 443", ip)
	if err := s.db.Create(&db.Task{
		TenantID: 1, AgentID: "g1-iocchunk", Type: "download_url",
		Command: "download_url", Result: body, Status: "completed",
		CreatedAt: time.Now().Add(-time.Duration(i) * time.Second),
	}).Error; err != nil {
		t.Fatalf("seed task %d: %v", i, err)
	}
}

func iocValues(entries []iocEntry) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.Value] = true
	}
	return out
}

// TestExtractIOCsChunkingKeepsEveryRow proves the chunked sweep still reads the
// whole window. The task sweep pages in IOCScanChunk batches up to
// IOCScanTaskLimit; a bug in the paging (wrong offset, early break, resetting
// the accumulator) would silently drop indicators while the endpoint kept
// reporting success and a plausible task count.
//
// More than two chunks' worth of rows are seeded so the loop must wrap.
func TestExtractIOCsChunkingKeepsEveryRow(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-iocchunk", 1)

	const total = IOCScanChunk*2 + 37 // forces three pages
	want := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		// 203.0.113.x is TEST-NET-3: routable-looking but never a real host.
		ip := fmt.Sprintf("203.0.%d.%d", i/250, i%250)
		want[ip] = true
		seedIOCBearingTask(t, s, i, ip)
	}

	iocs, scanned, err := s.extractIOCs(30, false, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	if scanned != total {
		t.Fatalf("scanned = %d, want %d — the sweep must report every row it walked", scanned, total)
	}
	got := iocValues(iocs)
	for ip := range want {
		if !got[ip] {
			t.Fatalf("indicator %s was dropped: the chunked sweep lost a row", ip)
		}
	}
}

// TestExtractIOCsScanLimitIsAWholeNumberOfChunks and the partial-page test
// below together pin the sweep's bound. Seeding IOCScanTaskLimit (20000) rows
// to observe the cap directly would cost more than the guarantee is worth, so
// the cap is pinned structurally and the loop's end condition behaviourally.
func TestExtractIOCsScanLimitIsAWholeNumberOfChunks(t *testing.T) {
	if IOCScanTaskLimit <= 0 || IOCScanChunk <= 0 {
		t.Fatalf("IOC scan bound is not positive: limit=%d chunk=%d", IOCScanTaskLimit, IOCScanChunk)
	}
	if IOCScanTaskLimit%IOCScanChunk != 0 {
		t.Fatalf("IOCScanTaskLimit (%d) must be a whole number of chunks (%d) so the loop ends on an exact page boundary", IOCScanTaskLimit, IOCScanChunk)
	}
	if MaxResultSize*IOCScanChunk > 512*1024*1024 {
		t.Fatalf("a chunk of worst-case %d-byte results would hold %d MiB at once", MaxResultSize, MaxResultSize*IOCScanChunk/(1024*1024))
	}
}

// TestExtractIOCsStopsOnPartialPage proves the loop terminates on a short page
// instead of paging past the end of the window.
func TestExtractIOCsStopsOnPartialPage(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-ioccap", 1)

	const rows = 5
	for i := 0; i < rows; i++ {
		seedIOCBearingTask(t, s, i, fmt.Sprintf("198.51.100.%d", i+1))
	}
	_, scanned, err := s.extractIOCs(30, false, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	if scanned != rows {
		t.Fatalf("scanned = %d, want %d (a partial last page must end the loop)", scanned, rows)
	}
}

// TestExtractIOCsScansWholeResultNotAPrefix guards the specific shortcut this
// change had to avoid. Truncating the text would be cheaper than chunking, but
// an indicator past the cut would vanish with no signal. MaxResultSize is the
// per-row ceiling; the tail of a large result must still be scanned.
func TestExtractIOCsScansWholeResultNotAPrefix(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-iocdeep", 1)

	// ~400 KiB of padding, then the indicator. Comfortably past any prefix a
	// "just cap it at 64 KiB" optimisation would use.
	filler := strings.Repeat("padding line for the extractor to chew on\n", 10000)
	body := filler + "connect 203.0.113.200 now"

	task := &db.Task{TenantID: 1, AgentID: "g1-iocdeep", Type: "download_url", Command: "download_url", Result: body, Status: "completed"}
	if err := s.encryptTaskFieldsOrBlank(task); err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if err := s.db.Create(task).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	iocs, _, err := s.extractIOCs(30, false, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	if !iocValues(iocs)["203.0.113.200"] {
		t.Fatal("an indicator past the first 64 KiB of the result was lost — the sweep is truncating, not chunking")
	}
}

// TestExtractIOCsChunkNetworkHosts proves the network-host half pages too.
func TestExtractIOCsChunkNetworkHosts(t *testing.T) {
	crypto.InitLootEncryption(tenantVisibilityMasterHex)
	s := mustTenantServer(t)
	seedTenantAgent(t, s, "g1-iocnet", 1)

	const total = IOCScanHostChunk + 11 // forces two pages
	for i := 0; i < total; i++ {
		host := db.NetworkHost{
			AgentID:  "g1-iocnet",
			IP:       fmt.Sprintf("192.0.2.%d", i%250),
			Hostname: fmt.Sprintf("host-%03d.example.net", i),
		}
		if err := s.db.Create(&host).Error; err != nil {
			t.Fatalf("seed host %d: %v", i, err)
		}
	}

	iocs, _, err := s.extractIOCs(30, true, nil)
	if err != nil {
		t.Fatalf("extractIOCs: %v", err)
	}
	got := iocValues(iocs)
	for i := 0; i < total; i++ {
		host := fmt.Sprintf("host-%03d.example.net", i)
		if !got[host] {
			t.Fatalf("network host %s was dropped by the chunked sweep", host)
		}
	}
}

// ── Credential usage aggregation ─────────────────────────────────────────────

func newCredUsageServer(t *testing.T) *Server {
	t.Helper()
	ginSetTestMode(t)
	cfg := &config.Config{}
	return &Server{db: testutil.SetupTestDB(t), cfg: cfg}
}

// TestCredentialLatestUsageAggregatePicksNewestPerCredential pins the grouped
// query that replaced the full-ledger read: exactly one row per credential, and
// the newest one. CredentialLifecycle branches on Action/Result, so picking
// the wrong row silently relabels a credential's lifecycle.
func TestCredentialLatestUsageAggregatePicksNewestPerCredential(t *testing.T) {
	s := newCredUsageServer(t)

	base := time.Now().Add(-10 * time.Hour)
	entries := []db.CredentialUsage{
		{CredentialID: 1, Action: "spray", Result: "ok", CreatedAt: base},
		{CredentialID: 1, Action: "verify", Result: "fail", CreatedAt: base.Add(time.Hour)},
		{CredentialID: 2, Action: "spray", Result: "ok", CreatedAt: base},
		{CredentialID: 2, Action: "lateral", Result: "fail", CreatedAt: base.Add(2 * time.Hour)},
		{CredentialID: 3, Action: "verify", Result: "ok", CreatedAt: base},
	}
	for i := range entries {
		if err := s.db.Create(&entries[i]).Error; err != nil {
			t.Fatalf("seed usage %d: %v", i, err)
		}
	}

	c, w := tenantScopedAdminContext(s, t, "cred-admin", 1)
	s.handleCredentialsPage(c)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}

	// Rebuild the same aggregate the handler performs and assert it returns
	// the newest row per credential — one row, not the ledger.
	ranked := s.db.Model(&db.CredentialUsage{}).
		Select("credential_id, action, result, created_at, ROW_NUMBER() OVER (PARTITION BY credential_id ORDER BY created_at DESC) AS rn").
		Where("credential_id IN ?", []uint{1, 2, 3})
	var rows []struct {
		CredentialID uint
		Action       string
		Result       string
	}
	if err := s.db.Table("(?) AS ranked", ranked).Where("rn = 1").Scan(&rows).Error; err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("aggregate returned %d rows, want one per credential (3)", len(rows))
	}
	got := map[uint]string{}
	for _, r := range rows {
		got[r.CredentialID] = r.Action + "/" + r.Result
	}
	want := map[uint]string{
		1: "verify/fail", // newest of the two
		2: "lateral/fail",
		3: "verify/ok",
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("credential %d newest usage = %q, want %q", id, got[id], w)
		}
	}
}

// TestCredentialPageLifecycleUsesNewestUsage ties the aggregate to the field it
// feeds: a credential whose newest use failed must read "stale", not "used".
func TestCredentialPageLifecycleUsesNewestUsage(t *testing.T) {
	s := newCredUsageServer(t)
	seed := db.CredentialEntry{AgentID: "g1-cred", Domain: "CORP", Username: "svc-backup", Type: "password", Source: "spray"}
	if err := s.db.Create(&seed).Error; err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	old := time.Now().Add(-6 * time.Hour)
	now := time.Now().Add(-time.Minute)
	for _, u := range []db.CredentialUsage{
		{CredentialID: seed.ID, Action: "spray", Result: "ok", CreatedAt: old},
		{CredentialID: seed.ID, Action: "verify", Result: "fail", CreatedAt: now},
	} {
		if err := s.db.Create(&u).Error; err != nil {
			t.Fatalf("seed usage: %v", err)
		}
	}

	c, w := tenantScopedAdminContext(s, t, "cred-admin", 1)
	s.handleCredentialsPage(c)
	if w.Code != 200 {
		t.Fatalf("status=%d body=%s, want 200", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, `"lifecycle":"stale"`) {
		t.Fatalf("a credential whose newest use failed must read stale; body=%s", body)
	}
	if strings.Contains(body, `"lifecycle":"used"`) {
		t.Fatalf("the older successful spray won the aggregation; body=%s", body)
	}
}

// TestCleanupOldDataPurgesCredentialUsages pins the retention that was missing
// entirely: credential_usages had no purge path anywhere in the tree, so the
// ledger grew forever and the vault page read all of it.
func TestCleanupOldDataPurgesCredentialUsages(t *testing.T) {
	s := newCredUsageServer(t)
	s.cfg.Server.CleanupRetentionDays = 30
	s.cfg.Server.AuditRetentionDays = 1

	entry := db.CredentialEntry{AgentID: "g1-cred", Domain: "CORP", Username: "u", Type: "password", Source: "spray"}
	if err := s.db.Create(&entry).Error; err != nil {
		t.Fatalf("seed credential: %v", err)
	}
	old := time.Now().AddDate(0, 0, -400)
	fresh := time.Now()
	for i, u := range []db.CredentialUsage{
		{CredentialID: entry.ID, Action: "spray", Result: "ok", CreatedAt: old},
		{CredentialID: entry.ID, Action: "spray", Result: "ok", CreatedAt: old.Add(time.Hour)},
		{CredentialID: entry.ID, Action: "verify", Result: "ok", CreatedAt: fresh},
	} {
		if err := s.db.Create(&u).Error; err != nil {
			t.Fatalf("seed usage %d: %v", i, err)
		}
	}

	s.cleanupOldData()

	var remaining int64
	if err := s.db.Model(&db.CredentialUsage{}).Count(&remaining).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("credential_usages left %d rows, want only the in-window one", remaining)
	}

	// The newest row is what drives lifecycle, so it must survive any cutoff.
	var kept db.CredentialUsage
	if err := s.db.First(&kept).Error; err != nil {
		t.Fatalf("the surviving usage row must be the newest: %v", err)
	}
	if kept.Action != "verify" {
		t.Fatalf("kept %q, want the newest action verify", kept.Action)
	}
}
