package server

import (
	"context"
	"net"
	"testing"
	"time"
)
func TestListenerHealthTrackerCountsConsecutiveFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}

	first := &listenerHealth{ListenerID: 7, Error: "first failure"}
	tracker.observe(first)
	if first.Failures != 1 {
		t.Fatalf("first failure count = %d, want 1", first.Failures)
	}

	second := &listenerHealth{ListenerID: 7, Error: "second failure"}
	tracker.observe(second)
	if second.Failures != listenerFailThreshold {
		t.Fatalf("second failure count = %d, want %d", second.Failures, listenerFailThreshold)
	}

	recovered := &listenerHealth{ListenerID: 7, OK: true}
	tracker.observe(recovered)
	if recovered.Failures != 0 {
		t.Fatalf("successful probe retained %d failures", recovered.Failures)
	}

	afterRecovery := &listenerHealth{ListenerID: 7, Error: "new sequence"}
	tracker.observe(afterRecovery)
	if afterRecovery.Failures != 1 {
		t.Fatalf("failure after recovery count = %d, want 1", afterRecovery.Failures)
	}
}

func TestListenerHealthTrackerSkippedProbeResetsFailures(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 3, Failures: 9})

	skipped := &listenerHealth{ListenerID: 3, OK: true, Skipped: true, Failures: 9}
	tracker.observe(skipped)
	if skipped.Failures != 0 {
		t.Fatalf("skipped probe retained %d failures", skipped.Failures)
	}
}

func TestListenerHealthTrackerPrunesAndSortsSnapshot(t *testing.T) {
	tracker := &listenerHealthTracker{state: map[uint]*listenerHealth{}}
	tracker.observe(&listenerHealth{ListenerID: 9, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 2, OK: true})
	tracker.observe(&listenerHealth{ListenerID: 5, OK: true})

	tracker.prune(map[uint]struct{}{9: {}, 2: {}})
	snapshot := tracker.snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot length = %d, want 2", len(snapshot))
	}
	if snapshot[0].ListenerID != 2 || snapshot[1].ListenerID != 9 {
		t.Fatalf("snapshot order = [%d, %d], want [2, 9]", snapshot[0].ListenerID, snapshot[1].ListenerID)
	}
}

func TestProbeSSHReadsBanner(t *testing.T) {
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
			_, _ = conn.Write([]byte("SSH-2.0-ForgeC2-test\r\n"))
			conn.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ok, err := probeSSH(ctx, ln.Addr().String()); !ok || err != nil {
		t.Fatalf("probeSSH = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := probeSSH(ctx, "127.0.0.1:1"); ok {
		t.Fatal("probeSSH against closed port must fail")
	}
}

func TestProbeH2CPriorKnowledge(t *testing.T) {
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
			go func(c net.Conn) {
				defer c.Close()
				// Read preface, answer with an empty SETTINGS frame.
				buf := make([]byte, 24+9)
				readFullForTest(c, buf)
				_, _ = c.Write([]byte{0, 0, 0, 0x4, 0, 0, 0, 0, 0})
			}(conn)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ok, err := probeH2C(ctx, ln.Addr().String()); !ok || err != nil {
		t.Fatalf("probeH2C = %v, %v; want true, nil", ok, err)
	}
}

func readFullForTest(conn net.Conn, buf []byte) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for off := 0; off < len(buf); {
		n, err := conn.Read(buf[off:])
		if err != nil || n == 0 {
			return
		}
		off += n
	}
}
