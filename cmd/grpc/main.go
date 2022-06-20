package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/ucarion/cli"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

type args struct {
	Target                string   `cli:"target"`
	Method                string   `cli:"method"`
	Long                  bool     `cli:"-l,--long" usage:"if listing methods, output in long format"`
	Protoset              []string `cli:"--protoset" value:"file" usage:"get schema from .protoset file(s); can be provided multiple times"`
	SchemaFrom            string   `cli:"--schema-from" value:"protoset|reflection" usage:"where to get schema from; default is to choose based on provided flags"`
	Header                []string `cli:"-H,--header"`
	HeaderRawKey          []string `cli:"--header-raw-key"`
	HeaderRawValue        []string `cli:"--header-raw-value"`
	ReflectHeader         []string `cli:"--reflect-header"`
	ReflectHeaderRawKey   []string `cli:"--reflect-header-raw-key"`
	ReflectHeaderRawValue []string `cli:"--reflect-header-raw-value"`
	RPCHeader             []string `cli:"--rpc-header"`
	RPCHeaderRawKey       []string `cli:"--rpc-header-raw-key"`
	RPCHeaderRawValue     []string `cli:"--rpc-header-raw-value"`
	DumpHeader            bool     `cli:"--dump-header"`
	DumpTrailer           bool     `cli:"--dump-trailer"`
	Insecure              bool     `cli:"-k,--insecure" usage:"disable TLS; default is to validate TLS if target is not a localhost shorthand"`
	ServerRootCA          []string `cli:"--server-root-ca"`
	ServerName            string   `cli:"--server-name"`
	ClientCert            []string `cli:"--client-cert"`
	ClientKey             []string `cli:"--client-key"`
	NoWarnStdinTTY        bool     `cli:"--no-warn-stdin-tty"`
}

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		var certPool *x509.CertPool
		if len(args.ServerRootCA) > 0 {
			// tls.Config's default is to use system pool, so only override
			// certPool if user provides CAs
			certPool = x509.NewCertPool()
			for _, f := range args.ServerRootCA {
				serverCA, err := ioutil.ReadFile(f)
				if err != nil {
					panic(err)
				}

				if !certPool.AppendCertsFromPEM(serverCA) {
					return fmt.Errorf("could not parse server CA file: %s", f)
				}
			}
		}

		if len(args.ClientCert) != len(args.ClientKey) {
			return fmt.Errorf("--client-cert and --client-key must be passed an equal number of times")
		}

		var certs []tls.Certificate
		for i, c := range args.ClientCert {
			k := args.ClientKey[i]
			cert, err := tls.LoadX509KeyPair(c, k)
			if err != nil {
				return fmt.Errorf("loading client key pair: cert: %s, key: %s: %w", c, k, err)
			}

			certs = append(certs, cert)
		}

		tlsConfig := tls.Config{
			InsecureSkipVerify: false, // todo support skipping server verify?
			RootCAs:            certPool,
			ServerName:         args.ServerName,
			Certificates:       certs,
		}

		target, isShorthand := parseTarget(args.Target)

		var creds credentials.TransportCredentials
		if isShorthand || args.Insecure {
			creds = insecure.NewCredentials()
		} else {
			creds = credentials.NewTLS(&tlsConfig)
		}

		cc, err := grpc.Dial(target, grpc.WithTransportCredentials(creds))
		if err != nil {
			return fmt.Errorf("dial: %w", err)
		}

		if args.SchemaFrom == "" {
			switch {
			case len(args.Protoset) != 0:
				args.SchemaFrom = "protoset"
			default:
				args.SchemaFrom = "reflection"
			}
		}

		md, err := parseHeaders(args.Header, args.HeaderRawKey, args.HeaderRawValue)
		if err != nil {
			return fmt.Errorf("--header/--header-raw-key/--header-raw-value: %w", err)
		}

		reflectMD, err := parseHeaders(args.ReflectHeader, args.ReflectHeaderRawKey, args.ReflectHeaderRawValue)
		if err != nil {
			return fmt.Errorf("--reflect-header/--reflect-header-raw-key/--reflect-header-raw-value: %w", err)
		}

		rpcMD, err := parseHeaders(args.RPCHeader, args.RPCHeaderRawKey, args.RPCHeaderRawValue)
		if err != nil {
			return fmt.Errorf("--rpc-header/--rpc-header-raw-key/--rpc-header-raw-value: %w", err)
		}

		ctx = metadata.AppendToOutgoingContext(ctx, md...)

		var msrc methodSource
		switch args.SchemaFrom {
		case "protoset":
			msrc, err = newProtosetMethodSource(args.Protoset)
		case "reflection":
			ctx = metadata.AppendToOutgoingContext(ctx, reflectMD...)
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

		if !args.NoWarnStdinTTY && term.IsTerminal(int(os.Stdin.Fd())) {
			_, _ = fmt.Fprintln(os.Stderr, "warning: reading message(s) from stdin (disable this message with --no-warn-stdin-tty)")
		}

		ctx = metadata.AppendToOutgoingContext(ctx, rpcMD...)
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
		return unaryInvoke(ctx, cc, method, args)
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

	if args.DumpHeader {
		header, err := stream.Header()
		if err != nil {
			return err
		}

		log, err := json.Marshal(headerTrailer{Header: header})
		if err != nil {
			panic(fmt.Errorf("marshal header/trailer: %w", err))
		}

		_, _ = fmt.Fprintln(os.Stderr, string(log))
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

	if args.DumpTrailer {
		log, err := json.Marshal(headerTrailer{Trailer: stream.Trailer()})
		if err != nil {
			panic(fmt.Errorf("marshal header/trailer: %w", err))
		}

		_, _ = fmt.Fprintln(os.Stderr, string(log))
	}

	return nil
}

type headerTrailer struct {
	Header  metadata.MD `json:"header,omitempty"`
	Trailer metadata.MD `json:"trailer,omitempty"`
}

func unaryInvoke(ctx context.Context, cc *grpc.ClientConn, method protoreflect.MethodDescriptor, args args) error {
	in, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	msgIn := dynamicpb.NewMessage(method.Input())
	if err := protojson.Unmarshal(in, msgIn); err != nil {
		return err
	}

	msgOut := dynamicpb.NewMessage(method.Output())

	var header, trailer metadata.MD
	if err := cc.Invoke(ctx, methodInvokeName(string(method.FullName())), msgIn, msgOut, grpc.Header(&header), grpc.Trailer(&trailer)); err != nil {
		return err
	}

	var headerTrailer headerTrailer
	if args.DumpHeader {
		headerTrailer.Header = header
	}
	if args.DumpTrailer {
		headerTrailer.Trailer = trailer
	}

	if args.DumpHeader || args.DumpTrailer {
		log, err := json.Marshal(headerTrailer)
		if err != nil {
			panic(fmt.Errorf("marshal header/trailer: %w", err))
		}

		_, _ = fmt.Fprintln(os.Stderr, string(log))
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
