package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	filemanagerv1 "github.com/easyp-tech/protoc-gen-mcp/examples/3_file_manager/proto"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type fileManagerAPI struct {
	basePath string
}

func (s *fileManagerAPI) ReadFile(ctx context.Context, req *filemanagerv1.ReadFileRequest) (*filemanagerv1.ReadFileResponse, error) {
	// The validation constraints in the proto schema already ensure LLM provides safe filenames,
	// but we should still double-check in Go.
	filePath := filepath.Join(s.basePath, req.Filename)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file failed: %v", err)
	}
	return &filemanagerv1.ReadFileResponse{Content: string(data)}, nil
}

func (s *fileManagerAPI) DeleteFile(ctx context.Context, req *filemanagerv1.DeleteFileRequest) (*filemanagerv1.DeleteFileResponse, error) {
	filePath := filepath.Join(s.basePath, req.Filename)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return &filemanagerv1.DeleteFileResponse{Success: false}, fmt.Errorf("file not found")
		}
		return nil, fmt.Errorf("delete file failed: %v", err)
	}
	return &filemanagerv1.DeleteFileResponse{Success: true}, nil
}

func main() {
	tmpDir := os.TempDir()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "filemanager-mcp-server",
		Version: "1.0.0",
	}, nil)

	impl := &fileManagerAPI{basePath: tmpDir}
	if err := filemanagerv1.RegisterFileManagerAPITools(server, impl); err != nil {
		log.Fatalf("failed to register tools: %v", err)
	}

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run server: %v", err)
	}
}
