package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"

	_ "github.com/easyp-tech/protoc-gen-mcp/mcp/options/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var fileDescRE = regexp.MustCompile(`fileDesc\("([^"]+)"`)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: protobufesnormalize <generated *_pb.ts>...")
		os.Exit(2)
	}

	for _, path := range os.Args[1:] {
		if err := normalizeFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

func normalizeFile(path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	changed := false
	out := fileDescRE.ReplaceAllFunc(src, func(match []byte) []byte {
		parts := fileDescRE.FindSubmatch(match)
		if len(parts) != 2 {
			return match
		}

		normalized, err := normalizeDescriptorString(string(parts[1]))
		if err != nil {
			return match
		}
		if normalized == string(parts[1]) {
			return match
		}

		changed = true
		return []byte(`fileDesc("` + normalized + `"`)
	})
	if !changed {
		return nil
	}
	return os.WriteFile(path, out, 0o644)
}

func normalizeDescriptorString(encoded string) (string, error) {
	padded := encoded
	if rem := len(padded) % 4; rem != 0 {
		padded += strings.Repeat("=", 4-rem)
	}

	raw, err := base64.StdEncoding.DecodeString(padded)
	if err != nil {
		return "", err
	}

	var descriptor descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal(raw, &descriptor); err != nil {
		return "", err
	}

	deterministic, err := proto.MarshalOptions{Deterministic: true}.Marshal(&descriptor)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(deterministic), nil
}
