package codegen

import (
	"fmt"
	"path"

	"google.golang.org/protobuf/compiler/protogen"
)

// Generate emits MCP bindings for protobuf services in generated files.
func Generate(plugin *protogen.Plugin, opts Options) error {
	switch opts.Language {
	case LanguageGo:
	case LanguagePython:
	case LanguageKotlin:
	case LanguageJava:
	case LanguageTypeScript:
	default:
		return fmt.Errorf("unsupported lang %q", opts.Language)
	}

	pythonPackageInitFiles := map[string]bool{}
	if opts.Language == LanguagePython {
		models, orderedFiles, err := collectPythonModels(plugin, opts)
		if err != nil {
			return err
		}
		pythonHandlers := pythonHandlersForOptions(opts)
		dualPythonHandlers := len(pythonHandlers) > 1
		if err := emitPythonSupportFiles(plugin); err != nil {
			return err
		}
		for _, file := range orderedFiles {
			model := models[file.Desc.Path()]
			for _, handler := range pythonHandlers {
				if !pythonModelRequiresOutputForHandler(model, handler) {
					continue
				}
				outputPath := pythonOutputPathForHandler(file, handler, dualPythonHandlers)
				emitPythonPackageInitFile(plugin, pythonPackageInitFiles, outputPath)
				if err := renderPythonFileForHandler(plugin, model, handler, outputPath); err != nil {
					return err
				}
			}
		}
		return nil
	}

	switch opts.Language {
	case LanguageKotlin:
		models, orderedFiles, err := collectKotlinModels(plugin, opts)
		if err != nil {
			return err
		}
		for _, file := range orderedFiles {
			model := models[file.Desc.Path()]
			if len(model.Services) == 0 {
				continue
			}
			if err := renderKotlinFile(plugin, model); err != nil {
				return err
			}
		}
		return nil
	case LanguageJava:
		models, orderedFiles, err := collectJavaModels(plugin, opts)
		if err != nil {
			return err
		}
		for _, file := range orderedFiles {
			model := models[file.Desc.Path()]
			if len(model.Services) == 0 {
				continue
			}
			if err := renderJavaFile(plugin, model); err != nil {
				return err
			}
		}
		return nil
	case LanguageTypeScript:
		models, orderedFiles, err := collectTypeScriptModels(plugin, opts)
		if err != nil {
			return err
		}
		for _, file := range orderedFiles {
			model := models[file.Desc.Path()]
			if len(model.Services) == 0 {
				continue
			}
			if err := renderTypeScriptFile(plugin, model); err != nil {
				return err
			}
		}
		return nil
	}

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		model, err := CollectFileModel(file, opts)
		if err != nil {
			return err
		}

		switch opts.Language {
		case LanguageGo:
			if len(model.Services) == 0 {
				continue
			}
			if err := renderGoFile(plugin, model); err != nil {
				return err
			}
		case LanguagePython:
			if !pythonModelRequiresOutput(model) {
				continue
			}
			emitPythonPackageInitFile(plugin, pythonPackageInitFiles, pythonOutputPath(file))
			if err := renderPythonFile(plugin, model); err != nil {
				return err
			}
		}
	}

	return nil
}

func collectTypeScriptModels(plugin *protogen.Plugin, opts Options) (map[string]TypeScriptFileModel, []*protogen.File, error) {
	models := make(map[string]TypeScriptFileModel)
	orderedFiles := make([]*protogen.File, 0, len(plugin.Files))

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		model, err := CollectFileModel(file, opts)
		if err != nil {
			return nil, nil, err
		}
		tsModel, err := CollectTypeScriptFileModel(file, model)
		if err != nil {
			return nil, nil, err
		}
		models[file.Desc.Path()] = tsModel
		orderedFiles = append(orderedFiles, file)
	}

	return models, orderedFiles, nil
}

func collectKotlinModels(plugin *protogen.Plugin, opts Options) (map[string]JVMFileModel, []*protogen.File, error) {
	models := make(map[string]JVMFileModel)
	orderedFiles := make([]*protogen.File, 0, len(plugin.Files))

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		model, err := CollectFileModel(file, opts)
		if err != nil {
			return nil, nil, err
		}
		jvmModel, err := CollectJVMFileModel(file, model)
		if err != nil {
			return nil, nil, err
		}
		if err := validateKotlinMetadata(jvmModel); err != nil {
			return nil, nil, err
		}
		models[file.Desc.Path()] = jvmModel
		orderedFiles = append(orderedFiles, file)
	}

	return models, orderedFiles, nil
}

func collectJavaModels(plugin *protogen.Plugin, opts Options) (map[string]JVMFileModel, []*protogen.File, error) {
	models := make(map[string]JVMFileModel)
	orderedFiles := make([]*protogen.File, 0, len(plugin.Files))

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		model, err := CollectFileModel(file, opts)
		if err != nil {
			return nil, nil, err
		}
		jvmModel, err := CollectJVMFileModel(file, model)
		if err != nil {
			return nil, nil, err
		}
		if err := validateJavaMetadata(jvmModel); err != nil {
			return nil, nil, err
		}
		models[file.Desc.Path()] = jvmModel
		orderedFiles = append(orderedFiles, file)
	}

	return models, orderedFiles, nil
}

func collectPythonModels(plugin *protogen.Plugin, opts Options) (map[string]FileModel, []*protogen.File, error) {
	models := make(map[string]FileModel)
	orderedFiles := make([]*protogen.File, 0, len(plugin.Files))

	for _, file := range plugin.Files {
		if !file.Generate {
			continue
		}

		model, err := CollectFileModel(file, opts)
		if err != nil {
			return nil, nil, err
		}
		models[file.Desc.Path()] = model
		orderedFiles = append(orderedFiles, file)
	}

	extraRefs := make(map[string]map[string]struct{})
	for _, file := range orderedFiles {
		graph := models[file.Desc.Path()].PythonTypes
		if graph == nil {
			continue
		}
		for _, typ := range graph.Types {
			if typ.Owner.IsCurrentFile {
				continue
			}
			if extraRefs[typ.Owner.ProtoPath] == nil {
				extraRefs[typ.Owner.ProtoPath] = make(map[string]struct{})
			}
			extraRefs[typ.Owner.ProtoPath][typ.ProtoFullName] = struct{}{}
		}
	}

	for _, file := range orderedFiles {
		refs := extraRefs[file.Desc.Path()]
		if len(refs) == 0 {
			continue
		}
		model := models[file.Desc.Path()]
		if err := augmentPythonModelWithCurrentTypeRefs(file, &model, refs); err != nil {
			return nil, nil, err
		}
		models[file.Desc.Path()] = model
	}

	return models, orderedFiles, nil
}

func pythonModelRequiresOutput(model FileModel) bool {
	return pythonModelRequiresOutputForHandler(model, model.Options.PythonHandler)
}

func pythonModelRequiresOutputForHandler(model FileModel, handler PythonHandler) bool {
	if len(model.Services) > 0 {
		return true
	}
	if handler == PythonHandlerProtobuf {
		return false
	}
	if model.PythonTypes == nil {
		return false
	}
	for _, typ := range model.PythonTypes.Types {
		if typ.Owner.IsCurrentFile {
			return true
		}
	}
	return false
}

func emitPythonPackageInitFile(plugin *protogen.Plugin, emitted map[string]bool, outputPath string) {
	dir := path.Dir(outputPath)
	if dir == "." || dir == "/" {
		return
	}
	filename := path.Join(dir, "__init__.py")
	if emitted[filename] {
		return
	}
	emitted[filename] = true

	generated := plugin.NewGeneratedFile(filename, "")
	generated.P("# Code generated by protoc-gen-mcp. DO NOT EDIT.")
}
