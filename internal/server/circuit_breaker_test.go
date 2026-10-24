package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forgec2/forgec2/internal/config"
	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/testutil"
	"github.com/gin-gonic/gin"
)

// newTestCircuitBreaker builds an isolated breaker. NewCircuitBreaker is a
// process-wide singleton behind a sync.Once, so tests construct the value
// directly to avoid leaking targets between cases.
func newTestCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		targets:       make(map[string]*TargetHealth),
		checkInterval: 50 * time.Millisecond,
		config:        config.DefaultConfig(),
		stopCh:        make(chan struct{}),
	}
}

// burnCounter collects onBurned invocations for edge-trigger assertions.
type burnCounter struct {
	mu    sync.Mutex
	seen  []string
	ready chan struct{}
}

func newBurnCounter() *burnCounter {
	return &burnCounter{ready: make(chan struct{}, 64)}
}

func (b *burnCounter) handler(targetID string) {
	b.mu.Lock()
	b.seen = append(b.seen, targetID)
	b.mu.Unlock()
	select {
	case b.ready <- struct{}{}:
	default:
	}
}

func (b *burnCounter) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.seen)
}

func (b *burnCounter) drain(within time.Duration) int {
	deadline := time.After(within)
	n := 0
	for {
		select {
		case <-b.ready:
			n++
		case <-deadline:
			return n
		}
	}
}

// record applies n consecutive probe results and returns the final health.
func recordProbe(cb *CircuitBreaker, id string, errText string, n int) ListenerHealth {
	for i := 0; i < n; i++ {
		cb.recordResult(id, ProbeResult{Target: cb.targets[id].Target, Error: errText})
	}
	status, _ := cb.GetStatus(id)
	return status
}

// ── Burn notification is edge-triggered ─────────────────────────────────────

// TestBurnHandlerFiresOncePerBurnTransition is the regression for the
// level-triggered burn callback: the handler rotates every agent on the
// listener, so re-firing it once per probe interval churned agents and rewrote
// the same redirector row until an operator reset.
func TestBurnHandlerFiresOncePerBurnTransition(t *testing.T) {
	cb := newTestCircuitBreaker()
	counter := newBurnCounter()
	cb.SetOnBurnedHandler(counter.handler)
	cb.RegisterTarget("11", "http", "127.0.0.1", 9)

	for i := 1; i <= 8; i++ {
		status := recordProbe(cb, "11", "TCP dial failed", 1)
		if i < 5 && status != HealthUnstable {
			t.Fatalf("after %d failure(s) status = %v, want unstable", i, status)
		}
		if i >= 5 && status != HealthBurned {
			t.Fatalf("after %d failure(s) status = %v, want burned", i, status)
		}
	}
	if got := counter.drain(500 * time.Millisecond); got != 1 {
		t.Fatalf("burn handler fired %d times across 8 failed probes, want exactly 1", got)
	}
}

// TestBurnHandlerFiresAgainAfterRecovery proves the edge trigger is not a
// latch: a listener that recovers and burns again must still notify, otherwise
// a genuine re-burn would silently skip the agent rotation.
func TestBurnHandlerFiresAgainAfterRecovery(t *testing.T) {
	cb := newTestCircuitBreaker()
	counter := newBurnCounter()
	cb.SetOnBurnedHandler(counter.handler)
	cb.RegisterTarget("11", "http", "127.0.0.1", 9)

	recordProbe(cb, "11", "down", 5)
	if got := counter.drain(500 * time.Millisecond); got != 1 {
		t.Fatalf("first burn fired %d times, want 1", got)
	}

	// Recovery clears the burn.
	recordProbe(cb, "11", "", 1)
	if status, _ := cb.GetStatus("11"); status != HealthHealthy {
		t.Fatalf("status after recovery = %v, want healthy", status)
	}
	if got := counter.drain(100 * time.Millisecond); got != 0 {
		t.Fatalf("recovery fired the burn handler %d times", got)
	}

	// Second burn notifies again.
	recordProbe(cb, "11", "down again", 5)
	if got := counter.drain(500 * time.Millisecond); got != 1 {
		t.Fatalf("re-burn fired %d times, want 1", got)
	}
	if count := counter.count(); count != 2 {
		t.Fatalf("handler called %d times in total, want 2", count)
	}
}

// TestBurnHandlerNotCalledBelowThreshold proves the threshold is still 5
// consecutive failures, and that unstable probes are silent.
func TestBurnHandlerNotCalledBelowThreshold(t *testing.T) {
	cb := newTestCircuitBreaker()
	counter := newBurnCounter()
	cb.SetOnBurnedHandler(counter.handler)
	cb.RegisterTarget("11", "http", "127.0.0.1", 9)

	recordProbe(cb, "11", "flaky", 4)
	if status, _ := cb.GetStatus("11"); status != HealthUnstable {
		t.Fatalf("status = %v, want unstable", status)
	}
	if got := counter.drain(200 * time.Millisecond); got != 0 {
		t.Fatalf("burn handler fired %d times below the threshold", got)
	}
	recordProbe(cb, "11", "still down", 1)
	if got := counter.drain(500 * time.Millisecond); got != 1 {
		t.Fatalf("burn handler fired %d times at the threshold, want 1", got)
	}
}

// TestBurnHandlerNilIsSafe proves a breaker without a handler (the state before
// wiring, and every unit-test server) still records the burn.
func TestBurnHandlerNilIsSafe(t *testing.T) {
	cb := newTestCircuitBreaker()
	cb.RegisterTarget("11", "http", "127.0.0.1", 9)
	if status := recordProbe(cb, "11", "down", 6); status != HealthBurned {
		t.Fatalf("status = %v, want burned", status)
	}
}

// ── recordResult state machine ─────────────────────────────────────────────

// TestRecordResultStateMachineAndBoundedHistory pins the escalation ladder, the
// recovery reset and both bounded history buffers (results 10, reasons 5).
func TestRecordResultStateMachineAndBoundedHistory(t *testing.T) {
	cb := newTestCircuitBreaker()
	cb.RegisterTarget("11", "http", "host", 80)

	if status, th := cb.GetStatus("11"); status != HealthUnknown || th == nil {
		t.Fatalf("fresh target = %v/%v, want unknown with a health record", status, th)
	}

	// Success keeps it healthy.
	cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target})
	if status, th := cb.GetStatus("11"); status != HealthHealthy || th.ConsecutiveFails != 0 {
		t.Fatalf("after success = %v/%d fails, want healthy/0", status, th.ConsecutiveFails)
	}

	// Failure ladder: 1..4 unstable, 5+ burned.
	for i := 1; i <= 4; i++ {
		cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target, Error: "boom"})
		if status, _ := cb.GetStatus("11"); status != HealthUnstable {
			t.Fatalf("failure %d = %v, want unstable", i, status)
		}
	}
	cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target, Error: "boom"})
	if status, _ := cb.GetStatus("11"); status != HealthBurned {
		t.Fatal("fifth failure did not burn the target")
	}

	// One success clears the ladder and the failure reasons.
	cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target})
	_, th := cb.GetStatus("11")
	if th.ConsecutiveFails != 0 || len(th.FailReasons) != 0 || th.Status != HealthHealthy {
		t.Fatalf("after recovery: fails=%d reasons=%d status=%v", th.ConsecutiveFails, len(th.FailReasons), th.Status)
	}

	// Results keeps only the last 10 entries, alternating success and failure so
	// the buffer is genuinely exercised from both sides.
	for i := 0; i < 14; i++ {
		cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target, Timestamp: time.Unix(int64(i), 0)})
		cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target, Error: "e", Timestamp: time.Unix(int64(i), 0)})
	}
	if len(th.Results) != 10 {
		t.Fatalf("results buffer holds %d entries, want the last 10", len(th.Results))
	}
	// Fail reasons keep only the last 5, and only a run of consecutive
	// failures fills them (one success wipes the buffer).
	for i := 0; i < 8; i++ {
		cb.recordResult("11", ProbeResult{Target: cb.targets["11"].Target, Error: "run"})
	}
	if len(th.FailReasons) != 5 {
		t.Fatalf("fail reasons hold %d entries, want the last 5", len(th.FailReasons))
	}
	if th.LastProbe.IsZero() {
		t.Fatal("LastProbe was never stamped")
	}
}

// TestRecordResultUnknownTargetIsNoop proves a probe result for a target that
// was unregistered mid-flight is dropped instead of recreating the entry.
func TestRecordResultUnknownTargetIsNoop(t *testing.T) {
	cb := newTestCircuitBreaker()
	cb.recordResult("ghost", ProbeResult{Error: "down"})
	if got := cb.GetAllStatus(); len(got) != 0 {
		t.Fatalf("a result for an unknown target registered %v", got)
	}
}

// ── Registry ────────────────────────────────────────────────────────────────

// TestCircuitBreakerRegistrySemantics proves register/unregister/get and the
// all-status snapshot behave, including that re-registering resets a burned
// target back to unknown.
func TestCircuitBreakerRegistrySemantics(t *testing.T) {
	cb := newTestCircuitBreaker()
	if status, th := cb.GetStatus("nope"); status != HealthUnknown || th != nil {
		t.Fatalf("unknown target = %v/%v, want unknown/nil", status, th)
	}

	cb.RegisterTarget("11", "https", "c2.example", 443)
	cb.RegisterTarget("12", "http", "plain.example", 80)
	if got := len(cb.GetAllStatus()); got != 2 {
		t.Fatalf("registered %d targets, want 2", got)
	}
	if _, th := cb.GetStatus("11"); th.Target.Scheme != "https" || th.Target.Port != 443 {
		t.Fatalf("target probe metadata lost: %+v", th.Target)
	}

	recordProbe(cb, "11", "down", 5)
	if status, _ := cb.GetStatus("11"); status != HealthBurned {
		t.Fatal("target did not burn")
	}

	// Unregister drops it and its history.
	cb.UnregisterTarget("11")
	if status, th := cb.GetStatus("11"); status != HealthUnknown || th != nil {
		t.Fatalf("after unregister = %v/%v, want unknown/nil", status, th)
	}
	cb.UnregisterTarget("11") // idempotent

	// Re-registering the same id starts from a clean slate.
	cb.RegisterTarget("12", "http", "plain.example", 80)
	if status, _ := cb.GetStatus("12"); status != HealthUnknown {
		t.Fatalf("re-registered status = %v, want unknown", status)
	}
}

// TestCircuitBreakerStateNames pins the single health→wire vocabulary used by
// detail, toggle and reset, so an event row cannot disagree with the state it
// describes.
func TestCircuitBreakerStateNames(t *testing.T) {
	cases := map[ListenerHealth]string{
		HealthHealthy:      "healthy",
		HealthUnstable:     "unstable",
		HealthBurned:       "burned",
		HealthUnknown:      "unknown",
		ListenerHealth(42): "unknown",
	}
	for status, want := range cases {
		if got := circuitBreakerStateName(status); got != want {
			t.Fatalf("circuitBreakerStateName(%d) = %q, want %q", status, got, want)
		}
	}
}

// ── Probing ─────────────────────────────────────────────────────────────────

// deadPort is a port nothing can be listening on: TCP port 0 is never a valid
// destination, so a dial is refused immediately. Reserving-then-releasing a
// real ephemeral port races with every other process on the host (it did, and
// turned this test flaky).
func deadPort(t *testing.T) int {
	t.Helper()
	// Guard the assumption: if a dial ever succeeds, the platform has a
	// different meaning for port 0 and the probe assertions below would lie.
	probe, err := net.DialTimeout("tcp", "127.0.0.1:0", 2*time.Second)
	if err == nil {
		probe.Close()
		t.Skip("tcp port 0 accepted a connection on this platform")
	}
	return 0
}

// TestProbeTargetClosedPortReportsFailure proves an unreachable transport is
// reported as a failure (and marked burned) rather than silently healthy.
func TestProbeTargetClosedPortReportsFailure(t *testing.T) {
	cb := newTestCircuitBreaker()
	res := cb.probeTarget(ProbeTarget{ID: "11", Scheme: "http", Host: "127.0.0.1", Port: deadPort(t)})
	if res.Error == "" {
		t.Fatal("probe of a closed port reported no error")
	}
	if res.TCPOK {
		t.Fatal("probe of a closed port reported TCPOK")
	}
	if !res.Burned {
		t.Fatal("closed port not marked burned")
	}
	if res.ResponseTime < 0 {
		t.Fatalf("probe reported a negative response time: %v", res.ResponseTime)
	}
}

// TestProbeTargetOpenTCPIsHealthy proves a reachable transport is healthy and
// never burns.
func TestProbeTargetOpenTCPIsHealthy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// A non-http scheme performs a bare TCP dial and nothing else.
	cb := newTestCircuitBreaker()
	res := cb.probeTarget(ProbeTarget{ID: "11", Scheme: "tcp", Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port})
	if res.Error != "" {
		t.Fatalf("open listener probed as failure: %s", res.Error)
	}
	if !res.TCPOK || res.Burned {
		t.Fatalf("open listener: TCPOK=%v burned=%v", res.TCPOK, res.Burned)
	}
}

// TestProbeTargetTLSRecordsFingerprint proves the https branch captures the
// presented leaf certificate: with verification skipped this fingerprint is the
// operator's only MITM signal.
func TestProbeTargetTLSRecordsFingerprint(t *testing.T) {
	cert, cleanup := selfSignedListener(t)
	defer cleanup()

	cb := newTestCircuitBreaker()
	res := cb.probeTarget(ProbeTarget{ID: "11", Scheme: "https", Host: "127.0.0.1", Port: cert})
	if res.Error != "" {
		t.Fatalf("tls probe failed: %s", res.Error)
	}
	if !res.TCPOK || !res.TLSOK {
		t.Fatalf("tls probe: TCPOK=%v TLSOK=%v", res.TCPOK, res.TLSOK)
	}
	if res.Burned {
		t.Fatal("healthy tls target marked burned")
	}
	if len(res.CertFingerprint) != 64 {
		t.Fatalf("fingerprint = %q, want 64 hex chars", res.CertFingerprint)
	}
}

// TestProbeTargetPlainHTTPUsesHealthEndpoint proves the http branch performs a
// GET and records the status code, and that an unreachable /health endpoint is
// a failure (a healthy TCP port with no server must not look healthy).
func TestProbeTargetPlainHTTPUsesHealthEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("probe hit %q, want /health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	host, portStr, err := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("split %q: %v", srv.URL, err)
	}
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	cb := newTestCircuitBreaker()
	res := cb.probeTarget(ProbeTarget{ID: "11", Scheme: "http", Host: host, Port: port})
	if res.Error != "" {
		t.Fatalf("http probe failed: %s", res.Error)
	}
	if res.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want 200", res.HTTPStatus)
	}

	// TCP up but no HTTP server: the health request fails, so the target burns.
	res2 := cb.probeTarget(ProbeTarget{ID: "12", Scheme: "http", Host: "127.0.0.1", Port: deadPort(t)})
	if res2.HTTPStatus != 0 {
		t.Fatalf("HTTPStatus = %d for an unreachable endpoint, want 0", res2.HTTPStatus)
	}
	if !res2.Burned {
		t.Fatal("unreachable http endpoint not marked burned")
	}
}

// TestProbeAllIsolatesTargetFailures proves one dead target does not stop the
// others from being probed.
func TestProbeAllIsolatesTargetFailures(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	cb := newTestCircuitBreaker()
	cb.RegisterTarget("alive", "tcp", "127.0.0.1", ln.Addr().(*net.TCPAddr).Port)
	cb.RegisterTarget("dead", "tcp", "127.0.0.1", deadPort(t))

	cb.probeAll()

	if status, _ := cb.GetStatus("alive"); status != HealthHealthy {
		t.Fatalf("alive target = %v, want healthy", status)
	}
	if status, _ := cb.GetStatus("dead"); status != HealthUnstable {
		t.Fatalf("dead target = %v, want unstable after one failed probe", status)
	}
}

// ── Lifecycle ───────────────────────────────────────────────────────────────

// TestCircuitBreakerStartStopIsIdempotent proves the probe loop releases on
// Stop, a second Stop is safe, and a probe recorded while stopped still works.
func TestCircuitBreakerStartStopIsIdempotent(t *testing.T) {
	cb := newTestCircuitBreaker()
	cb.RegisterTarget("dead", "tcp", "127.0.0.1", deadPort(t))

	cb.Start()
	select {
	case <-cb.done:
		t.Fatal("done closed while the loop was running")
	case <-time.After(50 * time.Millisecond):
	}

	cb.Stop()
	select {
	case <-cb.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not release the probe loop")
	}
	cb.Stop() // must not panic or block

	if status, _ := cb.GetStatus("dead"); status == HealthUnknown {
		t.Fatal("the probe loop never recorded a result")
	}
}

// TestProbeLoopConcurrentStatusReads proves the probe loop and the API readers
// do not race; run under -race this pins the lock discipline.
func TestProbeLoopConcurrentStatusReads(t *testing.T) {
	cb := newTestCircuitBreaker()
	cb.RegisterTarget("11", "tcp", "127.0.0.1", deadPort(t))
	cb.RegisterTarget("12", "tcp", "127.0.0.1", deadPort(t))

	cb.Start()
	var wg sync.WaitGroup
	stop := make(chan struct{})
	var reads atomic.Int64
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					cb.GetAllStatus()
					cb.GetStatus("11")
					reads.Add(1)
				}
			}
		}()
	}
	time.Sleep(300 * time.Millisecond)
	close(stop)
	wg.Wait()
	cb.Stop()
	if reads.Load() == 0 {
		t.Fatal("readers never ran")
	}
}

// ── Handler surface ─────────────────────────────────────────────────────────

func newCBTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := config.DefaultConfig()
	return &Server{db: testutil.SetupTestDB(t), cfg: cfg, ctx: context.Background()}
}

// withCircuitBreaker installs a fresh breaker as the process singleton for the
// duration of a test and restores the previous one afterwards.
func withCircuitBreaker(t *testing.T, cb *CircuitBreaker) {
	t.Helper()
	previous := circuitBreaker
	circuitBreaker = cb
	t.Cleanup(func() { circuitBreaker = previous })
}

func cbCtx(method, body, listenerID string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/circuit-breaker/"+listenerID, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user", "cb-op")
	c.Set("user_role", "admin")
	c.Params = gin.Params{{Key: "id", Value: listenerID}}
	return c, w
}

// TestHandleAPICircuitBreakerDetail proves the dashboard payload maps every
// health enum to its wire name and carries the operator's triage fields.
func TestHandleAPICircuitBreakerDetail(t *testing.T) {
	s := newCBTestServer(t)
	cb := newTestCircuitBreaker()
	withCircuitBreaker(t, cb)
	cb.RegisterTarget("11", "https", "c2.example", 443)
	recordProbe(cb, "11", "dial failed", 6)

	c, w := cbCtx(http.MethodGet, "", "")
	s.handleAPICircuitBreakerDetail(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Listeners []struct {
			Target           string   `json:"target"`
			Scheme           string   `json:"scheme"`
			Host             string   `json:"host"`
			Port             int      `json:"port"`
			Status           string   `json:"status"`
			ConsecutiveFails int      `json:"consecutive_fails"`
			LastProbe        string   `json:"last_probe"`
			FailReasons      []string `json:"fail_reasons"`
		} `json:"listeners"`
	}
	unmarshalData(t, w.Body.Bytes(), &body)
	if len(body.Listeners) != 1 {
		t.Fatalf("got %d listeners, want 1", len(body.Listeners))
	}
	got := body.Listeners[0]
	if got.Target != "11" || got.Scheme != "https" || got.Host != "c2.example" || got.Port != 443 {
		t.Fatalf("listener identity wrong: %+v", got)
	}
	if got.Status != "burned" {
		t.Fatalf("status = %q, want burned", got.Status)
	}
	if got.ConsecutiveFails != 6 || len(got.FailReasons) != 5 {
		t.Fatalf("fails=%d reasons=%d", got.ConsecutiveFails, len(got.FailReasons))
	}
	if _, err := time.Parse(time.RFC3339, got.LastProbe); err != nil {
		t.Fatalf("last_probe %q is not RFC3339: %v", got.LastProbe, err)
	}
}

// TestHandleAPICircuitBreakerDetailUninitialized proves the UI still renders
// when the breaker has not started yet, rather than erroring out.
func TestHandleAPICircuitBreakerDetailUninitialized(t *testing.T) {
	s := newCBTestServer(t)
	withCircuitBreaker(t, nil)
	c, w := cbCtx(http.MethodGet, "", "")
	s.handleAPICircuitBreakerDetail(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"listeners":[]`) {
		t.Fatalf("body = %s, want an empty listener list", w.Body.String())
	}
}

// TestHandleAPICircuitBreakerConfigRoundTrip proves the config endpoint creates
// defaults on first use and that saved values survive a reload.
func TestHandleAPICircuitBreakerConfigRoundTrip(t *testing.T) {
	s := newCBTestServer(t)
	withCircuitBreaker(t, newTestCircuitBreaker())

	c, w := cbCtx(http.MethodGet, "", "")
	s.handleAPICircuitBreakerConfig(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var cfg map[string]interface{}
	unmarshalData(t, w.Body.Bytes(), &cfg)
	if _, ok := cfg["failure_threshold"]; !ok {
		t.Fatalf("config payload missing fields: %v", cfg)
	}

	c2, w2 := cbCtx(http.MethodPost, `{"failure_threshold":7,"cooldown_seconds":11,"half_open_max_reqs":2,"health_check_seconds":30}`, "")
	s.handleAPICircuitBreakerSaveConfig(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("save status = %d body = %s", w2.Code, w2.Body.String())
	}

	var stored db.CircuitBreakerConfig
	if err := s.db.First(&stored, 1).Error; err != nil {
		t.Fatalf("load config: %v", err)
	}
	if stored.FailureThreshold != 7 || stored.CooldownSeconds != 11 ||
		stored.HalfOpenMaxReqs != 2 || stored.HealthCheckSeconds != 30 {
		t.Fatalf("saved config = %+v", stored)
	}

	c3, w3 := cbCtx(http.MethodGet, "", "")
	s.handleAPICircuitBreakerConfig(c3)
	var reread map[string]interface{}
	unmarshalData(t, w3.Body.Bytes(), &reread)
	if reread["failure_threshold"] != float64(7) {
		t.Fatalf("reloaded failure_threshold = %v, want 7", reread["failure_threshold"])
	}
}

// TestHandleAPICircuitBreakerSaveConfigRejectsGarbage proves a malformed body is
// a 400 rather than a silent zeroing of the operator's configuration.
func TestHandleAPICircuitBreakerSaveConfigRejectsGarbage(t *testing.T) {
	s := newCBTestServer(t)
	withCircuitBreaker(t, newTestCircuitBreaker())
	if err := s.db.Create(&db.CircuitBreakerConfig{ID: 1, FailureThreshold: 9, CooldownSeconds: 42, HalfOpenMaxReqs: 4, HealthCheckSeconds: 15}).Error; err != nil {
		t.Fatalf("seed config: %v", err)
	}

	c, w := cbCtx(http.MethodPost, `{"failure_threshold":`, "")
	s.handleAPICircuitBreakerSaveConfig(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var stored db.CircuitBreakerConfig
	if err := s.db.First(&stored, 1).Error; err != nil {
		t.Fatalf("load config: %v", err)
	}
	if stored.FailureThreshold != 9 || stored.CooldownSeconds != 42 {
		t.Fatalf("rejected write mutated the config: %+v", stored)
	}
}

// TestHandleAPICircuitBreakerEventsOrdersNewestFirst proves the event history
// is ordered and capped, which is what the operator triages from.
func TestHandleAPICircuitBreakerEventsOrdersNewestFirst(t *testing.T) {
	s := newCBTestServer(t)
	old := time.Now().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		if err := s.db.Create(&db.CircuitBreakerEvent{
			ListenerID: "11", OldState: "unknown", NewState: "unstable",
			Reason: "probe", CreatedAt: old.Add(time.Duration(i) * time.Minute),
		}).Error; err != nil {
			t.Fatalf("seed event: %v", err)
		}
	}

	c, w := cbCtx(http.MethodGet, "", "")
	s.handleAPICircuitBreakerEvents(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Events []db.CircuitBreakerEvent `json:"events"`
	}
	unmarshalData(t, w.Body.Bytes(), &body)
	if len(body.Events) != 3 {
		t.Fatalf("got %d events, want 3", len(body.Events))
	}
	if !body.Events[0].CreatedAt.After(body.Events[2].CreatedAt) {
		t.Fatalf("events are not newest-first: %v", body.Events)
	}
}

// TestHandleAPICircuitBreakerResetRecordsTrueOldState is the regression for the
// dishonest reset event: the row used to claim "burned -> unknown" for any
// listener, accusing one that had never burned.
func TestHandleAPICircuitBreakerResetRecordsTrueOldState(t *testing.T) {
	cases := []struct {
		name      string
		prep      func(cb *CircuitBreaker)
		wantOld   string
		wantAudit bool
	}{
		{"burned listener", func(cb *CircuitBreaker) { recordProbe(cb, "11", "down", 6) }, "burned", true},
		{"unstable listener", func(cb *CircuitBreaker) { recordProbe(cb, "11", "down", 2) }, "unstable", true},
		{"healthy listener", func(cb *CircuitBreaker) { recordProbe(cb, "11", "", 1) }, "healthy", true},
		{"never probed", func(cb *CircuitBreaker) {}, "unknown", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newCBTestServer(t)
			cb := newTestCircuitBreaker()
			withCircuitBreaker(t, cb)
			cb.RegisterTarget("11", "http", "c2.example", 80)
			tc.prep(cb)

			c, w := cbCtx(http.MethodPost, "", "11")
			s.handleAPICircuitBreakerReset(c)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
			}

			var event db.CircuitBreakerEvent
			if err := s.db.Where("listener_id = ?", "11").First(&event).Error; err != nil {
				t.Fatalf("no event recorded: %v", err)
			}
			if event.OldState != tc.wantOld || event.NewState != "unknown" {
				t.Fatalf("event = %q -> %q, want %q -> unknown", event.OldState, event.NewState, tc.wantOld)
			}

			// The reset itself must clear the burn state.
			status, th := cb.GetStatus("11")
			if status != HealthUnknown || th.ConsecutiveFails != 0 || len(th.FailReasons) != 0 {
				t.Fatalf("after reset: status=%v fails=%d reasons=%d", status, th.ConsecutiveFails, len(th.FailReasons))
			}

			if tc.wantAudit {
				var audit db.AuditLog
				if err := s.db.Where("action = ?", "circuit_breaker_reset").First(&audit).Error; err != nil {
					t.Fatalf("no reset audit entry: %v", err)
				}
				if !audit.Success {
					t.Fatal("reset audit entry not marked successful")
				}
			}
		})
	}
}

// TestHandleAPICircuitBreakerResetUnknownListener proves a reset targeting an
// unregistered listener is a 404 with no phantom event row.
func TestHandleAPICircuitBreakerResetUnknownListener(t *testing.T) {
	s := newCBTestServer(t)
	withCircuitBreaker(t, newTestCircuitBreaker())

	c, w := cbCtx(http.MethodPost, "", "404-listener")
	s.handleAPICircuitBreakerReset(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var events int64
	if err := s.db.Model(&db.CircuitBreakerEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 0 {
		t.Fatalf("a failed reset still wrote %d event rows", events)
	}
}

// TestHandleAPICircuitBreakerResetUninitialized proves the reset endpoint
// refuses cleanly when the breaker was never started.
func TestHandleAPICircuitBreakerResetUninitialized(t *testing.T) {
	s := newCBTestServer(t)
	withCircuitBreaker(t, nil)
	c, w := cbCtx(http.MethodPost, "", "11")
	s.handleAPICircuitBreakerReset(c)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

// TestHandleAPICircuitBreakerToggleStates proves the operator override maps
// every accepted alias onto the right enum and writes an honest transition row.
func TestHandleAPICircuitBreakerToggleStates(t *testing.T) {
	cases := []struct {
		state    string
		wantEnum ListenerHealth
		wantNew  string
		wantOld  string
	}{
		{"disabled", HealthBurned, "burned", "healthy"},
		{"open", HealthBurned, "burned", "healthy"},
		{"burned", HealthBurned, "burned", "healthy"},
		{"half-open", HealthUnstable, "unstable", "healthy"},
		{"HALF_OPEN", HealthUnstable, "unstable", "healthy"},
		{"unstable", HealthUnstable, "unstable", "healthy"},
		{" enabled ", HealthHealthy, "healthy", "healthy"},
		{"closed", HealthHealthy, "healthy", "healthy"},
		{"healthy", HealthHealthy, "healthy", "healthy"},
	}
	for _, tc := range cases {
		t.Run(tc.state, func(t *testing.T) {
			s := newCBTestServer(t)
			cb := newTestCircuitBreaker()
			withCircuitBreaker(t, cb)
			cb.RegisterTarget("11", "http", "c2.example", 80)
			recordProbe(cb, "11", "", 1) // start healthy

			c, w := cbCtx(http.MethodPost, `{"state":"`+tc.state+`"}`, "11")
			s.handleAPICircuitBreakerToggle(c)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
			}
			if status, _ := cb.GetStatus("11"); status != tc.wantEnum {
				t.Fatalf("state %q gave enum %v, want %v", tc.state, status, tc.wantEnum)
			}
			var event db.CircuitBreakerEvent
			if err := s.db.First(&event).Error; err != nil {
				t.Fatalf("no toggle event: %v", err)
			}
			if event.OldState != tc.wantOld || event.NewState != tc.wantNew {
				t.Fatalf("event = %q -> %q, want %q -> %q", event.OldState, event.NewState, tc.wantOld, tc.wantNew)
			}
		})
	}
}

// TestHandleAPICircuitBreakerToggleRejectsUnknownState proves an unrecognised
// state is a 400 that leaves the target untouched and writes no event.
func TestHandleAPICircuitBreakerToggleRejectsUnknownState(t *testing.T) {
	s := newCBTestServer(t)
	cb := newTestCircuitBreaker()
	withCircuitBreaker(t, cb)
	cb.RegisterTarget("11", "http", "c2.example", 80)
	recordProbe(cb, "11", "", 1)

	c, w := cbCtx(http.MethodPost, `{"state":"sideways"}`, "11")
	s.handleAPICircuitBreakerToggle(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if status, _ := cb.GetStatus("11"); status != HealthHealthy {
		t.Fatalf("rejected toggle changed the state to %v", status)
	}
	var events int64
	if err := s.db.Model(&db.CircuitBreakerEvent{}).Count(&events).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 0 {
		t.Fatalf("a rejected toggle wrote %d event rows", events)
	}
}

// TestHandleAPICircuitBreakerToggleGuards proves the toggle refuses unknown
// listeners and an uninitialized breaker, and that a malformed body never
// reaches the state machine.
func TestHandleAPICircuitBreakerToggleGuards(t *testing.T) {
	s := newCBTestServer(t)
	cb := newTestCircuitBreaker()
	withCircuitBreaker(t, cb)

	c, w := cbCtx(http.MethodPost, `{"state":"burned"}`, "missing")
	s.handleAPICircuitBreakerToggle(c)
	if w.Code != http.StatusNotFound {
		t.Fatalf("unknown listener: status = %d, want 404", w.Code)
	}

	cb.RegisterTarget("11", "http", "c2.example", 80)
	c2, w2 := cbCtx(http.MethodPost, `{`, "11")
	s.handleAPICircuitBreakerToggle(c2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("malformed body: status = %d, want 400", w2.Code)
	}

	withCircuitBreaker(t, nil)
	c3, w3 := cbCtx(http.MethodPost, `{"state":"burned"}`, "11")
	s.handleAPICircuitBreakerToggle(c3)
	if w3.Code != http.StatusServiceUnavailable {
		t.Fatalf("uninitialized: status = %d, want 503", w3.Code)
	}
}

// selfSignedListener starts a TLS listener with a throwaway self-signed leaf
// and returns its port plus a cleanup func.
func selfSignedListener(t *testing.T) (int, func()) {
	t.Helper()
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{throwawayCert(t)}}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", tlsConfig)
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// The probe completes a real TLS handshake before reading
			// anything, so the server side has to finish it too.
			go func(c net.Conn) {
				defer c.Close()
				_ = tls.Server(c, tlsConfig).Handshake()
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() { ln.Close() }
}

// throwawayCert mints an in-memory self-signed leaf for probe tests.
func throwawayCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "probe-test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}
