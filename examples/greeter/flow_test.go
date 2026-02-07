package greeter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type greeterServer struct {
	UnimplementedGreeterServer
}

func (greeterServer) SayHello(ctx context.Context, req *HelloRequest) (*HelloReply, error) {
	return &HelloReply{Message: fmt.Sprintf("Hello, %s", req.GetName())}, nil
}

func TestGreeterMCPFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start gRPC server on a bufconn listener.
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	RegisterGreeterServer(grpcServer, greeterServer{})
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(dialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("grpc dial failed: %v", err)
	}
	defer conn.Close()

	// Build gRPC client.
	grpcClient := NewGreeterClient(conn)

	// Start MCP server and register gateway tools.
	mcpServer := mcp.NewServer(&mcp.Implementation{Name: "greeter-mcp", Version: "v0.1.0"}, nil)
	RegisterGreeterMCPGateway(mcpServer, grpcClient)

	// Connect MCP client/server in-memory.
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	_, err = mcpServer.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("mcp server connect failed: %v", err)
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "greeter-client", Version: "v0.1.0"}, nil)
	session, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("mcp client connect failed: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "greeter.say_hello",
		Arguments: map[string]any{
			"name": "Ada",
		},
	})
	if err != nil {
		t.Fatalf("call tool failed: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}

	message := readStructuredMessage(t, res.StructuredContent)
	if message != "Hello, Ada" {
		t.Fatalf("unexpected message: %q", message)
	}
}

func readStructuredMessage(t *testing.T, structured any) string {
	t.Helper()
	if structured == nil {
		t.Fatalf("missing structured content")
	}

	var payload map[string]any
	switch v := structured.(type) {
	case map[string]any:
		payload = v
	case json.RawMessage:
		if err := json.Unmarshal(v, &payload); err != nil {
			t.Fatalf("failed to unmarshal structured content: %v", err)
		}
	case []byte:
		if err := json.Unmarshal(v, &payload); err != nil {
			t.Fatalf("failed to unmarshal structured content: %v", err)
		}
	default:
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("unexpected structured content type %T", v)
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("failed to unmarshal structured content: %v", err)
		}
	}

	msg, _ := payload["message"].(string)
	return msg
}
