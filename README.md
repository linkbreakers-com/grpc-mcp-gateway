# grpc-mcp-gateway

grpc-mcp-gateway is a Go code generator that maps gRPC service methods to MCP tools using protobuf annotations, similar in spirit to grpc-gateway but targeting MCP instead of REST.

## Status

This is a minimal, working v0 that:

- Generates MCP tool registrations from annotated gRPC methods.
- Bridges MCP tool calls to gRPC methods.
- Supports MCP tool metadata (name, title, description, annotations).

## MCP annotations

Define MCP annotations in your proto file alongside any REST annotations:

```proto
syntax = "proto3";

package demo.v1;

import "google/api/annotations.proto";
import "mcp/gateway/v1/annotations.proto";

service Greeter {
  rpc SayHello(HelloRequest) returns (HelloReply) {
    option (google.api.http) = {
      post: "/v1/hello"
      body: "*"
    };
    option (mcp.gateway.v1.mcp) = {
      tool: {
        name: "greeter.say_hello"
        title: "Say Hello"
        description: "Greets a caller."
        read_only: true
      }
    };
  }
}
```

Annotation schema lives at `proto/mcp/gateway/v1/annotations.proto`.

## Generator usage

```bash
protoc \
  -I . \
  -I ./proto \
  --go_out=. --go-grpc_out=. \
  --mcp-gateway_out=. \
  path/to/your.proto
```

## Generated API

For each service with annotated methods, the generator emits:

```go
func Register<YourService>MCPGateway(server *mcp.Server, client <YourService>Client)
```

This registers MCP tools for annotated methods and routes MCP tool calls to the gRPC client.

## Example server wiring

```go
server := mcp.NewServer(&mcp.Implementation{
  Name:    "greeter-mcp",
  Version: "v0.1.0",
}, nil)

conn, _ := grpc.Dial("localhost:50051", grpc.WithInsecure())
client := demov1.NewGreeterClient(conn)

RegisterGreeterMCPGateway(server, client)
```

## Example client (end-to-end)

This repo includes a tiny MCP client that spawns the example server and calls the `echo` tool:

```bash
go run ./examples/structecho-client
```

Expected output:

```
tool result error=false
content=[...]
structured=map[...]
```

## Example with real protobuf + gRPC

A full end-to-end test (gRPC server + MCP gateway + MCP client) lives in:

- `grpc-mcp-gateway/examples/greeter`

Run the test:

```bash
go test ./examples/greeter -run TestGreeterMCPFlow
```

## Limitations (v0)

- Only unary RPCs are supported (streaming RPCs are skipped).
- Tool input/output schemas are permissive object schemas (not fully derived from protobuf yet).

## Project layout

- `cmd/protoc-gen-mcp-gateway`: the protoc plugin
- `proto/mcp/gateway/v1/annotations.proto`: MCP annotation definitions
- `runtime`: MCP <-> protobuf conversion helpers
