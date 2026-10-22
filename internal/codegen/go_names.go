package codegen

import (
	"fmt"
	"path"
	"strings"
	"unicode"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// goFileAPIName deliberately excludes the descriptor's directory and .proto
// suffix. The resolved GoImportPath provides the namespace, so relocating a proto
// while retaining its basename and go_package does not change the public API.
func goFileAPIName(file *protogen.File) string {
	base := strings.TrimSuffix(path.Base(file.Desc.Path()), ".proto")
	var name strings.Builder
	upper := true
	for _, r := range base {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upper = true
			continue
		}
		if upper {
			r = unicode.ToUpper(r)
		}
		if name.Len() == 0 && !unicode.IsUpper(r) {
			// Go exported identifiers must start with an upper-case letter,
			// including basenames beginning with digits or uncased letters.
			name.WriteByte('X')
		}
		name.WriteRune(r)
		upper = false
	}
	if name.Len() == 0 {
		return "File"
	}
	return name.String()
}

// generateGoFiles validates the complete Go package API before emitting output.
// Ambiguous basenames are errors, never silently disambiguated using a directory
// or the set/order of files in a generation request.
func generateGoFiles(plugin *protogen.Plugin, opts Options) error {
	models := make([]FileModel, 0, len(plugin.Files))
	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}
		model, err := CollectFileModel(file, opts)
		if err != nil {
			return err
		}
		if len(model.Services) == 0 && len(model.Prompts) == 0 && len(model.Resources) == 0 && model.ServerConfig == nil {
			continue
		}
		models = append(models, model)
	}
	if err := validateGoNames(plugin, models); err != nil {
		return err
	}
	for _, model := range models {
		if err := renderGoFile(plugin, model); err != nil {
			return err
		}
	}
	return nil
}

func validateGoNames(plugin *protogen.Plugin, models []FileModel) error {
	// Imported sidecars can already exist in the same Go package even when this
	// request regenerates only one file. Scan their declarations too, without
	// generating imports or validating their unrelated schemas/runtime options.
	packages := map[protogen.GoImportPath]bool{}
	for _, file := range plugin.Files {
		if file.Generate {
			packages[file.GoImportPath] = true
		}
	}
	nameModels := append([]FileModel(nil), models...)
	for _, file := range plugin.Files {
		if file.Generate || !packages[file.GoImportPath] || file.Desc.Syntax() != protoreflect.Proto3 {
			continue
		}
		model, err := importedGoNameModel(file)
		if err != nil {
			return err
		}
		nameModels = append(nameModels, model)
	}
	type symbol struct {
		pkg  protogen.GoImportPath
		name string
	}
	names := map[symbol]string{}
	reserve := func(ident protogen.GoIdent, owner string) {
		names[symbol{ident.GoImportPath, ident.GoName}] = owner
	}
	reserveEnums := func(enums []*protogen.Enum) {
		for _, enum := range enums {
			owner := fmt.Sprintf("protobuf enum %q in %s", enum.Desc.FullName(), enum.Desc.ParentFile().Path())
			reserve(enum.GoIdent, owner)
			reserve(enum.GoIdent.GoImportPath.Ident(enum.GoIdent.GoName+"_name"), owner)
			reserve(enum.GoIdent.GoImportPath.Ident(enum.GoIdent.GoName+"_value"), owner)
			for _, value := range enum.Values {
				reserve(value.GoIdent, owner)
			}
		}
	}
	var reserveMessages func([]*protogen.Message)
	reserveMessages = func(messages []*protogen.Message) {
		for _, message := range messages {
			if message.Desc.IsMapEntry() {
				continue
			}
			owner := fmt.Sprintf("protobuf message %q in %s", message.Desc.FullName(), message.Desc.ParentFile().Path())
			reserve(message.GoIdent, owner)
			for _, field := range message.Fields {
				if field.Oneof != nil && !field.Oneof.Desc.IsSynthetic() {
					reserve(field.GoIdent, owner)
				}
			}
			reserveEnums(message.Enums)
			reserveMessages(message.Messages)
		}
	}
	for _, file := range plugin.Files {
		reserve(file.GoDescriptorIdent, fmt.Sprintf("protobuf descriptor in %s", file.Desc.Path()))
		reserveEnums(file.Enums)
		reserveMessages(file.Messages)
	}
	for _, model := range nameModels {
		info, err := newGoRenderInfo(plugin, model)
		if err != nil {
			return err
		}
		owner := fmt.Sprintf("MCP bindings for %s", model.ProtoPath)
		add := func(name string) error {
			key := symbol{info.file.GoImportPath, name}
			if existing, ok := names[key]; ok {
				return fmt.Errorf("Go symbol collision for %q in go_package %q between %s and %s; rename the proto basename or conflicting declaration, or use a distinct go_package", name, info.file.GoImportPath, existing, owner)
			}
			names[key] = owner
			return nil
		}
		for _, service := range model.Services {
			name, err := info.serviceGoName(service)
			if err != nil {
				return err
			}
			for _, symbol := range []string{name + "ToolHandler", "Register" + name + "Tools"} {
				if err := add(symbol); err != nil {
					return err
				}
			}
		}
		// Validate compatibility names as well: keeping an alias must not create
		// a compile failure when it conflicts with a protobuf declaration.
		for _, base := range []string{goFileAPIName(info.file), info.file.GoDescriptorIdent.GoName} {
			var symbols []string
			if len(model.Prompts) > 0 {
				symbols = append(symbols, base+"PromptHandler", "Register"+base+"Prompts")
			}
			if len(model.Resources) > 0 {
				symbols = append(symbols, base+"ResourceHandler", "Register"+base+"Resources")
			}
			if model.ServerConfig != nil || modelNeedsApps(model) {
				symbols = append(symbols, base+"MCPHandlers", "New"+base+"MCPServer")
			}
			if model.ServerConfig != nil {
				symbols = append(symbols, "New"+base+"MCPHTTPHandler")
			}
			for _, symbol := range symbols {
				if err := add(symbol); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// importedGoNameModel collects only the metadata that determines exported MCP
// symbols. CollectFileModel would also reject unsupported RPC/schema features
// in imports that this invocation is not being asked to generate.
func importedGoNameModel(file *protogen.File) (FileModel, error) {
	config, err := getServerConfig(file)
	if err != nil {
		return FileModel{}, err
	}
	model := FileModel{ProtoPath: file.Desc.Path(), ServerConfig: config}
	for _, service := range file.Services {
		serviceModel := ServiceModel{ProtoFullName: string(service.Desc.FullName())}
		for _, method := range service.Methods {
			if method.Desc.IsStreamingClient() || method.Desc.IsStreamingServer() {
				continue
			}
			metadata, err := loadMethodMetadata(method)
			if err != nil {
				return FileModel{}, err
			}
			if !metadata.Hidden && !metadata.Disabled {
				serviceModel.Methods = append(serviceModel.Methods, MethodModel{AppUI: metadata.AppUI})
			}
		}
		if len(serviceModel.Methods) > 0 {
			model.Services = append(model.Services, serviceModel)
		}
	}
	for _, message := range file.Messages {
		prompt, err := getPromptOptions(message)
		if err != nil {
			return FileModel{}, err
		}
		if prompt != nil {
			model.Prompts = append(model.Prompts, PromptModel{})
		}
		resource, err := getResourceOptions(message)
		if err != nil {
			return FileModel{}, err
		}
		if resource != nil {
			model.Resources = append(model.Resources, ResourceModel{
				MIMEType: strings.TrimSpace(resource.GetMimeType()),
				AppUI:    resource.GetAppUi(),
			})
		}
	}
	return model, nil
}
