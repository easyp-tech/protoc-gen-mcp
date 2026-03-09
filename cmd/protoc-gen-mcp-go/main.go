package main

import (
	"google.golang.org/protobuf/compiler/protogen"
	pluginpb "google.golang.org/protobuf/types/pluginpb"

	"github.com/easyp-tech/protoc-gen-mcp/internal/codegen"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		return codegen.Generate(plugin)
	})
}
