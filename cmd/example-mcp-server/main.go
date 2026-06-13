package main

import (
	"context"
	"log"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"
)

func main() {
	server, err := examplemcp.NewServer()
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	if err := mcpruntime.ServeStdio(context.Background(), server); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
