package c2pb

import (
	"context"
	"encoding/json"

	"github.com/forgec2/forgec2/pkg/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/status"
)

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

const (
	ServiceName = "c2.C2Service"
	BeaconRPC   = "Beacon"
)

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                       { return "json" }

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
	Send(*protocol.BeaconResponse) error
	Recv() (*protocol.BeaconRequest, error)
	grpc.ServerStream
}

type C2ServiceClient interface {
	Beacon(ctx context.Context, opts ...grpc.CallOption) (C2Service_BeaconClient, error)
}

type C2Service_BeaconClient interface {
	Send(*protocol.BeaconRequest) error
	Recv() (*protocol.BeaconResponse, error)
	CloseAndRecv() (*protocol.BeaconResponse, error)
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
	stream, err := c.cc.NewStream(ctx, &C2Service_ServiceDesc.Streams[0], ServiceName+"/"+BeaconRPC, opts...)
	if err != nil {
		return nil, err
	}
	return &beaconClientStream{ClientStream: stream}, nil
}

type beaconClientStream struct {
	grpc.ClientStream
}

func (s *beaconClientStream) Send(req *protocol.BeaconRequest) error {
	return s.ClientStream.SendMsg(req)
}

func (s *beaconClientStream) Recv() (*protocol.BeaconResponse, error) {
	m := new(protocol.BeaconResponse)
	if err := s.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *beaconClientStream) CloseAndRecv() (*protocol.BeaconResponse, error) {
	if err := s.CloseSend(); err != nil {
		return nil, err
	}
	m := new(protocol.BeaconResponse)
	if err := s.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type beaconServerStream struct {
	grpc.ServerStream
}

func (s *beaconServerStream) Send(resp *protocol.BeaconResponse) error {
	return s.ServerStream.SendMsg(resp)
}

func (s *beaconServerStream) Recv() (*protocol.BeaconRequest, error) {
	m := new(protocol.BeaconRequest)
	if err := s.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _C2Service_Beacon_Handler(srv any, stream grpc.ServerStream) error {
	return srv.(C2ServiceServer).Beacon(&beaconServerStream{ServerStream: stream})
}

type UnimplementedC2ServiceServer struct{}

func (UnimplementedC2ServiceServer) Beacon(C2Service_BeaconServer) error {
	return status.Errorf(codes.Unimplemented, "method Beacon not implemented")
}
func (UnimplementedC2ServiceServer) mustEmbedUnimplementedC2ServiceServer() {}

var _ C2ServiceServer = (*UnimplementedC2ServiceServer)(nil)
