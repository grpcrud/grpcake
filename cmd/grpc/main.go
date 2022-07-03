package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ucarion/cli"
	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type args struct {
	Target                   string   `cli:"target"`
	Method                   string   `cli:"method"`
	Long                     bool     `cli:"-l,--long" usage:"if listing methods, output in long format"`
	Protoset                 []string `cli:"--protoset" value:"file" usage:"get schema from .protoset file(s); can be provided multiple times"`
	SchemaFrom               string   `cli:"--schema-from" value:"protoset|reflection" usage:"where to get schema from; default is to choose based on provided flags"`
	UserAgent                string   `cli:"-A,--user-agent"`
	Header                   []string `cli:"-H,--header"`
	HeaderRawKey             []string `cli:"--header-raw-key"`
	HeaderRawValue           []string `cli:"--header-raw-value"`
	ReflectHeader            []string `cli:"--reflect-header"`
	ReflectHeaderRawKey      []string `cli:"--reflect-header-raw-key"`
	ReflectHeaderRawValue    []string `cli:"--reflect-header-raw-value"`
	RPCHeader                []string `cli:"--rpc-header"`
	RPCHeaderRawKey          []string `cli:"--rpc-header-raw-key"`
	RPCHeaderRawValue        []string `cli:"--rpc-header-raw-value"`
	DumpHeader               bool     `cli:"--dump-header"`
	DumpTrailer              bool     `cli:"--dump-trailer"`
	Insecure                 bool     `cli:"-k,--insecure" usage:"disable TLS; default is to validate TLS if target is not a localhost shorthand"`
	InsecureSkipServerVerify bool     `cli:"--insecure-skip-server-verify"`
	ServerRootCA             []string `cli:"--server-root-ca"`
	ServerName               string   `cli:"--server-name"`
	ClientCert               []string `cli:"--client-cert"`
	ClientKey                []string `cli:"--client-key"`
	NoWarnStdinTTY           bool     `cli:"--no-warn-stdin-tty"`
}

func (args args) Autocomplete_Method() []string {
	args.populateDefaults()
	ctx, _, err := args.metadataContexts(context.Background())
	if err != nil {
		return nil
	}

	cc, err := dial(args)
	if args.SchemaFrom == "protoreflect" && err != nil {
		// we only need cc if we're using reflection
		return nil
	}

	msrc, err := args.methodSource(ctx, cc)
	defer msrc.Close()

	methods, err := msrc.Methods()
	if err != nil {
		return nil
	}

	out := []string{"ls", "ll"}
	for _, m := range methods {
		out = append(out, string(m.FullName()))
	}

	return out
}

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		args.populateDefaults()

		cc, err := dial(args)
		if err != nil {
			return err
		}

		ctxReflect, ctxRPC, err := args.metadataContexts(ctx)
		if err != nil {
			return err
		}

		msrc, err := args.methodSource(ctxReflect, cc)
		defer msrc.Close()

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

		return invokeMethod(ctxRPC, cc, msrc, args)
	})
}

func (args *args) populateDefaults() {
	if args.UserAgent == "" {
		args.UserAgent = fmt.Sprintf("grpcake/%s", version)
	}

	if args.SchemaFrom == "" {
		switch {
		case len(args.Protoset) != 0:
			args.SchemaFrom = "protoset"
		default:
			args.SchemaFrom = "reflection"
		}
	}
}

// metadataContexts returns contexts to be used for reflection and RPC calls.
func (args args) metadataContexts(ctx context.Context) (context.Context, context.Context, error) {
	md, err := parseHeaders(args.Header, args.HeaderRawKey, args.HeaderRawValue)
	if err != nil {
		return nil, nil, fmt.Errorf("--header/--header-raw-key/--header-raw-value: %w", err)
	}

	reflectMD, err := parseHeaders(args.ReflectHeader, args.ReflectHeaderRawKey, args.ReflectHeaderRawValue)
	if err != nil {
		return nil, nil, fmt.Errorf("--reflect-header/--reflect-header-raw-key/--reflect-header-raw-value: %w", err)
	}

	rpcMD, err := parseHeaders(args.RPCHeader, args.RPCHeaderRawKey, args.RPCHeaderRawValue)
	if err != nil {
		return nil, nil, fmt.Errorf("--rpc-header/--rpc-header-raw-key/--rpc-header-raw-value: %w", err)
	}

	ctx = metadata.AppendToOutgoingContext(ctx, md...)
	return metadata.AppendToOutgoingContext(ctx, reflectMD...), metadata.AppendToOutgoingContext(ctx, rpcMD...), nil
}

func (args args) methodSource(ctx context.Context, cc *grpc.ClientConn) (methodSource, error) {
	switch args.SchemaFrom {
	case "protoset":
		return newProtosetMethodSource(args.Protoset)
	case "reflection":
		return newReflectMethodSource(ctx, cc)
	default:
		return nil, fmt.Errorf("invalid --schema-from: %s", args.SchemaFrom)
	}
}
