# gRPCake

gRPCake, which installs as a binary called `grpc`, is an easy-to-use
command-line gRPC client.

```bash
echo '{ "message": "hello" }' | grpc localhost:8080 echo.Echo.Echo | jq 
```
```json
{
  "message": "hello"
}
```

You can also list methods available on the server using `ls`:

```bash
grpc localhost:8080 ls
```

```text
echo.Echo.Ping
echo.Echo.Echo
[...]
```

`ls -l`, or its alias `ll`, shows you the signature of each method:

```bash
grpc localhost:8080 ll
```

```text
rpc echo.Echo.Ping(google.protobuf.Empty) returns (echo.PingMessage)
rpc echo.Echo.Echo(echo.EchoMessage) returns (echo.EchoMessage)
[...]
```

## Method Discovery

You can't call a gRPC method without knowing the signature of that gRPC method
-- what its input and output types are, and whether the client and/or server is
streaming.

To deal with this, gRPCake supports three different ways of discovering methods
and their signatures:

* With the `reflection` strategy, gRPCake uses the gRPC reflection API
* With the `protoset` strategy, gRPCake loads methods from a `.protoset` file
* With the `proto-path` strategy, gRPCake loads methods by compiling `.proto`
  files into a `.protoset` file for you
