package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"runtime/debug"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// ForgeC2 gRPC service — manual registration without protoc.
// Message format: protobuf-encoded BeaconEnvelope wrapping inner payload bytes.

// BeaconEnvelope is the protobuf wrapper for gRPC beacon exchanges.
type BeaconEnvelope struct {
	Payload []byte `protobuf:"bytes,1,opt,name=payload,proto3"`
}

func (m *BeaconEnvelope) Reset()         {}
func (m *BeaconEnvelope) String() string { return string(m.Payload) }
func (m *BeaconEnvelope) ProtoMessage()  {}

const grpcServiceName = "forgec2.BeaconService"

// grpcBeaconServer implements the beacon handler
type grpcBeaconServer struct {
	handler func(agentID string, reqJSON []byte) []byte
}

// proxiedHandler wraps the []byte-returning handler into error-returning for gRPC
func beaconHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	var req BeaconEnvelope
	if err := dec(&req); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "decode failed: %v", err)
	}

	svc := srv.(*grpcBeaconServer)
	if p, ok := peer.FromContext(ctx); ok {
		slog.Debug("gRPC beacon", "remote", p.Addr.String(), "payload_size", len(req.Payload))
	}

	// Parse incoming to extract agent ID
	var envelope struct {
		UUID string `json:"uuid,omitempty"`
	}
	var agentID string
	if err := json.Unmarshal(req.Payload, &envelope); err == nil {
		agentID = envelope.UUID
	} else {
		slog.Warn("gRPC beacon failed to unmarshal envelope, using fallback agentID", "err", err)
		agentID = "unknown-" + uuid.New().String()
	}

	respPayload := svc.handler(agentID, req.Payload)
	return &BeaconEnvelope{Payload: respPayload}, nil
}

func handshakeHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return beaconHandler(srv, ctx, dec, interceptor)
}

var grpcServiceDesc = grpc.ServiceDesc{
	ServiceName: grpcServiceName,
	Methods: []grpc.MethodDesc{
		{MethodName: "Beacon", Handler: beaconHandler},
		{MethodName: "Handshake", Handler: handshakeHandler},
	},
	Metadata: "beacon.proto",
}

// GRPCListener wraps the gRPC server for beacon transport
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

	opts := []grpc.ServerOption{grpc.MaxRecvMsgSize(GRPCMaxRecvMsgSize)}
	mode := "insecure"
	if l.tlsCreds != nil {
		opts = append(opts, grpc.Creds(l.tlsCreds))
		mode = "tls"
	}
	l.server = grpc.NewServer(opts...)
	l.server.RegisterService(&grpcServiceDesc, l.beaconSrv)

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
