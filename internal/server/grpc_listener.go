package server

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime/debug"

	"github.com/forgec2/forgec2/pkg/c2pb"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// GRPCListener serves beacon check-ins over the bidirectional streaming RPC
// defined in pkg/c2pb (JSON codec). Each stream carries opaque v2 beacon
// envelopes — one request envelope in, one response envelope out — so the
// gRPC transport authenticates frames exactly like the HTTP beacon path and
// cannot bypass beacon_key auth or downgrade to plaintext.

type GRPCListener struct {
	addr      string
	server    *grpc.Server
	beaconSrv *grpcBeaconServer
	tlsCreds  credentials.TransportCredentials // optional TLS for grpcs://
}

func NewGRPCListener(addr string) *GRPCListener {
	return &GRPCListener{addr: addr}
}

func (l *GRPCListener) SetHandler(h func(agentID string, reqJSON []byte) []byte) {
	l.beaconSrv = &grpcBeaconServer{handler: h}
}

// SetTLS enables TLS credentials (for grpcs://). Pass nil for plain lab grpc://.
func (l *GRPCListener) SetTLS(creds credentials.TransportCredentials) {
	l.tlsCreds = creds
}

func (l *GRPCListener) Start() error {
	lis, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}

	opts := []grpc.ServerOption{grpc.MaxRecvMsgSize(GRPCMaxRecvMsgSize), grpc.ForceServerCodec(c2pb.JSONCodec)}
	mode := "insecure"
	if l.tlsCreds != nil {
		opts = append(opts, grpc.Creds(l.tlsCreds))
		mode = "tls"
	}
	l.server = grpc.NewServer(opts...)
	c2pb.RegisterC2ServiceServer(l.server, l.beaconSrv)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		slog.Info("gRPC listener started", "addr", l.addr, "mode", mode)
		if err := l.server.Serve(lis); err != nil {
			slog.Error("gRPC server error", "err", err)
		}
	}()
	return nil
}

func (l *GRPCListener) Stop() error {
	if l.server != nil {
		l.server.GracefulStop()
	}
	return nil
}

// Close implements io.Closer for use with extraListeners map.
func (l *GRPCListener) Close() error {
	return l.Stop()
}

// grpcBeaconServer implements c2pb.C2ServiceServer. Each stream carries any
// number of check-in cycles; a cycle is Recv → handler → Send. handler reuses
// the standard listener beacon path (ECDH/AES-256-GCM, seq replay window,
// registration/handshake MACs), so the transport adds no trust.
type grpcBeaconServer struct {
	handler func(agentID string, reqJSON []byte) []byte
}

func (s *grpcBeaconServer) Beacon(stream c2pb.C2Service_BeaconServer) error {
	for {
		env, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Aborted, "recv failed: %v", err)
		}
		if len(env.Payload) == 0 {
			return status.Errorf(codes.InvalidArgument, "empty envelope")
		}

		if p, ok := peer.FromContext(stream.Context()); ok {
			slog.Debug("gRPC beacon", "remote", p.Addr.String(), "payload_size", len(env.Payload))
		}

		// Extract agent ID for logging/failover; the handler authenticates
		// the frame itself (handshake/registration path uses the UUID).
		var header struct {
			UUID string `json:"uuid,omitempty"`
		}
		agentID := ""
		if err := json.Unmarshal(env.Payload, &header); err == nil {
			agentID = header.UUID
		} else {
			agentID = "unknown-" + uuid.New().String()
		}

		respPayload := s.handler(agentID, env.Payload)
		if respPayload == nil {
			// Frame rejected (replay, bad MAC, unknown agent). End this
			// stream cycle without a response so the agent can fail over.
			return nil
		}
		if err := stream.Send(&c2pb.Envelope{Payload: respPayload}); err != nil {
			return status.Errorf(codes.Aborted, "send failed: %v", err)
		}
	}
}

// startGRPCListener registers the gRPC listener in server startup.
// When TLS is enabled and cert/key exist, serves grpcs://; otherwise plain grpc:// (lab).
func (s *Server) startGRPCListener() {
	addr := s.cfg.Server.GRPCAddr
	listener := NewGRPCListener(addr)
	if s.cfg.Server.TLSEnabled && s.cfg.Server.CertFile != "" && s.cfg.Server.KeyFile != "" {
		if cert, err := tls.LoadX509KeyPair(s.cfg.Server.CertFile, s.cfg.Server.KeyFile); err == nil {
			tlsCfg := &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}

			// mTLS: load client CA for mutual TLS verification
			if s.cfg.Server.ClientCAFile != "" && s.cfg.Server.RequireClientCert {
				caCert, caErr := os.ReadFile(s.cfg.Server.ClientCAFile)
				if caErr != nil {
					slog.Warn("gRPC mTLS: failed to load client CA", "err", caErr)
				} else {
					caPool := x509.NewCertPool()
					if !caPool.AppendCertsFromPEM(caCert) {
						slog.Warn("gRPC mTLS: failed to parse client CA")
					} else {
						tlsCfg.ClientCAs = caPool
						tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
						slog.Info("gRPC mTLS enabled", "client_ca", s.cfg.Server.ClientCAFile)
					}
				}
			}

			listener.SetTLS(credentials.NewTLS(tlsCfg))
			slog.Info("gRPC TLS credentials loaded", "cert", s.cfg.Server.CertFile)
		} else {
			slog.Error("gRPC TLS load failed; refusing to start in insecure mode", "err", err)
			return
		}
	}
	listener.SetHandler(s.makeBeaconHandler())
	if err := listener.Start(); err != nil {
		slog.Error("Failed to start gRPC listener", "addr", addr, "err", err)
	} else {
		s.grpcListener = listener
		slog.Info("gRPC listener registered", "addr", addr)
	}
}