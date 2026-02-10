# gRPC MCP Gateway

`grpc-mcp-gateway` is a Go code generator that maps gRPC service methods to MCP tools using protobuf annotations, similar in spirit to `grpc-gateway` but targeting MCP instead of REST.

## What is MCP?

MCP (Model Context Protocol) is a lightweight protocol that lets AI clients discover tools and call them over a simple JSON-RPC interface. It provides a standard way to expose capabilities (tools) so models can interact with your systems safely and consistently.

## Status

This is a minimal, working version that:

- Generates MCP tool registrations from annotated gRPC methods.
- Bridges MCP tool calls to gRPC methods.
- Supports MCP tool metadata (name, title, description, annotations).
- Provides a lightweight MCP HTTP handler (`runtime.MCPServeMux`) with pluggable request logging.
- Keeps MCP tooling stateless (no sessions).

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

## Buf usage

If you generate protos with Buf, install the plugin and add it to your `buf.gen.yaml`.

Install Buf:

```bash
brew install buf
```

Install the plugin (puts `protoc-gen-mcp-gateway` on your PATH):

```bash
go install github.com/linkbreakers-com/grpc-mcp-gateway/cmd/protoc-gen-mcp-gateway@latest
```

Example `buf.gen.yaml`:

```yaml
version: v1
plugins:
  - name: go
    out: generated/go
  - name: go-grpc
    out: generated/go
  - name: mcp-gateway
    out: generated/go
```

Then run:

```bash
buf generate
```

## Generated API

For each service with annotated methods, the generator emits:

```go
func Register<YourService>MCPHandler(mux *runtime.MCPServeMux, client <YourService>Client)
```

This registers MCP tools for annotated methods and routes MCP tool calls to the gRPC client.

## Minimal server startup

```go
lis, _ := net.Listen("tcp", ":50051")
grpcServer := grpc.NewServer()
demov1.RegisterGreeterServer(grpcServer, greeterSvc)
go grpcServer.Serve(lis)

conn, _ := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
client := demov1.NewGreeterClient(conn)

handler := runtime.NewMCPServeMux(
  runtime.ServerMetadata{Name: "greeter-mcp", Version: "v0.1.0"},
  runtime.WithRequestLogger(func(ctx context.Context, req *runtime.MCPRequest) {
    // Optional: log MCP requests here
  }),
)
demov1.RegisterGreeterMCPHandler(handler, client)

http.ListenAndServe(":8090", handler)
```

## Minimal client request (curl)

List tools:

```bash
curl -s http://localhost:8090/ \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

Call a tool:

```bash
curl -s http://localhost:8090/ \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"greeter.say_hello","arguments":{"name":"Ada"}}}'
```

## Production notes

- Add auth and token verification at the HTTP layer before the MCP handler.
- Configure CORS if the MCP client runs in a browser or remote environment.
- Set timeouts on the HTTP server and gRPC client to avoid hanging tool calls.
- Use structured logging by passing `WithRequestLogger` in your MCP mux.

## Complete Production Example

For a comprehensive, production-ready implementation showing all the best practices, see:

**[examples/complete-server](./examples/complete-server/README.md)**

This example demonstrates:
- Bearer token authentication with JSON-RPC error handling
- Request logging for all MCP protocol messages
- Health check endpoints for Kubernetes
- CORS configuration
- Proper handling of `notifications/initialized`
- Response recording for auth failure debugging
- gRPC ↔ HTTP dual server architecture
- Kubernetes deployment patterns

Perfect for teams building production MCP servers.

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

- `github.com/linkbreakers-com/grpc-mcp-gateway/examples/greeter`

Run the test:

```bash
go test ./examples/greeter -run TestGreeterMCPFlow
```

## In Production

This library is used in production at Linkbreakers. We open-sourced it to make it easy for any team with a Protobuf/gRPC API to add MCP support quickly, because we believe MCP will become an increasingly important way to integrate tools into AI workflows.

Linkbreakers MCP server: https://mcp.linkbreakers.com  
MCP directory listing: https://mcp.so/server/linkbreakers

## Limitations (v0)

- Only unary RPCs are supported (streaming RPCs are skipped).
- Tool input/output schemas are permissive object schemas (not fully derived from protobuf yet).

## Project layout

- `cmd/protoc-gen-mcp-gateway`: the protoc plugin
- `proto/mcp/gateway/v1/annotations.proto`: MCP annotation definitions
- `runtime`: MCP <-> protobuf conversion helpers
