# gRPCake

gRPCake is a command-line [gRPC](https://grpc.io) client. Its goal is to make
interacting with gRPC services as straightforward as possible. gRPCake is great 
for testing, debugging, or otherwise interacting with gRPC services, including
in development. gRPCake ships as a conventional Unix tool called `grpc`.

Here's how you'd test an "echo" endpoint on a gRPC server running on
`localhost:8080`:

```console
$ echo '{ "message": "hello" }' | grpc :8080 echo.EchoService.Echo
{"message":"hello"}
```

Here's gRPCake listing all available endpoints on the public
[grpcb.in](https://grpcb.in/) service:

```console
$ grpc grpcb.in:9001 ls
grpcbin.GRPCBin.Index
grpcbin.GRPCBin.Empty
grpcbin.GRPCBin.DummyUnary
[...]
```

## Installation

TODO

## Basic Usage

You use gRPCake using the `grpc` command. There are two ways you can use `grpc`. 
To list available RPC methods, use `ls`:

```sh
# you can also do "ls -l" (or "ll") for more detailed output
grpc TARGET ls
```

To call an endpoint, provide the full name (e.g. `package.Service.Method`) of
the endpoint:

```sh
grpc TARGET package.Service.Method
```

In either case, `TARGET` can be:

* `:xxxx` is a shorthand for `localhost:xxxx`
* `:` is a shorthand for `localhost:50051`, the most common "dev" gRPC port
* Any [standard gRPC target
  name](https://github.com/grpc/grpc/blob/master/doc/naming.md), such as
  `grpc.example.com:8080`

When you call an endpoint, `grpc` will read JSON from stdin and will output JSON
to stdout. Typically, that means you'll want to pipe a message into `grpc`, and
you can pretty-print or manipulate `grpc`'s output by piping it into
[`jq`](https://stedolan.github.io/jq/). Something like this:

```sh
echo '{ "message": "hello" }' | grpc : echo.EchoService.Echo | jq '.'
```

(If your endpoint is client-streaming or server-streaming, `grpc` will
read/write a *stream* of JSON instead of a singular message.)

To pass an auth token to your endpoint as a gRPC "metadata" field, use `-H` /
`--header`:

```sh
grpc -H 'Authorization: Bearer ...' : package.service.Method
```

### Debugging Common Problems

If you get an error about an unknown "reflection" service:

```console
$ grpc : ls
grpc: recv ListServices: rpc error: code = Unimplemented desc = unknown service grpc.reflection.v1alpha.ServerReflection (does the server have gRPC reflection enabled?)
```

Then it's likely the server you're connecting to doesn't have gRPC reflection
enabled. You can solve this by [enabling gRPC reflection on your
server](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md#known-implementations)
or by [using `.protoset`-based method discovery instead](#method-discovery).

If you get an error about the server expecting plaintext:

```console
$ grpc grpcb.in:9000 ls
grpc: rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: tls: first record does not look like a TLS handshake" (is the server expecting plaintext?)
```

Then it's likely you're using TLS, but the server expects non-TLS (plaintext)
communication. You can solve this by [passing `-k` / `--insecure`](#server-tls).

## Advanced Usage

### Method Discovery

### gRPC Metadata

### Headers and Trailers

### Server TLS

### Mutual TLS

[//]: # ()
[//]: # (You can also list methods available on the server using `ls`:)

[//]: # ()
[//]: # (```bash)

[//]: # (grpc localhost:8080 ls)

[//]: # (```)

[//]: # ()
[//]: # (```text)

[//]: # (echo.Echo.Ping)

[//]: # (echo.Echo.Echo)

[//]: # ([...])

[//]: # (```)

[//]: # ()
[//]: # (`ls -l`, or its alias `ll`, shows you the signature of each method:)

[//]: # ()
[//]: # (```bash)

[//]: # (grpc localhost:8080 ll)

[//]: # (```)

[//]: # ()
[//]: # (```text)

[//]: # (rpc echo.Echo.Ping&#40;google.protobuf.Empty&#41; returns &#40;echo.PingMessage&#41;)

[//]: # (rpc echo.Echo.Echo&#40;echo.EchoMessage&#41; returns &#40;echo.EchoMessage&#41;)

[//]: # ([...])

[//]: # (```)

[//]: # ()
[//]: # (## Method Discovery)

[//]: # ()
[//]: # (You can't call a gRPC method without knowing the signature of that gRPC method)

[//]: # (-- what its input and output types are, and whether the client and/or server is)

[//]: # (streaming.)

[//]: # ()
[//]: # (To deal with this, gRPCake supports three different ways of discovering methods)

[//]: # (and their signatures:)

[//]: # ()
[//]: # (* With the `reflection` strategy, gRPCake uses the gRPC reflection API)

[//]: # (* With the `protoset` strategy, gRPCake loads methods from a `.protoset` file)

[//]: # (* With the `proto-path` strategy, gRPCake loads methods by compiling `.proto`)

[//]: # (  files into a `.protoset` file for you)
