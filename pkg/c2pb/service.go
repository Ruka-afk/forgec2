package c2pb

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

// Package c2pb defines the gRPC beacon transport shared by agent and server.
//
// The transport relays opaque v2 beacon envelopes (see pkg/protocol): a
// check-in is exactly one envelope in, one envelope out, both serialized as
// JSON via the registered "json" codec. Neither side parses the envelope
// content inside the codec — handshake/registration/ciphertext frames keep
// their full field set (mac, secret_id, ...) intact.

// JSONCodec is the shared gRPC wire codec. Both the server (ForceServerCodec)
// and the agent (ForceCodec) must select it explicitly so opaque envelopes
// travel as JSON instead of the default protobuf codec.
var JSONCodec = jsonCodec{}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

const (
	ServiceName = "c2.C2Service"
	BeaconRPC   = "Beacon"
)

// Envelope carries one opaque beacon envelope (raw JSON bytes) per side.
type Envelope struct {
	Payload []byte `json:"payload"`
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return "json" }

// grpc.ServiceDesc for the bidirectional streaming Beacon RPC. Streaming
// mirrors the HTTP transport's one-request/one-response semantics but keeps
// the stream open so a single connection can carry repeated check-ins.
var C2Service_ServiceDesc = grpc.ServiceDesc{
	ServiceName: ServiceName,
	HandlerType: (*C2ServiceServer)(nil),
	Streams: []grpc.StreamDesc{
		{
			StreamName:    BeaconRPC,
			Handler:       _C2Service_Beacon_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "c2.proto",
}

type C2ServiceServer interface {
	Beacon(C2Service_BeaconServer) error
}

type C2Service_BeaconServer interface {
	Send(*Envelope) error
	Recv() (*Envelope, error)
	grpc.ServerStream
}

type C2ServiceClient interface {
	Beacon(ctx context.Context, opts ...grpc.CallOption) (C2Service_BeaconClient, error)
}

type C2Service_BeaconClient interface {
	Send(*Envelope) error
	Recv() (*Envelope, error)
	CloseAndRecv() (*Envelope, error)
	grpc.ClientStream
}

func RegisterC2ServiceServer(s grpc.ServiceRegistrar, srv C2ServiceServer) {
	s.RegisterService(&C2Service_ServiceDesc, srv)
}

func NewC2ServiceClient(cc grpc.ClientConnInterface) C2ServiceClient {
	return &c2ServiceClient{cc: cc}
}

type c2ServiceClient struct {
	cc grpc.ClientConnInterface
}

func (c *c2ServiceClient) Beacon(ctx context.Context, opts ...grpc.CallOption) (C2Service_BeaconClient, error) {
	stream, err := c.cc.NewStream(ctx, &C2Service_ServiceDesc.Streams[0], "/"+ServiceName+"/"+BeaconRPC, opts...)
	if err != nil {
		return nil, err
	}
	return &beaconClientStream{ClientStream: stream}, nil
}

type beaconClientStream struct {
	grpc.ClientStream
}

func (s *beaconClientStream) Send(env *Envelope) error {
	return s.ClientStream.SendMsg(env)
}

func (s *beaconClientStream) Recv() (*Envelope, error) {
	m := new(Envelope)
	if err := s.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *beaconClientStream) CloseAndRecv() (*Envelope, error) {
	if err := s.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Envelope)
	if err := s.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type beaconServerStream struct {
	grpc.ServerStream
}

func (s *beaconServerStream) Send(env *Envelope) error {
	return s.ServerStream.SendMsg(env)
}

func (s *beaconServerStream) Recv() (*Envelope, error) {
	m := new(Envelope)
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _C2Service_Beacon_Handler(srv any, stream grpc.ServerStream) error {
	return srv.(C2ServiceServer).Beacon(&beaconServerStream{ServerStream: stream})
}

// UnimplementedC2ServiceServer provides a compile-time guard for future
// service versions. It is not registered by default.
type UnimplementedC2ServiceServer struct{}

func (UnimplementedC2ServiceServer) Beacon(C2Service_BeaconServer) error {
	return status.Error(codes.Unimplemented, "method Beacon not implemented")
}
func (UnimplementedC2ServiceServer) mustEmbedUnimplementedC2ServiceServer() {}

var _ C2ServiceServer = (*UnimplementedC2ServiceServer)(nil)