package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/easyp-tech/protoc-gen-mcp/internal/examplemcp"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("run stdio server: %v", err)
		}
	case "http":
		// Stateless enables the modern 2026-07-28 protocol. Legacy MCP clients
		// continue to negotiate a supported older version automatically.
		handler := http.Handler(mcp.NewStreamableHTTPHandler(
			func(*http.Request) *mcp.Server { return server },
			&mcp.StreamableHTTPOptions{Stateless: true},
		))
		if !*allowAllOrigins {
			handler = http.NewCrossOriginProtection().Handler(handler)
		}
		mux := http.NewServeMux()
		mux.Handle(*path, handler)
		httpServer := &http.Server{
			Addr:              *addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP shutdown: %v", err)
			}
		}()
		log.Printf("MCP endpoint: http://%s%s", *addr, *path)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("run HTTP server: %v", err)
		}
		stop()
		<-done
	default:
		log.Fatalf("unknown -transport %q (want stdio or http)", *transport)
	}
}
