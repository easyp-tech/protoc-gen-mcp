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

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var transport = flag.String("transport", "stdio", "MCP transport: stdio or http")
	var address = flag.String("addr", "127.0.0.1:8080", "listen address for HTTP mode")
	var devAuth = flag.Bool("demo-auth", false, "enable INSECURE development tokens; loopback-only")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server, err := newShowcaseServer(ctx)
	if err != nil { log.Fatalf("build MCP server: %v", err) }

	switch *transport {
	case "stdio":
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("run stdio: %v", err)
		}
	case "http":
		if !*devAuth {
			log.Fatal("this example's OAuth/JWKS URLs are placeholders; pass -demo-auth for loopback testing or supply a real IdP implementation")
		}
		if !demoHTTPAddressIsLoopback(*address) {
			log.Fatal("-demo-auth is permitted only on a loopback bind address (127.0.0.1, ::1, or localhost)")
		}
		handler, err := newShowcaseHTTPHandler(server, demoVerifier)
		if err != nil { log.Fatalf("configure OAuth HTTP handler: %v", err) }
		srv := &http.Server{Addr: *address, Handler: handler, ReadHeaderTimeout: 5*time.Second}
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdownCtx)
		}()
		log.Printf("DEVELOPMENT ONLY: HTTP MCP endpoint listening at http://%s/mcp", *address)
		log.Print("Demo tokens: demo-read (showcase:read), demo-write (showcase:read + showcase:write)")
		log.Print("The advertised OAuth issuer/JWKS in the .proto file are placeholders, NOT a running IdP.")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err,http.ErrServerClosed) {
			log.Fatalf("serve MCP HTTP: %v",err)
		}
	default:
		log.Fatalf("unsupported -transport=%q; expected stdio or http",*transport)
	}
}
