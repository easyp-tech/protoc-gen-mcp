package mcpruntime

import (
	"bufio"
	"context"
	"io"
	"os"
)

const maxScannerBuf = 1 << 20 // 1 MB

// ServeStdio runs the MCP server over stdin/stdout using newline-delimited JSON-RPC.
// Server logs are written to stderr. Blocks until stdin is closed or ctx is cancelled.
func ServeStdio(ctx context.Context, server *Server) error {
	return ServeIO(ctx, server, os.Stdin, os.Stdout)
}

// ServeIO runs the MCP server over the provided reader/writer using newline-delimited JSON-RPC.
// Useful for testing without real stdin/stdout.
func ServeIO(ctx context.Context, server *Server, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, maxScannerBuf), maxScannerBuf)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !scanner.Scan() {
			break
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := server.HandleRaw(ctx, line)
		if resp == nil {
			// Notification — no response to send.
			continue
		}

		// Write response followed by newline.
		resp = append(resp, '\n')
		if _, err := out.Write(resp); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
