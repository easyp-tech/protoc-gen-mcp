package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"
)

func main() {
	transport := flag.String("transport", "stdio", "MCP transport: stdio | http")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address for -transport=http")
	path := flag.String("path", "/mcp", "MCP endpoint path for -transport=http")
	allowAllOrigins := flag.Bool("allow-all-origins", false, "disable Origin checks (http transport, unsafe)")
	flag.Parse()

	server, err := examplemcp.NewServer()
	if err != nil {
		log.Fatalf("create server: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch *transport {
	case "stdio":
		if err := mcpruntime.ServeStdio(ctx, server); err != nil {
			log.Fatalf("run server: %v", err)
		}
	case "http":
		opts := mcpruntime.StreamableHTTPOptions{
			Path:            *path,
			AllowAllOrigins: *allowAllOrigins,
		}
		log.Printf("Streamable HTTP MCP endpoint on http://%s%s", *addr, *path)
		if err := mcpruntime.ServeStreamableHTTP(ctx, *addr, server, opts); err != nil {
			log.Fatalf("run http server: %v", err)
		}
	default:
		log.Fatalf("unknown -transport %q (want stdio or http)", *transport)
	}
}
