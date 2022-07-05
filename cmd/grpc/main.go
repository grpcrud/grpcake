package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/ucarion/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type args struct {
	Target                   string   `cli:"target"`
	Method                   string   `cli:"method"`
	Long                     bool     `cli:"-l,--long" usage:"if listing methods, output in long format"`
	Protoset                 []string `cli:"--protoset" value:"file" usage:"get schema from .protoset file(s); can be provided multiple times"`
	UserAgent                string   `cli:"-A,--user-agent" value:"user-agent" usage:"user-agent string to use in all RPCs"`
	Header                   []string `cli:"-H,--header" value:"header" usage:"metadata header key/value pair, of the form 'key: value'"`
	HeaderRawKey             []string `cli:"--header-raw-key" value:"raw-key" usage:"metadata header key; use in pairs with --header-raw-value"`
	HeaderRawValue           []string `cli:"--header-raw-value" value:"raw-value" usage:"metadata header value"`
	ReflectHeader            []string `cli:"--reflect-header" value:"header" usage:"metadata header key/value pair to use only in reflection RPCs, of the form 'key: value'"`
	ReflectHeaderRawKey      []string `cli:"--reflect-header-raw-key" value:"raw-key" usage:"metadata header key to use only in reflection RPCs; use in pairs with --reflect-header-raw-value"`
	ReflectHeaderRawValue    []string `cli:"--reflect-header-raw-value" value:"raw-value" usage:"metadata header value to use only in reflection RPCs"`
	RPCHeader                []string `cli:"--rpc-header" value:"header" usage:"metadata header key/value pair to use only in non-reflection RPCs, of the form 'key: value'"`
	RPCHeaderRawKey          []string `cli:"--rpc-header-raw-key" value:"raw-key" usage:"metadata header key to use only in non-reflection RPCs; use in pairs with --rpc-header-raw-value"`
	RPCHeaderRawValue        []string `cli:"--rpc-header-raw-value" value:"raw-value" usage:"metadata header value to use only in non-reflection RPCs"`
	DumpHeader               bool     `cli:"--dump-header" usage:"dump server metadata headers to stderr"`
	DumpTrailer              bool     `cli:"--dump-trailer" usage:"dump server metadata trailers to stderr"`
	Insecure                 bool     `cli:"-k,--insecure" usage:"disable TLS; default is to validate TLS if target is not a localhost shorthand"`
	InsecureSkipServerVerify bool     `cli:"--insecure-skip-server-verify" usage:"when using TLS, skip verifying the server's certificate chain and host name"`
	ServerRootCA             []string `cli:"--server-root-ca" value:"ca-cert" usage:"server root CA; default is to use system cert pool"`
	ServerName               string   `cli:"--server-name" value:"server-name-override" usage:"override server name for handshake and TLS host name verification"`
	ClientCert               []string `cli:"--client-cert" value:"cert-file" usage:"client cert (i.e. public key) file; enables mutual TLS"`
	ClientKey                []string `cli:"--client-key" value:"key-file" usage:"client key (i.e. private key) file"`
	NoWarnStdinTTY           bool     `cli:"--no-warn-stdin-tty" usage:"disable warnings about stdin being a tty"`
}

func (_ args) Description() string {
	return "gRPC client"
}

func (_ args) ExtendedDescription() string {
	return strings.TrimSpace(`
gRPCake is a cli gRPC client.

To invoke an RPC, run:

	echo '...' | grpc TARGET METHOD

Where '...' is a JSON-encoded Protobuf message, TARGET is a URL like
"example.com", and METHOD is a full gRPC method name, like
"example.v1.Service.Method". gPRCake will output a JSON-encoded response.

For example:

	$ echo '{"message": "hi"}' | grpc localhost:50051 echo.Echo.Echo
	{"message":"hi"}

If METHOD is client-streaming, then pipe in a sequence of JSON messages instead.
If METHOD is server-streaming, gRPCake will output a stream of JSON messages.

gRPCake discovers methods using reflection by default. To discover using a
".protoset" file instead, use "--protoset".

If METHOD is "ls" or "ll", then gRPCake lists available methods. For example:

	$ gprc localhost:50051 ll
	echo.Echo.Ping
	echo.Echo.Echo
	echo.Echo.ClientStreamEcho
	echo.Echo.ServerStreamEcho
	echo.Echo.BidiStreamEcho
	echo.Echo.EchoMetadata
	grpc.reflection.v1alpha.ServerReflection.ServerReflectionInfo

gRPCake treats ":" as an alias for "localhost:50051", and ":PORT" as an alias
for "localhost:PORT", where "PORT" is a decimal number. In all of the examples
above, you can replace "localhost:50051" with ":" and get the same result.

If you use these aliases, then gRPCake will assume you're developing on a local
RPC server, and will disable TLS. You can always disable TLS with "-k" or
"--insecure".

By default, gRPCake uses the system cert pool to verify servers. You can
customize server verification with "--server-root-ca" and "--server-name". You
can provide multiple server root CAs to try to match against.

gRPCake supports client verification (i.e. "mutual TLS" or "mTLS"). You can
specify client keypairs with "--client-cert" and "--client-key".

Putting it all together, here's how you can test mTLS on a local server:

	grpc ls localhost:50051 \
		--server-root-ca server-ca.crt --server-name test.example.com \
		--client-cert client.crt --client-key client.key

(Note: You can't use TARGET aliases (e.g. ":", ":50051") when testing TLS
locally, because the aliases implicitly disable TLS.)

To pass "metadata" (the gRPC equivalent of HTTP's headers) to a request, use
"-H" or "--header":

	grpc -H "key: value" -H "another key: another value" ...

You can pass the same key multiple times. You can also pass "--header-raw-key"
and "--header-raw-value" in pairs instead:

	grpc --header-raw-key "some key" --header-raw-value "some value" ...

If you want to include/exclude headers in/from reflection calls,
"--reflect-header" and "--rpc-header" (and their "raw" equivalents) are only
used in reflection and non-reflection RPC calls, respectively.

To output server response headers and trailers, use "--dump-header" and
"--dump-trailer".
`)
}

func main() {
	cli.Run(context.Background(), func(ctx context.Context, args args) error {
		args.populateDefaults()

		cc, err := dial(ctx, args)
		if err != nil {
			return err
		}

		ctxReflect, ctxRPC, err := args.metadataContexts(ctx)
		if err != nil {
			return err
		}

		msrc, err := args.methodSource(ctxReflect, cc)
		if err != nil {
			return err
		}

		defer msrc.Close()

		if args.Method == "ll" {
			args.Method = "ls"
			args.Long = true
		}

		if args.Method == "ls" {
			return listMethods(msrc, args)
		}

		return invokeMethod(ctxRPC, cc, msrc, args)
	})
}

func (args args) Autocomplete_Method() []string {
	args.populateDefaults()
	ctx, _, err := args.metadataContexts(context.Background())
	if err != nil {
		return nil
	}

	cc, err := dial(context.Background(), args)
	if len(args.Protoset) == 0 && err != nil {
		// we only need cc if we're using reflection
		return nil
	}

	msrc, err := args.methodSource(ctx, cc)
	if err != nil {
		return nil
	}

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

func (args *args) populateDefaults() {
	if args.UserAgent == "" {
		args.UserAgent = fmt.Sprintf("grpcake/%s", version)
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
	if len(args.Protoset) == 0 {
		return newReflectMethodSource(ctx, args, cc)
	}

	return newProtosetMethodSource(args.Protoset)
}
