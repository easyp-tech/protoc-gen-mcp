package main

import (
	"context"
	"fmt"
	"log"

	helloworldv1 "github.com/easyp-tech/protoc-gen-mcp/examples/1_helloworld/proto"
	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"
)

type greeter struct{}

func (s *greeter) SayHello(ctx context.Context, req *helloworldv1.SayHelloRequest) (*helloworldv1.SayHelloResponse, error) {
	msg := fmt.Sprintf("Hello, %s!", req.Name)
	return &helloworldv1.SayHelloResponse{Message: msg}, nil
}

func main() {
	server := mcpruntime.NewServer("helloworld-mcp-server", "1.0.0")

	if err := helloworldv1.RegisterGreeterAPITools(server, &greeter{}); err != nil {
		log.Fatalf("failed to register tools: %v", err)
	}

	if err := mcpruntime.ServeStdio(context.Background(), server); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
