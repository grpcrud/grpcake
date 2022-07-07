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

To call a gRPC RPC method, you need to know the method's input and output
schema. `grpc` supports two ways of discovering what methods are available, and
what types they take and return.

* By default, `grpc` uses [the gRPC reflection
  API](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md) to
  discover methods. If you get an error about:

  ```text
  unknown service grpc.reflection.v1alpha.ServerReflection
  ```

  Then it's likely because `grpc` is trying to do discovery using reflection, but
  the server doesn't implement the reflection API. In that case, you'll need to
  use the other supported discovery method:

* If you pass `--protoset` at least once, `grpc` will instead use [`.protoset`
  files](https://developers.google.com/protocol-buffers/docs/techniques#self-description)
  to discover methods. `.protoset` files, also called "file descriptor sets",
  are a machine-readable version of `.proto` files.

  To generate `.protoset` files, you'll typically need to take your existing
  `protoc` invocation:

  ```sh
  protoc ...
  ```

  And tell it to also generate a `.protoset` in addition to whatever else it's
  generating:

  ```sh
  protoc ... --descriptor_set_out=example.protoset --include_imports
  ```

  Which you can then pass to `grpc`:

  ```sh
  grpc --protoset example.protoset : echo.EchoService.Echo
  ```

  You can pass `--protoset` multiple times if you have multiple `.protoset`
  files.

### gRPC Metadata

To send [gRPC
metadata](https://grpc.io/docs/what-is-grpc/core-concepts/#metadata) using
`grpc`, you can use `-H` / `--header`:

```sh
grpc -H 'key: value' ...
```

The syntax for `-H` / `--header` is meant to be familiar to users of `curl`. But
gRPC metadata keys are allowed to contain colons, so this syntax can be too
constraining. In this case, you can instead pass metadata key/value pairs using
`--header-raw-key` / `--header-raw-value` pairs:

```sh
grpc --header-raw-key key --header-raw-value value
```

gRPC treats metadata keys ending in `-bin` specially, and `grpc` does too.
Metadata keys ending in `-bin` must have base64-encoded binary values. For
instance, to send a header whose value is just NULL (`\0`), which base64-encodes
as `AA==`, do this:

```sh
# these are equivalent
grpc --header "key-bin: AA==" ...
grpc --header-raw-key 'key-bin' --header-raw-value 'AA==' ...
```

Metadata provided via `--header` / `--header-raw-key` / `--header-raw-value` are
passed to both reflection endpoints (if `grpc` is using [reflection-based
discovery](#method-discovery)) and RPC endpoints (i.e. the endpoint you are
calling with `grpc`). To send metadata only to one or the other, you can instead
use:

* `--reflect-header` / `--reflect-header-raw-key` / `--reflect-header-raw-value`
  are only used for method-discovery-related reflection API calls.
* `--rpc-header` / `--rpc-header-raw-key` / `--rpc-header-raw-value` are only
  used for your end RPC calls.

### Headers and Trailers

gRPC server responses can contain both headers and trailers. To dump the headers
and trailers from the server's response, use `--dump-header`
and `--dump-trailer`. Server response metadata are carried in the response
header, so you can use `--dump-header` to see that data.

For example, you can use:

```sh
grpc --dump-header --dump-trailer : example.ExampleService.ExampleMethod
```

This will output JSON messages to stderr that look like this:

```json
{"header":{"content-type":["application/grpc"],"my-custom-header":["foo"]}}
{"trailer":{"my-custom-trailer":["bar"]}}
```

`grpc` will output headers first, then RPC results, then trailers. There will be
exactly one header log line, and exactly one trailer log line.

### Server TLS

`grpc` uses TLS by default. You can force `grpc` to use plaintext by:

* Using one of the shorthand target syntaxes, i.e. `:` or `:xxxx`
* Explicitly disabling TLS using `-k` / `--insecure`

By default, `grpc` will authenticate the server whenever communicating over TLS.
You can customize how that authentication works with a few options:

* `--insecure-skip-server-verify` disables server authentication altogether.
  Communication will still happen over TLS, but the server's authenticity will
  not be established.
* `--server-root-ca <ca-cert>` lets you specify a certificate authority to use
  when verifying the server's certificates. `<ca-cert>` should be the path to a
  PEM-encoded CA certificate. You can pass this multiple times to create a pool
  of CAs. The default is to use the system CA pool.
* `--server-name <server-name-override>` lets you override the expected hostname
  of the server. This value is also used as the `:authority` HTTP/2
  pseudo-header.

Taken together, these last two headers can be useful for testing TLS servers
locally. For instance, here's a sequence of commands that generates a server
keypair for local development:

```sh
openssl genrsa -out server-ca.key 4096
openssl req -x509 -new -nodes -sha256 -key server-ca.key -subj "/CN=example-server-ca" -days 365 -out server-ca.crt

openssl genrsa -out server.key 4096
openssl req -new -sha256 -key server.key -subj "/CN=example-server" -config openssl.conf -reqexts server -out server.csr

openssl x509 -req -sha256 -in server.csr -CA server-ca.crt -CAkey server-ca.key -set_serial 1 -out server.crt -days 365 -extfile openssl.conf -extensions server
```

Where `openssl.conf` contains:

```text
[req]
distinguished_name = req_distinguished_name
attributes = req_attributes

[req_distinguished_name]

[req_attributes]

[server]
subjectAltName=DNS:example-server.example.com
```

Assuming the server then serves over TLS with the creds `server.key` and
`server.crt`, then you can connect to the server by running:

```sh
grpc localhost:50051 ls --server-root-ca server-ca.crt --server-name example-server.example.com
```

Note that you must use `localhost:50051`, not the shorthand `:50051` or `:`,
because `grpc` will disable TLS if you use a shorthand target syntax.

### Mutual TLS

Mutual TLS (aka "mTLS") refers to the idea of the *server* establishing the
authenticity of the *client*, in addition to the server authenticity discussed
in [the previous section on "Server TLS"](#server-tls).

You can provide credentials to present to the server using `--client-key` and
`--client-cert`. You can provide these multiple times, and the first pair
satisfying the server's requirements will be used.

You can use these options to test mutual TLS locally. Here's a sequence of
commands that creates a "client CA" that the server use to verify clients, and a
keypair for the client to use:

```sh
openssl genrsa -out client-ca.key 4096
openssl req -x509 -new -nodes -sha256 -key server-ca.key -subj "/CN=example-client-ca" -days 365 -out client-ca.crt

openssl genrsa -out client.key 4096
openssl req -new -sha256 -key client.key -subj "/CN=example-client" -out client.csr

openssl x509 -req -sha256 -in client.csr -CA client-ca.crt -CAkey client-ca.key -set_serial 1 -out client.crt -days 365
```

Then you could connect to the server, with mutual TLS authentication, by
running:

```sh
grpc localhost:50051 ls \
  --server-root-ca server-ca.crt --server-name example-server.example.com \
  --client-cert client.crt --client-key client.key
```

### Verbose Logging

`grpc` is built on top of [the standard grpc-go
client](https://github.com/grpc/grpc-go), which has built-in support for
logging. You can enable this logging by passing the environment variables:

```sh
GRPC_GO_LOG_VERBOSITY_LEVEL=99 GRPC_GO_LOG_SEVERITY_LEVEL=info grpc ...
```

Which will enable logging of the form:

```text
2022/07/07 10:57:39 INFO: [core] original dial target is: "localhost:50051"
2022/07/07 10:57:39 INFO: [core] parsed dial target is: {Scheme:localhost Authority: Endpoint:50051 URL:{Scheme:localhost Opaque:50051 User: Host: Path: RawPath: ForceQuery:false RawQuery: Fragment: RawFragment:}}
2022/07/07 10:57:39 INFO: [core] fallback to scheme "passthrough"
2022/07/07 10:57:39 INFO: [core] parsed dial target is: {Scheme:passthrough Authority: Endpoint:localhost:50051 URL:{Scheme:passthrough Opaque: User: Host: Path:/localhost:50051 RawPath: ForceQuery:false RawQuery: Fragment: RawFragment:}}
...
```
