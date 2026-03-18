package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	helloworldv1 "github.com/easyp-tech/protoc-gen-mcp/examples/1-helloworld/proto"
)

type greeter struct{}

func (s *greeter) SayHello(ctx context.Context, req *helloworldv1.SayHelloRequest) (*helloworldv1.SayHelloResponse, error) {
	msg := fmt.Sprintf("Hello, %s!", req.Name)
	return &helloworldv1.SayHelloResponse{Message: msg}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "helloworld-mcp-server",
		Version: "1.0.0",
	}, nil)

	if err := helloworldv1.RegisterGreeterAPITools(server, &greeter{}); err != nil {
		log.Fatalf("failed to register tools: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
