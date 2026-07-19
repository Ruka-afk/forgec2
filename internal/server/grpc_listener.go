package server

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
}

func NewGRPCListener(addr string) *GRPCListener {
	return &GRPCListener{addr: addr}
}

func (l *GRPCListener) SetHandler(h func(agentID string, reqJSON []byte) []byte) {
	l.beaconSrv = &grpcBeaconServer{handler: h}
}

func (l *GRPCListener) Start() error {
	lis, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}

	l.server = grpc.NewServer(grpc.MaxRecvMsgSize(GRPCMaxRecvMsgSize))
	l.server.RegisterService(&grpcServiceDesc, l.beaconSrv)

	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("[PANIC RECOVERED] %v\n%s", r, debug.Stack()) } }()
		slog.Info("gRPC listener started", "addr", l.addr)
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

// startGRPCListener registers the gRPC listener in server startup
func (s *Server) startGRPCListener() {
	addr := s.cfg.Server.GRPCAddr
	listener := NewGRPCListener(addr)
	listener.SetHandler(func(agentID string, reqJSON []byte) []byte {
		var req beaconRequest
		if len(reqJSON) > 0 {
			if err := json.Unmarshal(reqJSON, &req); err != nil {
				slog.Error("gRPC beacon handler unmarshal error", "err", err)
			}
		}
		if req.UUID == "" {
			req.UUID = agentID
		}
		resp := s.processBeacon(req, "")
		respJSON, err := json.Marshal(resp)
		if err != nil {
			slog.Error("gRPC marshal response failed", "error", err)
			return nil
		}
		return respJSON
	})
	if err := listener.Start(); err != nil {
		slog.Error("Failed to start gRPC listener", "addr", addr, "err", err)
	}
	s.grpcListener = listener
	slog.Info("gRPC listener registered", "addr", addr)
}
