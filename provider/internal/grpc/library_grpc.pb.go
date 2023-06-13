// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.15.8
// source: provider/internal/grpc/library.proto

package grpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ProviderServiceClient is the client API for ProviderService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProviderServiceClient interface {
	Capabilities(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CapabilitiesResponse, error)
	Init(ctx context.Context, in *Config, opts ...grpc.CallOption) (*InitResponse, error)
	Evaluate(ctx context.Context, in *EvaluateRequest, opts ...grpc.CallOption) (*EvaluateResponse, error)
	Stop(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetDependencies(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*DependencyResponse, error)
	GetDependenciesDAG(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*DependencyDAGResponse, error)
}

type providerServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewProviderServiceClient(cc grpc.ClientConnInterface) ProviderServiceClient {
	return &providerServiceClient{cc}
}

func (c *providerServiceClient) Capabilities(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CapabilitiesResponse, error) {
	out := new(CapabilitiesResponse)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/Capabilities", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerServiceClient) Init(ctx context.Context, in *Config, opts ...grpc.CallOption) (*InitResponse, error) {
	out := new(InitResponse)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/Init", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerServiceClient) Evaluate(ctx context.Context, in *EvaluateRequest, opts ...grpc.CallOption) (*EvaluateResponse, error) {
	out := new(EvaluateResponse)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/Evaluate", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerServiceClient) Stop(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/Stop", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerServiceClient) GetDependencies(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*DependencyResponse, error) {
	out := new(DependencyResponse)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/GetDependencies", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *providerServiceClient) GetDependenciesDAG(ctx context.Context, in *ServiceRequest, opts ...grpc.CallOption) (*DependencyDAGResponse, error) {
	out := new(DependencyDAGResponse)
	err := c.cc.Invoke(ctx, "/provider.ProviderService/GetDependenciesDAG", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProviderServiceServer is the server API for ProviderService service.
// All implementations must embed UnimplementedProviderServiceServer
// for forward compatibility
type ProviderServiceServer interface {
	Capabilities(context.Context, *emptypb.Empty) (*CapabilitiesResponse, error)
	Init(context.Context, *Config) (*InitResponse, error)
	Evaluate(context.Context, *EvaluateRequest) (*EvaluateResponse, error)
	Stop(context.Context, *ServiceRequest) (*emptypb.Empty, error)
	GetDependencies(context.Context, *ServiceRequest) (*DependencyResponse, error)
	GetDependenciesDAG(context.Context, *ServiceRequest) (*DependencyDAGResponse, error)
	mustEmbedUnimplementedProviderServiceServer()
}

// UnimplementedProviderServiceServer must be embedded to have forward compatible implementations.
type UnimplementedProviderServiceServer struct {
}

func (UnimplementedProviderServiceServer) Capabilities(context.Context, *emptypb.Empty) (*CapabilitiesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Capabilities not implemented")
}
func (UnimplementedProviderServiceServer) Init(context.Context, *Config) (*InitResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Init not implemented")
}
func (UnimplementedProviderServiceServer) Evaluate(context.Context, *EvaluateRequest) (*EvaluateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Evaluate not implemented")
}
func (UnimplementedProviderServiceServer) Stop(context.Context, *ServiceRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Stop not implemented")
}
func (UnimplementedProviderServiceServer) GetDependencies(context.Context, *ServiceRequest) (*DependencyResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDependencies not implemented")
}
func (UnimplementedProviderServiceServer) GetDependenciesDAG(context.Context, *ServiceRequest) (*DependencyDAGResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDependenciesDAG not implemented")
}
func (UnimplementedProviderServiceServer) mustEmbedUnimplementedProviderServiceServer() {}

// UnsafeProviderServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProviderServiceServer will
// result in compilation errors.
type UnsafeProviderServiceServer interface {
	mustEmbedUnimplementedProviderServiceServer()
}

func RegisterProviderServiceServer(s grpc.ServiceRegistrar, srv ProviderServiceServer) {
	s.RegisterService(&ProviderService_ServiceDesc, srv)
}

func _ProviderService_Capabilities_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).Capabilities(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/Capabilities",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).Capabilities(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderService_Init_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Config)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).Init(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/Init",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).Init(ctx, req.(*Config))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderService_Evaluate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(EvaluateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).Evaluate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/Evaluate",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).Evaluate(ctx, req.(*EvaluateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderService_Stop_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).Stop(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/Stop",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).Stop(ctx, req.(*ServiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderService_GetDependencies_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).GetDependencies(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/GetDependencies",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).GetDependencies(ctx, req.(*ServiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderService_GetDependenciesDAG_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ServiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderServiceServer).GetDependenciesDAG(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/provider.ProviderService/GetDependenciesDAG",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProviderServiceServer).GetDependenciesDAG(ctx, req.(*ServiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ProviderService_ServiceDesc is the grpc.ServiceDesc for ProviderService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ProviderService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "provider.ProviderService",
	HandlerType: (*ProviderServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Capabilities",
			Handler:    _ProviderService_Capabilities_Handler,
		},
		{
			MethodName: "Init",
			Handler:    _ProviderService_Init_Handler,
		},
		{
			MethodName: "Evaluate",
			Handler:    _ProviderService_Evaluate_Handler,
		},
		{
			MethodName: "Stop",
			Handler:    _ProviderService_Stop_Handler,
		},
		{
			MethodName: "GetDependencies",
			Handler:    _ProviderService_GetDependencies_Handler,
		},
		{
			MethodName: "GetDependenciesDAG",
			Handler:    _ProviderService_GetDependenciesDAG_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "provider/internal/grpc/library.proto",
}
