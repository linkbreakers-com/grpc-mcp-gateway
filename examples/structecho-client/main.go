package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "structecho-client", Version: "v0.1.0"}, nil)
	transport := &mcp.CommandTransport{
		Command: exec.Command("go", "run", "./examples/structecho"),
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "echo",
		Arguments: map[string]any{
			"message": "hello from client",
			"count":   2,
		},
	})
	if err != nil {
		log.Fatalf("call tool failed: %v", err)
	}

	fmt.Printf("tool result error=%v\n", res.IsError)
	fmt.Printf("content=%v\n", res.Content)
	if res.StructuredContent != nil {
		fmt.Printf("structured=%v\n", res.StructuredContent)
	}
}
