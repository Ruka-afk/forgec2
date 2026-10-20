package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"seconds", "90", 90 * time.Second},
		{"zero", "0", 0},
		{"negative", "-5", 0},
		{"invalid", "soon", 0},
		{"empty", "", 0},
		{"http date", now.Add(30 * time.Second).Format(http.TimeFormat), 30 * time.Second},
		{"past date", now.Add(-time.Minute).Format(http.TimeFormat), 0},
		{"clamped", "3600", maxHTTPRetryAfter},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.raw, now); got != tc.want {
				t.Fatalf("parseRetryAfter(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestTakeHTTPRetryAfter(t *testing.T) {
	lastHTTPRetryDeadline.Store(time.Now().Add(5 * time.Second).UnixNano())
	got := takeHTTPRetryAfter()
	if got < 3*time.Second || got > 5*time.Second {
		t.Fatalf("takeHTTPRetryAfter() = %v, want about 5s", got)
	}
	if again := takeHTTPRetryAfter(); again != 0 {
		t.Fatalf("second takeHTTPRetryAfter() = %v, want 0", again)
	}
}

func TestBeaconStatusResetsSession(t *testing.T) {
	if !beaconStatusResetsSession(http.StatusBadRequest) {
		t.Error("400 must reset the session")
	}
	for _, code := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		if beaconStatusResetsSession(code) {
			t.Errorf("%d must not reset the ECDH session", code)
		}
	}
}

func TestBeaconResponseBodyIsJSON(t *testing.T) {
	valid := [][]byte{[]byte(`{"c":"abc"}`), []byte(`{"seq":1,"mac":"x"}`)}
	for _, body := range valid {
		if !beaconResponseBodyIsJSON(body) {
			t.Errorf("valid JSON %q rejected", body)
		}
	}
	for _, body := range [][]byte{nil, []byte(""), []byte("<html>login</html>"), []byte(`{"c":`)} {
		if beaconResponseBodyIsJSON(body) {
			t.Errorf("invalid body %q accepted", body)
		}
	}
}

func TestNoteBeaconFailureRequeuesSentResults(t *testing.T) {
	pendingMu.Lock()
	prevResults := pendingResults
	prevAcks := pendingTaskAcks
	pendingResults = nil
	pendingTaskAcks = nil
	pendingMu.Unlock()
	prevFailures := beaconConsecutiveFailures
	t.Cleanup(func() {
		pendingMu.Lock()
		pendingResults = prevResults
		pendingTaskAcks = prevAcks
		pendingMu.Unlock()
		beaconConsecutiveFailures = prevFailures
	})

	pendingMu.Lock()
	pendingResults = []TaskResult{{TaskID: 9}}
	pendingTaskAcks = []uint{7}
	pendingMu.Unlock()

	noteBeaconFailure(true, []TaskResult{{TaskID: 1}}, []uint{2})

	pendingMu.Lock()
	results := append([]TaskResult(nil), pendingResults...)
	acks := append([]uint(nil), pendingTaskAcks...)
	pendingMu.Unlock()
	if len(results) != 2 || results[0].TaskID != 1 || results[1].TaskID != 9 {
		t.Fatalf("pending results = %+v, want requeued result first", results)
	}
	if len(acks) != 2 || acks[0] != 2 || acks[1] != 7 {
		t.Fatalf("pending acks = %v, want requeued ack first", acks)
	}
	if beaconConsecutiveFailures != prevFailures+1 {
		t.Fatalf("failure count = %d, want %d", beaconConsecutiveFailures, prevFailures+1)
	}
}

func TestSendToC2Treats429AsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded"}`))
	}))
	defer srv.Close()

	prevClient := client
	client = srv.Client()
	defer func() { client = prevClient }()
	withC2URLs(t, srv.URL)
	prevStatus := lastHTTPStatus.Load()
	prevDeadline := lastHTTPRetryDeadline.Load()
	lastHTTPStatus.Store(0)
	lastHTTPRetryDeadline.Store(0)
	defer func() {
		lastHTTPStatus.Store(prevStatus)
		lastHTTPRetryDeadline.Store(prevDeadline)
	}()

	if got := sendToC2(0, []byte(`{}`)); got != nil {
		t.Fatalf("sendToC2() = %q, want nil on 429", got)
	}
	if got := lastHTTPStatus.Load(); got != http.StatusTooManyRequests {
		t.Fatalf("lastHTTPStatus = %d, want 429", got)
	}
	if d := takeHTTPRetryAfter(); d < 5*time.Second || d > 7*time.Second {
		t.Fatalf("Retry-After = %v, want about 7s", d)
	}
}

func TestSendToC2RejectsHTML200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>captive portal</body></html>"))
	}))
	defer srv.Close()

	prevClient := client
	client = srv.Client()
	defer func() { client = prevClient }()
	withC2URLs(t, srv.URL)
	prevStatus := lastHTTPStatus.Load()
	prevDeadline := lastHTTPRetryDeadline.Load()
	lastHTTPStatus.Store(0)
	lastHTTPRetryDeadline.Store(0)
	defer func() {
		lastHTTPStatus.Store(prevStatus)
		lastHTTPRetryDeadline.Store(prevDeadline)
	}()

	if got := sendToC2(0, []byte(`{}`)); got != nil {
		t.Fatalf("sendToC2() = %q, want nil for HTML 200", got)
	}
	if got := lastHTTPStatus.Load(); got != http.StatusOK {
		t.Fatalf("lastHTTPStatus = %d, want 200", got)
	}
}

func TestNoteBeaconFailureDoesNotDuplicateAuthResults(t *testing.T) {
	pendingMu.Lock()
	prevResults := pendingResults
	prevAcks := pendingTaskAcks
	pendingResults = nil
	pendingTaskAcks = nil
	pendingMu.Unlock()
	prevFailures := beaconConsecutiveFailures
	t.Cleanup(func() {
		pendingMu.Lock()
		pendingResults = prevResults
		pendingTaskAcks = prevAcks
		pendingMu.Unlock()
		beaconConsecutiveFailures = prevFailures
	})

	noteBeaconFailure(false, []TaskResult{{TaskID: 1}}, []uint{2})

	pendingMu.Lock()
	defer pendingMu.Unlock()
	if len(pendingResults) != 0 || len(pendingTaskAcks) != 0 {
		t.Fatalf("auth-frame failure requeued results twice: %+v / %v", pendingResults, pendingTaskAcks)
	}
}
