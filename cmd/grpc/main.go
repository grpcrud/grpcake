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
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type args struct {
	Target     string   `cli:"target"`
	Method     string   `cli:"method"`
	Long       bool     `cli:"-l,--long" usage:"if listing methods, output in long format"`
	Protoset   []string `cli:"--protoset" value:"file" usage:"get schema from .protoset file(s); can be provided multiple times"`
	ProtoPath  []string `cli:"-I,--proto-path" value:"path" usage:"get schema from .proto files; can be provided multiple times"`
	SchemaFrom string   `cli:"--schema-from" value:"protoset|proto-path|reflection" usage:"where to get schema from; default is to choose based on provided flags"`
}

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		creds, err := credentials.NewClientTLSFromFile("internal/echoserver/server.crt", "grpcake-test-server.example.com")
		if err != nil {
			return err
		}

		target := parseTarget(args.Target)
		cc, err := grpc.Dial(target, grpc.WithTransportCredentials(creds))
		if err != nil {
			return fmt.Errorf("dial: %w", err)
		}

		if args.SchemaFrom == "" {
			switch {
			case len(args.Protoset) != 0:
				args.SchemaFrom = "protoset"
			case len(args.ProtoPath) != 0:
				args.SchemaFrom = "proto-path"
			default:
				args.SchemaFrom = "reflection"
			}
		}

		var msrc methodSource
		switch args.SchemaFrom {
		case "protoset":
			msrc, err = newProtosetMethodSource(args.Protoset)
		case "proto-path":
			msrc, err = newProtopathMethodSource(ctx, args.ProtoPath)
		case "reflection":
			msrc, err = newReflectMethodSource(ctx, cc)
		default:
			return fmt.Errorf("invalid --schema-from: %s", args.SchemaFrom)
		}

		if err != nil {
			return err
		}

		if args.Method == "ll" {
			args.Method = "ls"
			args.Long = true
		}

		if args.Method == "ls" {
			return listMethods(msrc, args)
		}

		return invokeMethod(ctx, cc, msrc, args)
	})
}

func listMethods(msrc methodSource, args args) error {
	methods, err := msrc.Methods()
	if err != nil {
		return err
	}

	for _, m := range methods {
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

	return nil
}

func invokeMethod(ctx context.Context, cc *grpc.ClientConn, msrc methodSource, args args) error {
	method, err := msrc.Method(protoreflect.FullName(args.Method))
	if err != nil {
		return err
	}

	if !method.IsStreamingClient() && !method.IsStreamingServer() {
		return unaryInvoke(ctx, cc, method)
	}

	streamDesc := grpc.StreamDesc{
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

	return nil
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

func methodInvokeName(name string) string {
	i := strings.LastIndexByte(name, '.')
	return "/" + name[:i] + "/" + name[i+1:]
}
