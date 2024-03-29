// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.21.1
// source: conductor.proto

package packet

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ConductorClient is the client API for Conductor service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ConductorClient interface {
	Sync(ctx context.Context, opts ...grpc.CallOption) (Conductor_SyncClient, error)
	Infor(ctx context.Context, opts ...grpc.CallOption) (Conductor_InforClient, error)
	Logger(ctx context.Context, opts ...grpc.CallOption) (Conductor_LoggerClient, error)
}

type conductorClient struct {
	cc grpc.ClientConnInterface
}

func NewConductorClient(cc grpc.ClientConnInterface) ConductorClient {
	return &conductorClient{cc}
}

func (c *conductorClient) Sync(ctx context.Context, opts ...grpc.CallOption) (Conductor_SyncClient, error) {
	stream, err := c.cc.NewStream(ctx, &Conductor_ServiceDesc.Streams[0], "/protobuf.Conductor/sync", opts...)
	if err != nil {
		return nil, err
	}
	x := &conductorSyncClient{stream}
	return x, nil
}

type Conductor_SyncClient interface {
	Send(*WorkerSessions) error
	Recv() (*WorkerSessions, error)
	grpc.ClientStream
}

type conductorSyncClient struct {
	grpc.ClientStream
}

func (x *conductorSyncClient) Send(m *WorkerSessions) error {
	return x.ClientStream.SendMsg(m)
}

func (x *conductorSyncClient) Recv() (*WorkerSessions, error) {
	m := new(WorkerSessions)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *conductorClient) Infor(ctx context.Context, opts ...grpc.CallOption) (Conductor_InforClient, error) {
	stream, err := c.cc.NewStream(ctx, &Conductor_ServiceDesc.Streams[1], "/protobuf.Conductor/infor", opts...)
	if err != nil {
		return nil, err
	}
	x := &conductorInforClient{stream}
	return x, nil
}

type Conductor_InforClient interface {
	Send(*WorkerInfor) error
	CloseAndRecv() (*Closer, error)
	grpc.ClientStream
}

type conductorInforClient struct {
	grpc.ClientStream
}

func (x *conductorInforClient) Send(m *WorkerInfor) error {
	return x.ClientStream.SendMsg(m)
}

func (x *conductorInforClient) CloseAndRecv() (*Closer, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Closer)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *conductorClient) Logger(ctx context.Context, opts ...grpc.CallOption) (Conductor_LoggerClient, error) {
	stream, err := c.cc.NewStream(ctx, &Conductor_ServiceDesc.Streams[2], "/protobuf.Conductor/logger", opts...)
	if err != nil {
		return nil, err
	}
	x := &conductorLoggerClient{stream}
	return x, nil
}

type Conductor_LoggerClient interface {
	Send(*WorkerLog) error
	CloseAndRecv() (*Closer, error)
	grpc.ClientStream
}

type conductorLoggerClient struct {
	grpc.ClientStream
}

func (x *conductorLoggerClient) Send(m *WorkerLog) error {
	return x.ClientStream.SendMsg(m)
}

func (x *conductorLoggerClient) CloseAndRecv() (*Closer, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Closer)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ConductorServer is the server API for Conductor service.
// All implementations must embed UnimplementedConductorServer
// for forward compatibility
type ConductorServer interface {
	Sync(Conductor_SyncServer) error
	Infor(Conductor_InforServer) error
	Logger(Conductor_LoggerServer) error
	mustEmbedUnimplementedConductorServer()
}

// UnimplementedConductorServer must be embedded to have forward compatible implementations.
type UnimplementedConductorServer struct {
}

func (UnimplementedConductorServer) Sync(Conductor_SyncServer) error {
	return status.Errorf(codes.Unimplemented, "method Sync not implemented")
}
func (UnimplementedConductorServer) Infor(Conductor_InforServer) error {
	return status.Errorf(codes.Unimplemented, "method Infor not implemented")
}
func (UnimplementedConductorServer) Logger(Conductor_LoggerServer) error {
	return status.Errorf(codes.Unimplemented, "method Logger not implemented")
}
func (UnimplementedConductorServer) mustEmbedUnimplementedConductorServer() {}

// UnsafeConductorServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConductorServer will
// result in compilation errors.
type UnsafeConductorServer interface {
	mustEmbedUnimplementedConductorServer()
}

func RegisterConductorServer(s grpc.ServiceRegistrar, srv ConductorServer) {
	s.RegisterService(&Conductor_ServiceDesc, srv)
}

func _Conductor_Sync_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ConductorServer).Sync(&conductorSyncServer{stream})
}

type Conductor_SyncServer interface {
	Send(*WorkerSessions) error
	Recv() (*WorkerSessions, error)
	grpc.ServerStream
}

type conductorSyncServer struct {
	grpc.ServerStream
}

func (x *conductorSyncServer) Send(m *WorkerSessions) error {
	return x.ServerStream.SendMsg(m)
}

func (x *conductorSyncServer) Recv() (*WorkerSessions, error) {
	m := new(WorkerSessions)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Conductor_Infor_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ConductorServer).Infor(&conductorInforServer{stream})
}

type Conductor_InforServer interface {
	SendAndClose(*Closer) error
	Recv() (*WorkerInfor, error)
	grpc.ServerStream
}

type conductorInforServer struct {
	grpc.ServerStream
}

func (x *conductorInforServer) SendAndClose(m *Closer) error {
	return x.ServerStream.SendMsg(m)
}

func (x *conductorInforServer) Recv() (*WorkerInfor, error) {
	m := new(WorkerInfor)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _Conductor_Logger_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(ConductorServer).Logger(&conductorLoggerServer{stream})
}

type Conductor_LoggerServer interface {
	SendAndClose(*Closer) error
	Recv() (*WorkerLog, error)
	grpc.ServerStream
}

type conductorLoggerServer struct {
	grpc.ServerStream
}

func (x *conductorLoggerServer) SendAndClose(m *Closer) error {
	return x.ServerStream.SendMsg(m)
}

func (x *conductorLoggerServer) Recv() (*WorkerLog, error) {
	m := new(WorkerLog)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Conductor_ServiceDesc is the grpc.ServiceDesc for Conductor service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Conductor_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "protobuf.Conductor",
	HandlerType: (*ConductorServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "sync",
			Handler:       _Conductor_Sync_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "infor",
			Handler:       _Conductor_Infor_Handler,
			ClientStreams: true,
		},
		{
			StreamName:    "logger",
			Handler:       _Conductor_Logger_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "conductor.proto",
}
