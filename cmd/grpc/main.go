package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ucarion/cli"
	"golang.org/x/term"
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

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		cc, err := dial(args)
		if err != nil {
			return err
		}

		if args.SchemaFrom == "" {
			switch {
			case len(args.Protoset) != 0:
				args.SchemaFrom = "protoset"
			default:
				args.SchemaFrom = "reflection"
			}
		}

		// always parse all header params, even if some are ignored, so the user
		// gets validation errors early
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

		// ctx shared between rpc and reflection
		ctx = metadata.AppendToOutgoingContext(ctx, md...)

		var msrc methodSource
		switch args.SchemaFrom {
		case "protoset":
			msrc, err = newProtosetMethodSource(args.Protoset)
			if err != nil {
				return err
			}
		case "reflection":
			// ctx only used for reflection
			ctx := metadata.AppendToOutgoingContext(ctx, reflectMD...)
			msrc, err = newReflectMethodSource(ctx, cc)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid --schema-from: %s", args.SchemaFrom)
		}

		defer msrc.Close() // ignore this error as long as list/invoke works

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

		// ctx only used for rpc
		ctx = metadata.AppendToOutgoingContext(ctx, rpcMD...)
		return invokeMethod(ctx, cc, msrc, args)
	})
}
