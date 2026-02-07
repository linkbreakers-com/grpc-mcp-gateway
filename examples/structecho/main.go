package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type echoClient struct{}

func (e *echoClient) Echo(ctx context.Context, in *structpb.Struct, _ ...grpc.CallOption) (*structpb.Struct, error) {
	return in, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "structecho", Version: "v0.1.0"}, nil)
	RegisterEchoServiceMCPGateway(server, &echoClient{})

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
