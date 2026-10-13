package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ── Hub registration semantics ──────────────────────────────────────────────

func newTestBeacon(agentID string) *WebSocketBeacon {
	return &WebSocketBeacon{
		AgentID:    agentID,
		Send:       make(chan []byte, 8),
		BatchQueue: make([][]byte, 0, 16),
		flush:      make(chan struct{}, 1),
	}
}

// channelClosed reports whether ch has been closed. Test channels carry no
// buffered payloads, so a ready receive means "closed" and the default branch
// means "open and empty".
func channelClosed(ch chan []byte) bool {
	select {
	case _, ok := <-ch:
		return !ok
	default:
		return false
	}
}

// TestWebSocketHubRegisterReplacesPredecessor proves a second registration for
// the same agent evicts the previous connection AND closes its Send channel —
// otherwise the old write pump blocks forever on a channel nobody drains.
func TestWebSocketHubRegisterReplacesPredecessor(t *testing.T) {
	h := NewWebSocketHub()
	first := newTestBeacon("a1")
	h.Register("a1", first)
	if h.Get("a1") != first {
		t.Fatal("first registration did not take effect")
	}

	second := newTestBeacon("a1")
	h.Register("a1", second)
	if !channelClosed(first.Send) {
		t.Fatal("evicted connection's Send channel was left open")
	}
	if channelClosed(second.Send) {
		t.Fatal("newly registered connection had its Send channel closed")
	}
	if h.Get("a1") != second {
		t.Fatal("hub still points at the evicted connection")
	}
}

// TestWebSocketHubRegisterIsIdempotentForSameBeacon proves re-registering the
// SAME connection does not self-close its Send channel.
func TestWebSocketHubRegisterIsIdempotentForSameBeacon(t *testing.T) {
	h := NewWebSocketHub()
	b := newTestBeacon("a1")
	h.Register("a1", b)
	h.Register("a1", b)
	h.Register("a1", b)
	if channelClosed(b.Send) {
		t.Fatal("same-beacon re-registration closed its own Send channel")
	}
	if h.Get("a1") != b {
		t.Fatal("same-beacon re-registration dropped the entry")
	}
}

// TestWebSocketHubRegisterExclusiveRejectsLiveConnection proves the atomic
// duplicate-connection guard: a recently-seen connection is returned as the
// conflict and stays in place, while the challenger's channel is untouched
// (the caller closes the socket itself).
func TestWebSocketHubRegisterExclusiveRejectsLiveConnection(t *testing.T) {
	h := NewWebSocketHub()
	live := newTestBeacon("a1")
	live.touchLastSeen()
	if conflict := h.RegisterExclusive("a1", live, 30*time.Second); conflict != nil {
		t.Fatal("first registration reported a conflict")
	}

	challenger := newTestBeacon("a1")
	conflict := h.RegisterExclusive("a1", challenger, 30*time.Second)
	if conflict != live {
		t.Fatalf("conflict = %p, want the live connection %p", conflict, live)
	}
	if h.Get("a1") != live {
		t.Fatal("rejected challenger displaced the live connection")
	}
	if channelClosed(live.Send) {
		t.Fatal("rejecting a duplicate killed the live connection")
	}
	if channelClosed(challenger.Send) {
		t.Fatal("hub closed the rejected challenger's channel")
	}
}

// TestWebSocketHubRegisterExclusiveEvictsStaleConnection proves the mirror
// case: a connection that has not been seen within the window (here: never
// seen at all) is evicted and replaced instead of blocking reconnects forever.
func TestWebSocketHubRegisterExclusiveEvictsStaleConnection(t *testing.T) {
	h := NewWebSocketHub()
	// A beacon whose lastSeenNano was never set reads back as the Unix epoch,
	// which is far outside any stale window.
	stale := newTestBeacon("a1")
	if conflict := h.RegisterExclusive("a1", stale, 30*time.Second); conflict != nil {
		t.Fatal("first registration reported a conflict")
	}

	fresh := newTestBeacon("a1")
	fresh.touchLastSeen()
	if conflict := h.RegisterExclusive("a1", fresh, 30*time.Second); conflict != nil {
		t.Fatalf("stale connection was not evicted (conflict %p)", conflict)
	}
	if !channelClosed(stale.Send) {
		t.Fatal("evicted stale connection kept its Send channel open")
	}
	if h.Get("a1") != fresh {
		t.Fatal("hub did not point at the new connection")
	}
}

// TestWebSocketHubRegisterExclusiveSelfReentry proves a connection
// re-announcing itself (e.g. after a UUID migration) is never treated as its
// own duplicate.
func TestWebSocketHubRegisterExclusiveSelfReentry(t *testing.T) {
	h := NewWebSocketHub()
	b := newTestBeacon("a1")
	b.touchLastSeen()
	h.RegisterExclusive("a1", b, 30*time.Second)

	if conflict := h.RegisterExclusive("a1", b, 30*time.Second); conflict != nil {
		t.Fatal("same-beacon re-registration reported a conflict")
	}
	if channelClosed(b.Send) {
		t.Fatal("same-beacon re-registration closed its own Send channel")
	}
}

// TestWebSocketHubUnregisterIsolatesConnections proves a connection can only
// tear down ITS OWN entry: a stale connection calling Unregister must not kill
// the live connection's channel (the cross-connection DoS the guard prevents).
func TestWebSocketHubUnregisterIsolatesConnections(t *testing.T) {
	h := NewWebSocketHub()
	live := newTestBeacon("a1")
	h.Register("a1", live)

	stale := newTestBeacon("a1")
	h.Unregister("a1", stale)
	if h.Get("a1") != live {
		t.Fatal("foreign Unregister removed the live entry")
	}
	if channelClosed(live.Send) {
		t.Fatal("foreign Unregister closed the live connection's channel")
	}

	h.Unregister("a1", live)
	if h.Get("a1") != nil {
		t.Fatal("owner Unregister left the entry behind")
	}
	if !channelClosed(live.Send) {
		t.Fatal("owner Unregister did not close the channel")
	}
	// A second teardown (read pump + write pump both defer Unregister) must be
	// a no-op instead of a double close panic.
	h.Unregister("a1", live)
	h.Unregister("a1", stale)
}

// TestWebSocketHubRemoveKeepsChannelOpen proves Remove drops the entry without
// closing the channel — the UUID-migration path needs the connection alive.
func TestWebSocketHubRemoveKeepsChannelOpen(t *testing.T) {
	h := NewWebSocketHub()
	b := newTestBeacon("conn-uuid")
	h.Register("conn-uuid", b)

	h.Remove("conn-uuid", b)
	if h.Get("conn-uuid") != nil {
		t.Fatal("Remove left the entry behind")
	}
	if channelClosed(b.Send) {
		t.Fatal("Remove closed the migrating connection's Send channel")
	}
	// The migrated connection registers under its real UUID.
	h.Register("real-uuid", b)
	if h.Get("real-uuid") != b {
		t.Fatal("migrated connection was not registered under its real UUID")
	}
}

// TestWebSocketHubRemoveIgnoresForeignBeacon proves a stale connection cannot
// evict someone else's entry through Remove.
func TestWebSocketHubRemoveIgnoresForeignBeacon(t *testing.T) {
	h := NewWebSocketHub()
	owner := newTestBeacon("a1")
	foreign := newTestBeacon("a1")
	h.Register("a1", owner)

	h.Remove("a1", foreign)
	if h.Get("a1") != owner {
		t.Fatal("foreign Remove evicted the live entry")
	}
	if channelClosed(owner.Send) {
		t.Fatal("foreign Remove closed the live channel")
	}
	if channelClosed(foreign.Send) {
		t.Fatal("foreign Remove closed a channel it does not own")
	}
}

// TestWebSocketHubGetUnknownAgent proves the miss path returns nil rather than
// a zero beacon (callers branch on it to decide whether to queue).
func TestWebSocketHubGetUnknownAgent(t *testing.T) {
	h := NewWebSocketHub()
	if got := h.Get("nobody"); got != nil {
		t.Fatalf("Get(unknown) = %p, want nil", got)
	}
	b := newTestBeacon("a1")
	h.Register("a1", b)
	if got := h.Get("a1"); got != b {
		t.Fatal("Get returned the wrong beacon")
	}
}

// TestWebSocketHubRegisterExclusiveConcurrentChallengersAllRejected hammers
// the claim path the way duplicate upgrades actually race it: with a live
// connection in place, every concurrent challenger must be rejected and the
// incumbent must survive. Run under -race this also pins the single critical
// section.
func TestWebSocketHubRegisterExclusiveConcurrentChallengersAllRejected(t *testing.T) {
	h := NewWebSocketHub()
	incumbent := newTestBeacon("a1")
	incumbent.touchLastSeen()
	h.RegisterExclusive("a1", incumbent, time.Minute)

	const n = 16
	challengers := make([]*WebSocketBeacon, n)
	rejected := make([]bool, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		challengers[i] = newTestBeacon("a1")
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rejected[idx] = h.RegisterExclusive("a1", challengers[idx], time.Minute) != nil
		}(i)
	}
	wg.Wait()

	for i, got := range rejected {
		if !got {
			t.Fatalf("challenger %d displaced a live connection", i)
		}
	}
	if h.Get("a1") != incumbent {
		t.Fatal("hub entry changed under concurrent duplicate upgrades")
	}
	if channelClosed(incumbent.Send) {
		t.Fatal("incumbent was killed by concurrent duplicate upgrades")
	}
	for i, b := range challengers {
		if channelClosed(b.Send) {
			t.Fatalf("rejected challenger %d had its channel closed", i)
		}
	}
}

// TestWebSocketHubConcurrentLifecycleNoDoubleClose exercises the hub under the
// -race detector: independent agents register/unregister while others churn.
func TestWebSocketHubConcurrentLifecycleNoDoubleClose(t *testing.T) {
	h := NewWebSocketHub()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				id := string(rune('a'+worker)) + "-agent"
				b := newTestBeacon(id)
				h.Register(id, b)
				h.RegisterExclusive(id, b, time.Minute)
				_ = h.Get(id)
				h.Remove(id, b)
				h.Unregister(id, b)
				h.Unregister(id, b) // double teardown must stay a no-op
			}
		}(w)
	}
	wg.Wait()
	if got := len(h.beacons); got != 0 {
		t.Fatalf("hub retained %d entries after teardown", got)
	}
}

// ── Beacon / operator-session liveness ──────────────────────────────────────

// TestWebSocketBeaconLastSeen proves the atomic liveness stamp starts unset and
// advances on touch (presence scans and the duplicate guard read it).
func TestWebSocketBeaconLastSeen(t *testing.T) {
	b := newTestBeacon("a1")
	if got := b.lastSeenAt().UnixNano(); got != 0 {
		t.Fatalf("fresh beacon lastSeen = %d, want 0", got)
	}
	b.touchLastSeen()
	first := b.lastSeenAt()
	if first.UnixNano() == 0 {
		t.Fatal("touchLastSeen did not stamp the beacon")
	}
	if lag := time.Since(first); lag > time.Minute || lag < 0 {
		t.Fatalf("lastSeenAt is %v away from now", lag)
	}
	time.Sleep(2 * time.Millisecond)
	b.touchLastSeen()
	if !b.lastSeenAt().After(first) {
		t.Fatal("second touchLastSeen did not advance the stamp")
	}
}

// TestWSOperatorSessionLastSeen mirrors the beacon stamp for the operator
// socket tracker, which is read by presence scans from other goroutines.
func TestWSOperatorSessionLastSeen(t *testing.T) {
	sess := &WSOperatorSession{UserID: 7, Username: "op"}
	if got := sess.lastSeenAt().UnixNano(); got != 0 {
		t.Fatalf("fresh session lastSeen = %d, want 0", got)
	}
	sess.touchLastSeen()
	stamped := sess.lastSeenAt()
	if lag := time.Since(stamped); lag > time.Minute || lag < 0 {
		t.Fatalf("lastSeenAt is %v away from now", lag)
	}
}

// ── Compression ─────────────────────────────────────────────────────────────

// TestGzipCompressRoundTrip proves every payload — including empty and binary
// — survives a compress/decompress cycle, since the implant decodes these
// frames directly.
func TestGzipCompressRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"text", []byte(`{"type":"task","id":42}`)},
		{"binary with NULs", bytes.Repeat([]byte{0x00, 0xff, 0x7f, 0x80}, 1024)},
		{"highly compressible", bytes.Repeat([]byte("A"), 200_000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := gzipCompress(tc.data)
			if err != nil {
				t.Fatalf("gzipCompress: %v", err)
			}
			zr, err := gzip.NewReader(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("output is not a valid gzip stream: %v", err)
			}
			defer zr.Close()
			back, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			if !bytes.Equal(back, tc.data) {
				t.Fatalf("round trip mismatch: %d bytes in, %d out", len(tc.data), len(back))
			}
		})
	}
}

// TestGzipCompressShrinksRepetitivePayload proves the compression path is
// actually worth taking for large results.
func TestGzipCompressShrinksRepetitivePayload(t *testing.T) {
	in := bytes.Repeat([]byte(`{"output":"AAAA"}`), 4096)
	out, err := gzipCompress(in)
	if err != nil {
		t.Fatalf("gzipCompress: %v", err)
	}
	if len(out) >= len(in) {
		t.Fatalf("compressed %d bytes from %d — no gain", len(out), len(in))
	}
}

// ── Batching ────────────────────────────────────────────────────────────────

// newBeaconConnPair stands up a real WebSocket pair so the write path
// (SetWriteDeadline + NextWriter + Close) runs end to end.
func newBeaconConnPair(t *testing.T) (serverConn, clientConn *websocket.Conn) {
	t.Helper()
	upgraded := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		upgraded <- conn
		// Return immediately: the connection is hijacked, so returning does
		// not close it and httptest stops tracking it.
	}))
	t.Cleanup(srv.Close)

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	clientConn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	select {
	case serverConn = <-upgraded:
	case <-time.After(5 * time.Second):
		t.Fatal("server never completed the upgrade")
	}
	t.Cleanup(func() { serverConn.Close(); clientConn.Close() })
	return serverConn, clientConn
}

// readFrame reads one frame with a bounded deadline so a missing send fails the
// test instead of hanging it.
func readFrame(t *testing.T, c *websocket.Conn) (int, []byte) {
	t.Helper()
	if err := c.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	mt, data, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	return mt, data
}

// TestEnqueueBatchMessageArmsTimerAndSignalsAtThreshold proves the batching
// policy: the first message arms the flush timer, messages below the threshold
// stay silent, and reaching the threshold signals an immediate flush.
func TestEnqueueBatchMessageArmsTimerAndSignalsAtThreshold(t *testing.T) {
	s := &Server{}
	beacon := newTestBeacon("a1")
	beacon.BatchQueue = nil

	s.enqueueBatchMessage(beacon, []byte(`{"i":1}`))
	beacon.BatchMutex.Lock()
	timer := beacon.BatchTimer
	queued := len(beacon.BatchQueue)
	beacon.BatchMutex.Unlock()
	if timer == nil {
		t.Fatal("first message did not arm the flush timer")
	}
	timer.Stop()
	if queued != 1 {
		t.Fatalf("queue holds %d messages, want 1", queued)
	}
	select {
	case <-beacon.flush:
		t.Fatal("flush signalled before the threshold")
	default:
	}

	for i := 1; i < BatchFlushThreshold; i++ {
		s.enqueueBatchMessage(beacon, []byte(`{"i":2}`))
	}
	select {
	case <-beacon.flush:
	default:
		t.Fatalf("reaching the threshold (%d) did not signal a flush", BatchFlushThreshold)
	}

	// The flush signal is a capacity-1 channel: a second burst must never
	// block a sender.
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.enqueueBatchMessage(beacon, []byte(`{"i":3}`))
		s.signalBatchFlush(beacon)
		s.signalBatchFlush(beacon)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("signalling a flush twice blocked the caller")
	}
}

// TestFlushBatchQueueSingleMessageSentVerbatim proves a one-message flush is
// written through unwrapped (no batchedMessage envelope) so the agent's parser
// sees the same shape it always has.
func TestFlushBatchQueueSingleMessageSentVerbatim(t *testing.T) {
	srvConn, cliConn := newBeaconConnPair(t)
	s := &Server{}
	beacon := newTestBeacon("a1")
	beacon.Conn = srvConn

	payload := []byte(`{"type":"task_output","data":"x"}`)
	s.enqueueBatchMessage(beacon, payload)
	beacon.BatchMutex.Lock()
	if beacon.BatchTimer != nil {
		beacon.BatchTimer.Stop()
	}
	beacon.BatchMutex.Unlock()

	s.flushBatchQueue(beacon)

	mt, data := readFrame(t, cliConn)
	if mt != websocket.BinaryMessage {
		t.Fatalf("frame type = %d, want binary", mt)
	}
	if !bytes.Equal(data, payload) {
		t.Fatalf("frame = %s, want the payload verbatim", data)
	}
	beacon.BatchMutex.Lock()
	remaining := len(beacon.BatchQueue)
	beacon.BatchMutex.Unlock()
	if remaining != 0 {
		t.Fatalf("flush left %d messages queued", remaining)
	}
}

// TestFlushBatchQueueBatchesMultipleMessages proves two or more queued messages
// are merged into the documented {"messages":[...]} envelope, order preserved.
func TestFlushBatchQueueBatchesMultipleMessages(t *testing.T) {
	srvConn, cliConn := newBeaconConnPair(t)
	s := &Server{}
	beacon := newTestBeacon("a1")
	beacon.Conn = srvConn

	msgs := [][]byte{[]byte(`{"i":1}`), []byte(`{"i":2}`), []byte(`{"i":3}`)}
	for _, m := range msgs {
		s.enqueueBatchMessage(beacon, m)
	}
	beacon.BatchMutex.Lock()
	if beacon.BatchTimer != nil {
		beacon.BatchTimer.Stop()
	}
	beacon.BatchMutex.Unlock()

	s.flushBatchQueue(beacon)

	_, data := readFrame(t, cliConn)
	var envelope batchedMessage
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("frame is not a batched envelope: %v (%s)", err, data)
	}
	if len(envelope.Messages) != len(msgs) {
		t.Fatalf("envelope carries %d messages, want %d", len(envelope.Messages), len(msgs))
	}
	for i, want := range msgs {
		if string(envelope.Messages[i]) != string(want) {
			t.Fatalf("message %d = %s, want %s (order not preserved)", i, envelope.Messages[i], want)
		}
	}
}

// TestFlushBatchQueueCompressesLargePayloadOnlyForCompressBeacons proves the
// size threshold (256 bytes) and that the compressed frame decompresses back
// to exactly what would have been sent uncompressed.
func TestFlushBatchQueueCompressesLargePayloadOnlyForCompressBeacons(t *testing.T) {
	cases := []struct {
		name      string
		compress  bool
		size      int
		wantPlain bool
	}{
		{"compress beacon small payload stays plain", true, 64, true},
		{"compress beacon large payload is gzipped", true, 4096, false},
		{"plain beacon large payload stays plain", false, 4096, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srvConn, cliConn := newBeaconConnPair(t)
			s := &Server{}
			beacon := newTestBeacon("a1")
			beacon.Conn = srvConn
			beacon.compress = tc.compress

			payload := bytes.Repeat([]byte("A"), tc.size)
			s.enqueueBatchMessage(beacon, payload)
			beacon.BatchMutex.Lock()
			if beacon.BatchTimer != nil {
				beacon.BatchTimer.Stop()
			}
			beacon.BatchMutex.Unlock()

			s.flushBatchQueue(beacon)

			_, data := readFrame(t, cliConn)
			if tc.wantPlain {
				if !bytes.Equal(data, payload) {
					t.Fatalf("expected an uncompressed frame, got %d bytes", len(data))
				}
				return
			}
			zr, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("frame is not gzipped: %v", err)
			}
			defer zr.Close()
			back, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			if !bytes.Equal(back, payload) {
				t.Fatalf("decompressed %d bytes, want %d", len(back), len(payload))
			}
		})
	}
}

// TestFlushBatchQueueRequeuesUndeliveredBatch proves a failed write puts the
// batch back at the FRONT of the queue — a transient socket error must not
// silently drop results the agent is waiting on.
func TestFlushBatchQueueRequeuesUndeliveredBatch(t *testing.T) {
	srvConn, _ := newBeaconConnPair(t)
	s := &Server{}
	beacon := newTestBeacon("a1")
	beacon.Conn = srvConn
	// Closed connection: writes fail deterministically instead of racing the
	// network stack.
	if err := srvConn.Close(); err != nil {
		t.Fatalf("close server conn: %v", err)
	}

	payload := []byte(`{"type":"task_output","data":"important"}`)
	s.enqueueBatchMessage(beacon, payload)
	beacon.BatchMutex.Lock()
	if beacon.BatchTimer != nil {
		beacon.BatchTimer.Stop()
	}
	beacon.BatchMutex.Unlock()

	s.flushBatchQueue(beacon)

	beacon.BatchMutex.Lock()
	defer beacon.BatchMutex.Unlock()
	if len(beacon.BatchQueue) != 1 {
		t.Fatalf("queue holds %d messages after a failed write, want the batch retained", len(beacon.BatchQueue))
	}
	if string(beacon.BatchQueue[0]) != string(payload) {
		t.Fatalf("retained %s, want the undelivered batch", beacon.BatchQueue[0])
	}
}

// TestRequeueBatchPrependsPreservingOrder proves retries go to the front and
// keep their relative order (the agent blocks on the oldest result).
func TestRequeueBatchPrependsPreservingOrder(t *testing.T) {
	s := &Server{}
	beacon := newTestBeacon("a1")

	s.requeueBatch(beacon, []byte(`first`))
	s.requeueBatch(beacon, []byte(`second`))

	beacon.BatchMutex.Lock()
	defer beacon.BatchMutex.Unlock()
	if len(beacon.BatchQueue) != 2 {
		t.Fatalf("queue holds %d messages, want 2", len(beacon.BatchQueue))
	}
	if string(beacon.BatchQueue[0]) != "second" || string(beacon.BatchQueue[1]) != "first" {
		t.Fatalf("queue = %q, want the newest retry at the front", beacon.BatchQueue)
	}
}

// TestRequeueBatchRetainsOversizedSinglePayload proves an undeliverable frame
// that alone exceeds the retention cap is still retained: dropping it would
// leave the agent waiting forever for a result that never arrives. The cap
// applies to the *backlog*, so the backlog entry is dropped instead.
func TestRequeueBatchRetainsOversizedSinglePayload(t *testing.T) {
	s := &Server{}
	beacon := newTestBeacon("a1")
	beacon.BatchQueue = [][]byte{[]byte(`backlog`)}

	oversize := bytes.Repeat([]byte("Z"), 9*1024*1024) // > 8 MiB retention cap
	s.requeueBatch(beacon, oversize)

	beacon.BatchMutex.Lock()
	defer beacon.BatchMutex.Unlock()
	if len(beacon.BatchQueue) != 1 {
		t.Fatalf("queue holds %d entries, want only the oversize payload", len(beacon.BatchQueue))
	}
	if !bytes.Equal(beacon.BatchQueue[0], oversize) {
		t.Fatalf("retained %d bytes, want the full %d-byte payload", len(beacon.BatchQueue[0]), len(oversize))
	}
}

// TestRequeueBatchTrimsBacklogToRetentionCap proves the bounded-retry contract:
// an oversize batch is trimmed from the front until the retained backlog fits
// under the cap, so a stuck socket cannot grow the queue without limit.
func TestRequeueBatchTrimsBacklogToRetentionCap(t *testing.T) {
	const (
		msgSize    = 1 << 20 // 1 MiB per message
		msgCount   = 12      // 12 MiB total, over the 8 MiB cap
		retention  = 8 << 20
		wantSuffix = true // the oldest messages are dropped, the newest survive
	)
	envelopes := make([]json.RawMessage, msgCount)
	for i := range envelopes {
		body := strings.Repeat(string(rune('a'+i)), msgSize)
		envelopes[i] = json.RawMessage(`"` + body + `"`)
	}
	oversize, err := json.Marshal(batchedMessage{Messages: envelopes})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(oversize) <= retention {
		t.Fatalf("fixture is only %d bytes, needs to exceed the %d-byte cap", len(oversize), retention)
	}

	s := &Server{}
	beacon := newTestBeacon("a1")
	s.requeueBatch(beacon, oversize)

	beacon.BatchMutex.Lock()
	defer beacon.BatchMutex.Unlock()
	if len(beacon.BatchQueue) == 0 {
		t.Fatal("trimming dropped the whole batch")
	}
	total := 0
	for _, b := range beacon.BatchQueue {
		total += len(b)
	}
	if total > retention {
		t.Fatalf("retained %d bytes, over the %d-byte cap", total, retention)
	}
	if wantSuffix {
		// The trim drops from the front, so the last message (index 11) must
		// survive in some retained payload.
		joined := ""
		for _, b := range beacon.BatchQueue {
			joined += string(b)
		}
		if !strings.Contains(joined, strings.Repeat(string(rune('a'+msgCount-1)), 64)) {
			t.Fatal("trimming dropped the newest message instead of the oldest")
		}
		if strings.Contains(joined, strings.Repeat(string(rune('a')), 64)) {
			t.Fatal("trimming kept the oldest message")
		}
	}
}
