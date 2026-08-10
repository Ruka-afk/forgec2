package server

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/forgec2/forgec2/pkg/c2pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func testFreeAddr(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := lis.Addr().String()
	lis.Close()
	return addr
}

// testGRPCDial connects with the same codec requirements as the agent client.
func testGRPCDial(t *testing.T, addr string) (*grpc.ClientConn, c2pb.C2ServiceClient) {
	t.Helper()
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(c2pb.JSONCodec)),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, c2pb.NewC2ServiceClient(conn)
}

// TestGRPCBeaconEnvelopeRoundTrip proves the agent and server speak the same
// transport: registers the server-side c2pb service, dials it with the
// agent-side client (pkg/c2pb, JSON codec), and exchanges an opaque envelope.
func TestGRPCBeaconEnvelopeRoundTrip(t *testing.T) {
	addr := testFreeAddr(t)

	srv := NewGRPCListener(addr)
	srv.SetHandler(func(agentID string, reqJSON []byte) []byte {
		t.Logf("handler called: agent=%s payload=%d bytes", agentID, len(reqJSON))
		return []byte(`{"tasks":[],"seq":1}`)
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start grpc listener: %v", err)
	}
	defer srv.Stop()

	_, cli := testGRPCDial(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := cli.Beacon(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	envelope := `{"uuid":"test-agent-1","seq":5,"ts":1700000000,"ecdh_pub":"aGFuZHNoYWtl","c":"Y2lwaGVydGV4dA=="}`
	if err := stream.Send(&c2pb.Envelope{Payload: []byte(envelope)}); err != nil {
		t.Fatalf("send: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if string(resp.Payload) != `{"tasks":[],"seq":1}` {
		t.Fatalf("unexpected response: %s", string(resp.Payload))
	}
}

// TestGRPCBeaconRejectedFrame ensures a rejected frame (nil response from the
// handler) terminates the stream without a response.
func TestGRPCBeaconRejectedFrame(t *testing.T) {
	addr := testFreeAddr(t)

	srv := NewGRPCListener(addr)
	srv.SetHandler(func(agentID string, reqJSON []byte) []byte {
		return nil // reject
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("start grpc listener: %v", err)
	}
	defer srv.Stop()

	_, cli := testGRPCDial(t, addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := cli.Beacon(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	if err := stream.Send(&c2pb.Envelope{Payload: []byte(`{"uuid":"rejected"}`)}); err != nil {
		t.Fatalf("send: %v", err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("expected stream to end without a response")
	}
}