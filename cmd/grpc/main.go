package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ucarion/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	cli.Run(context.Background(), call)

	// cc, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	// if err != nil {
	// 	panic(err)
	// }
	//
	// reflectClient := grpc_reflection_v1alpha.NewServerReflectionClient(cc)
	// reflectInfoClient, err := reflectClient.ServerReflectionInfo(context.Background())
	// if err != nil {
	// 	panic(err)
	// }
	//
	// if err := reflectInfoClient.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
	// 	// Host:           "",
	// 	MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
	// 		FileContainingSymbol: "echo.Echo",
	// 	},
	// }); err != nil {
	// 	panic(err)
	// }
	//
	// recv, err := reflectInfoClient.Recv()
	// if err != nil {
	// 	panic(err)
	// }
	//
	// res := recv.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse)
	//
	// var fd descriptorpb.FileDescriptorProto
	// if err := proto.Unmarshal(res.FileDescriptorResponse.FileDescriptorProto[0], &fd); err != nil {
	// 	panic(err)
	// }
	//
	// for _, s := range fd.Service {
	// 	for _, m := range s.Method {
	// 		fmt.Println(*m.Name)
	// 	}
	// }

	// if err := reflectInfoClient.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
	// 	// Host:           "",
	// 	MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{},
	// }); err != nil {
	// 	panic(err)
	// }
	//
	// recv, err := reflectInfoClient.Recv()
	// if err != nil {
	// 	panic(err)
	// }
	//
	// res := recv.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	// for _, s := range res.ListServicesResponse.Service {
	// 	fmt.Println(s.Name)
	// }
}

type callArgs struct {
	Target string `cli:"target"`
	Method string `cli:"method"`
	Long   bool   `cli:"-l,--long" usage:"if method is 'ls', output methods in long format"`
}

func call(ctx context.Context, args callArgs) error {
	if args.Method == "ls" {
		return listMethods(ctx, args)
	}

	cc, err := grpc.Dial(args.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	client, err := grpc_reflection_v1alpha.NewServerReflectionClient(cc).ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("start reflection info client: %w", err)
	}

	method, err := getMethod(ctx, client, args.Method)
	if err != nil {
		return err
	}

	if !method.IsStreamingClient() && !method.IsStreamingServer() {
		if err := unaryInvoke(ctx, cc, method); err != nil {
			return err
		}
	} else {
		streamDesc := grpc.StreamDesc{
			// StreamName:    "BidiStreamEcho",
			// Handler: func(srv interface{}, stream grpc.ServerStream) error {
			//
			// },
			ServerStreams: method.IsStreamingServer(),
			ClientStreams: method.IsStreamingClient(),
		}

		stream, err := cc.NewStream(ctx, &streamDesc, methodInvokeName(string(method.FullName())))
		if err != nil {
			return err
		}

		scan := bufio.NewScanner(os.Stdin)
		for scan.Scan() {
			msgIn := dynamicpb.NewMessage(method.Input())
			if err := protojson.Unmarshal(scan.Bytes(), msgIn); err != nil {
				return err
			}

			if err := stream.SendMsg(msgIn); err != nil {
				return err
			}
		}

		if err := stream.CloseSend(); err != nil {
			return err
		}

		for {
			msgOut := dynamicpb.NewMessage(method.Output())
			if err := stream.RecvMsg(msgOut); err != nil {
				if err == io.EOF {
					break
				}

				return err
			}

			outputBytes, err := protojson.Marshal(msgOut)
			if err != nil {
				return err
			}

			fmt.Println(string(outputBytes))
		}
	}

	return nil

	// scan := bufio.NewScanner(os.Stdin)
	//
	// if method.IsStreamingClient() {
	// 	for scan.Scan() {
	// 		inputBytes := scan.Bytes()
	//
	// 		input := dynamicpb.NewMessage(method.Input())
	// 		if err := protojson.Unmarshal(inputBytes, input); err != nil {
	// 			return err
	// 		}
	//
	// 		output := dynamicpb.NewMessage(method.Output())
	// 		if err := cc.Invoke(ctx, methodInvokeName(args.Method), input, output); err != nil {
	// 			return err
	// 		}
	//
	// 		outputBytes, err := protojson.Marshal(output)
	// 		if err != nil {
	// 			return err
	// 		}
	//
	// 		fmt.Println(string(outputBytes))
	// 	}
	// } else {
	// 	scan.Scan()
	// 	inputBytes := scan.Bytes()
	//
	// 	input := dynamicpb.NewMessage(method.Input())
	// 	if err := protojson.Unmarshal(inputBytes, input); err != nil {
	// 		return err
	// 	}
	//
	// 	output := dynamicpb.NewMessage(method.Output())
	// 	if err := cc.Invoke(ctx, methodInvokeName(args.Method), input, output); err != nil {
	// 		return err
	// 	}
	//
	// 	outputBytes, err := protojson.Marshal(output)
	// 	if err != nil {
	// 		return err
	// 	}
	//
	// 	fmt.Println(string(outputBytes))
	// }
}

func listMethods(ctx context.Context, args callArgs) error {
	cc, err := grpc.Dial(args.Target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	client, err := grpc_reflection_v1alpha.NewServerReflectionClient(cc).ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("start reflection info client: %w", err)
	}

	if err := client.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		// todo host
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_ListServices{},
	}); err != nil {
		return fmt.Errorf("send ListServices: %w", err)
	}

	res, err := client.Recv()
	if err != nil {
		return fmt.Errorf("recv ListServices: %w", err)
	}

	var svcs []string
	var fds descriptorpb.FileDescriptorSet
	listSvcRes := res.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_ListServicesResponse)
	for _, svc := range listSvcRes.ListServicesResponse.Service {
		svcs = append(svcs, svc.Name)
		files, err := getFilesForSymbol(ctx, client, svc.Name)
		if err != nil {
			return err
		}

		for _, f := range files {
			var fd descriptorpb.FileDescriptorProto
			if err := proto.Unmarshal(f, &fd); err != nil {
				return err
			}

			fds.File = append(fds.File, &fd)
		}
	}

	reg, err := protodesc.NewFiles(&fds)
	if err != nil {
		return err
	}

	for _, svc := range svcs {
		d, err := reg.FindDescriptorByName(protoreflect.FullName(svc))
		if err != nil {
			return err
		}

		methods := d.(protoreflect.ServiceDescriptor).Methods()
		for i, l := 0, methods.Len(); i < l; i++ {
			m := methods.Get(i)
			if args.Long {
				var streamClient string
				if m.IsStreamingClient() {
					streamClient = "stream "
				}

				var streamServer string
				if m.IsStreamingServer() {
					streamServer = "stream "
				}

				fmt.Printf("rpc %s(%s%s) returns (%s%s)\n", m.FullName(), streamClient, m.Input().FullName(), streamServer, m.Output().FullName())
			} else {
				fmt.Println(m.FullName())
			}
		}
	}

	return nil
}

func getFilesForSymbol(ctx context.Context, client grpc_reflection_v1alpha.ServerReflection_ServerReflectionInfoClient, symbol string) ([][]byte, error) {
	if err := client.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
		MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	}); err != nil {
		return nil, fmt.Errorf("send FileContainingSymbol: %w", err)
	}

	res, err := client.Recv()
	if err != nil {
		return nil, err
	}

	return res.MessageResponse.(*grpc_reflection_v1alpha.ServerReflectionResponse_FileDescriptorResponse).FileDescriptorResponse.FileDescriptorProto, nil
}

func getMethod(ctx context.Context, client grpc_reflection_v1alpha.ServerReflection_ServerReflectionInfoClient, name string) (protoreflect.MethodDescriptor, error) {
	files, err := getFilesForSymbol(ctx, client, name)
	if err != nil {
		return nil, err
	}

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

	d, err := reg.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, err
	}

	return d.(protoreflect.MethodDescriptor), nil
}

func methodInvokeName(name string) string {
	i := strings.LastIndexByte(name, '.')
	return "/" + name[:i] + "/" + name[i+1:]
}

func unaryInvoke(ctx context.Context, cc *grpc.ClientConn, method protoreflect.MethodDescriptor) error {
	in, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	msgIn := dynamicpb.NewMessage(method.Input())
	if err := protojson.Unmarshal(in, msgIn); err != nil {
		return err
	}

	msgOut := dynamicpb.NewMessage(method.Output())
	if err := cc.Invoke(ctx, methodInvokeName(string(method.FullName())), msgIn, msgOut); err != nil {
		return err
	}

	outputBytes, err := protojson.Marshal(msgOut)
	if err != nil {
		return err
	}

	fmt.Println(string(outputBytes))
	return nil
}

// func sendMessages(ctx context.Context, cc *grpc.ClientConn, method protoreflect.MethodDescriptor) error {
// 	name := methodInvokeName(string(method.FullName()))
//
// 	if method.IsStreamingClient() {
// 		scanner := bufio.NewScanner(os.Stdin)
// 	}
//
// 	in, err := ioutil.ReadAll(os.Stdin)
// 	if err != nil {
// 		return err
// 	}
//
// 	msg := dynamicpb.NewMessage(method.Input())
// 	if err := protojson.Unmarshal(in, msg); err != nil {
// 		return err
// 	}
//
// 	if err := cc.Invoke(ctx, name, msg); err != nil {
// 		return err
// 	}
//
// 	return nil
// }
