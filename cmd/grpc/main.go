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

func (_ *args) ExtendedDescription() string {
	return strings.TrimSpace(`
Call a gRPC method on a server, or list methods available on the server.

"target" must be in the syntax supported by the gRPC name resolution system:

	https://github.com/grpc/grpc/blob/master/doc/naming.md

Examples of valid "target" values include:

	localhost:80
	example.com:443
	dns:example.com:443
	unix:path/to/socket

"method" must be of the syntax "package.service.method", such as:

	routeguide.RouteGuide.GetFeature

The following two method values are special-cased:

	ls
	ll

Passing "ls" for as the "method" will list all methods available on the server.
"ll" is like "ls", but also implies "--long", which produces information on the
input and output types of the methods.

Calling a gRPC method is only possible if you know the schema of that method. To
that end, there are three ways this tool can discover a schema:

1. Using the gRPC reflection API. This is the default, but will not work if the
server does not have the reflection API registered.

2. Using ".protoset" file(s). This is the default if "--protoset" is provided at
least once. This works like protoc's "--descriptor_set_in" option. 

3. Using ".proto" file(s). This is the default if "--proto" (alias: "-I") is
provided at least once. This works like protoc's "-I"/"--proto_path" option.

You can force which of these strategies to choose by passing the "--schema-from"
option.

When using ".protoset" or ".proto" files to infer a schema, it's not possible to
tell what methods the server actually has registered at runtime. Instead, all
methods in the set of protobuf data are listed.

When using ".proto" files to infer a schema, all of the ".proto" files specified
have to be compiled by protoc into a ".protoset" file internally; generating the
".protoset" file yourself may perform better than having gRPCake doing it each
time. You can create ".protoset" files by running:

	protoc --descriptor_set_out xxx.protoset --include_imports ...
`)
}

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		target := parseTarget(args.Target)
		cc, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
