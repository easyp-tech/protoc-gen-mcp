package mcpruntime

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
	"weak"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/protobuf/types/known/emptypb"
)

func registryTestTool(name string) ToolSpec[*emptypb.Empty, *emptypb.Empty] {
	return ToolSpec[*emptypb.Empty, *emptypb.Empty]{
		Name:             name,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		NewRequest:       func() *emptypb.Empty { return &emptypb.Empty{} },
		NewResponse:      func() *emptypb.Empty { return &emptypb.Empty{} },
		Handler: func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		},
	}
}

func TestSDKServerRegistryDoesNotRetainServer(t *testing.T) {
	// Do not create a session or capture the server in a test cleanup: the
	// duplicate-name registry must be the only possible remaining owner.
	serverRef := func() weak.Pointer[mcp.Server] {
		server := mcp.NewServer(&mcp.Implementation{Name: "collectible", Version: "v1"}, nil)
		if err := RegisterSDKProtoTool(server, registryTestTool("hello")); err != nil {
			t.Fatal(err)
		}
		return weak.Make(server)
	}()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		runtime.GC()
		_, registered := sdkServerRegistries.Load(serverRef)
		if serverRef.Value() == nil && !registered {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if serverRef.Value() != nil {
		t.Fatal("duplicate-name registry retained an unreachable SDK server")
	}
	t.Fatal("unreachable SDK server's registry entry was not cleaned up")
}

func TestSDKServerRegistryDuplicateNamesAreServerScoped(t *testing.T) {
	spec := registryTestTool("hello")
	for i := 0; i < 2; i++ {
		server := mcp.NewServer(&mcp.Implementation{Name: "scoped", Version: "v1"}, nil)
		if err := RegisterSDKProtoTool(server, spec); err != nil {
			t.Fatalf("register first tool on server %d: %v", i, err)
		}
		runtime.GC()
		if err := RegisterSDKProtoTool(server, spec); err == nil {
			t.Fatalf("duplicate registration on live server %d succeeded after GC", i)
		}
		runtime.KeepAlive(server)
	}
}

func TestSDKServerRegistryConcurrentDuplicateNames(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "concurrent", Version: "v1"}, nil)
	const registrations = 32
	results := make(chan error, registrations)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := 0; i < registrations; i++ {
		workers.Go(func() {
			<-start
			results <- RegisterSDKProtoTool(server, registryTestTool("hello"))
		})
	}
	close(start)
	workers.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful concurrent registrations = %d, want 1", succeeded)
	}
	tools, err := testSDKServerClient(t, server).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "hello" {
		t.Fatalf("registered tools = %+v, want only hello", tools.Tools)
	}
}
