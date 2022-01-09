package main

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type methodSource interface {
	Methods() ([]protoreflect.MethodDescriptor, error)
	Method(protoreflect.FullName) (protoreflect.MethodDescriptor, error)
	Close() error
}

type reflectMethodSource struct {
	client grpc_reflection_v1alpha.ServerReflection_ServerReflectionInfoClient
}

func newReflectMethodSource(ctx context.Context, cc *grpc.ClientConn, opts ...grpc.CallOption) (reflectMethodSource, error) {
	client, err := grpc_reflection_v1alpha.NewServerReflectionClient(cc).ServerReflectionInfo(ctx, opts...)
	return reflectMethodSource{client: client}, err
}

func (r reflectMethodSource) Methods() ([]protoreflect.MethodDescriptor, error) {
	if err := r.client.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return nil, fmt.Errorf("send ListServices: %w", err)
	}

	res, err := r.client.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv ListServices: %w", err)
	}

	var svcs []string
	var fds descriptorpb.FileDescriptorSet
	listSvcRes := res.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	for _, svc := range listSvcRes.ListServicesResponse.Service {
		svcs = append(svcs, svc.Name)

		if err := r.client.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: svc.Name,
			},
		}); err != nil {
			return nil, fmt.Errorf("send FileContainingSymbol: %w", err)
		}

		res, err := r.client.Recv()
		if err != nil {
			return nil, fmt.Errorf("recv FileContainingSymbol: %w", err)
		}

		fdRes := res.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
		files := fdRes.FileDescriptorResponse.FileDescriptorProto
		for _, f := range files {
			var fd descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(f, &fd); err != nil {
				return nil, fmt.Errorf("unmarshal FileDescriptorProto: %w", err)
			}

			fds.File = append(fds.File, &fd)
		}
	}

	reg, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("create file registry: %w", err)
	}

	var mds []protoreflect.MethodDescriptor
	reg.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		svcs := fd.Services()
		for i, l := 0, svcs.Len(); i < l; i++ {
			methods := svcs.Get(i).Methods()
			for i, l := 0, methods.Len(); i < l; i++ {
				mds = append(mds, methods.Get(i))
			}
		}

		return true
	})

	return mds, nil
}

func (r reflectMethodSource) Method(name protoreflect.FullName) (protoreflect.MethodDescriptor, error) {
	if err := r.client.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: string(name),
		},
	}); err != nil {
		return nil, fmt.Errorf("send FileContainingSymbol: %w", err)
	}

	res, err := r.client.Recv()
	if err != nil {
		return nil, fmt.Errorf("recv FileContainingSymbol: %w", err)
	}

	fdRes := res.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
	files := fdRes.FileDescriptorResponse.FileDescriptorProto
	var fds descriptorpb.FileDescriptorSet
	for _, f := range files {
		var fd descriptorpb.FileDescriptorProto
		if err := proto.Unmarshal(f, &fd); err != nil {
			return nil, err
		}

		fds.File = append(fds.File, &fd)
	}

	reg, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, err
	}

	d, err := reg.FindDescriptorByName(name)
	if err != nil {
		return nil, err
	}

	return d.(protoreflect.MethodDescriptor), nil
}

func (r reflectMethodSource) Close() error {
	return r.client.CloseSend()
}

// ls
// call

// reflection
// protoset

// ls reflection -> list methods, get files for methods, put into reg, get from reg
// ls protoset -> put all files into reg, list all methods in reg

// call reflection -> get file for method, put into reg, get from reg
// call protoset -> put all files into reg, get from reg
