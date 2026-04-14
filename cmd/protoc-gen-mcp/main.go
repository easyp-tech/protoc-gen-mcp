package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	pluginpb "google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/protoc-gen-mcp/internal/codegen"
)

func main() {
	requestBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeErrorResponse(fmt.Errorf("read CodeGeneratorRequest: %w", err))
		return
	}

	var request pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(requestBytes, &request); err != nil {
		writeErrorResponse(fmt.Errorf("unmarshal CodeGeneratorRequest: %w", err))
		return
	}

	opts, err := codegen.ParseOptions(request.GetParameter())
	if err != nil {
		writeErrorResponse(err)
		return
	}
	codegen.PrepareRequestForProtogen(&request, opts)

	optionParser := codegen.NewOptionsParser()

	plugin, err := (protogen.Options{
		ParamFunc: optionParser.Set,
	}).New(&request)
	if err != nil {
		writeErrorResponse(err)
		return
	}

	plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
	parsedOpts, err := optionParser.Options()
	if err != nil {
		plugin.Error(err)
	} else if err := codegen.Generate(plugin, parsedOpts); err != nil {
		plugin.Error(err)
	}

	writeResponse(plugin.Response())
}

func writeErrorResponse(err error) {
	writeResponse(&pluginpb.CodeGeneratorResponse{
		Error: proto.String(err.Error()),
	})
}

func writeResponse(response *pluginpb.CodeGeneratorResponse) {
	responseBytes, err := proto.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal CodeGeneratorResponse: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(responseBytes); err != nil {
		fmt.Fprintf(os.Stderr, "write CodeGeneratorResponse: %v\n", err)
		os.Exit(1)
	}
}
