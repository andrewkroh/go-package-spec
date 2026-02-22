package generator

import "fmt"

// Config holds all configuration for a generator run.
type Config struct {
	SchemaDir   string
	AugmentFile string
	FileMapFile string
	OutputDir   string
	PackageName string
}

// EntryPoint defines a schema file and the Go type name for its root.
type EntryPoint struct {
	SchemaPath string
	GoTypeName string
}

// DefaultEntryPoints returns the standard set of entry-point schemas.
func DefaultEntryPoints() []EntryPoint {
	return []EntryPoint{
		{"integration/manifest.jsonschema.json", "IntegrationManifest"},
		{"input/manifest.jsonschema.json", "InputManifest"},
		{"content/manifest.jsonschema.json", "ContentManifest"},
		{"integration/data_stream/manifest.jsonschema.json", "DataStreamManifest"},
		{"integration/data_stream/fields/fields.jsonschema.json", "Fields"},
		{"integration/changelog.jsonschema.json", "Changelog"},
		{"integration/validation.jsonschema.json", "Validation"},
		{"integration/elasticsearch/transform/manifest.jsonschema.json", "TransformManifest"},
		{"integration/elasticsearch/transform/transform.jsonschema.json", "Transform"},
		{"integration/kibana/tags.jsonschema.json", "Tags"},
		{"integration/data_stream/lifecycle.jsonschema.json", "Lifecycle"},
		{"integration/elasticsearch/pipeline.jsonschema.json", "IngestPipeline"},
		{"integration/data_stream/routing_rules.jsonschema.json", "RoutingRules"},
	}
}

// Run executes the full code generation pipeline.
func Run(cfg Config) error {
	// 1. Load augmentation config (optional).
	var augConfig *AugmentConfig
	if cfg.AugmentFile != "" {
		var err error
		augConfig, err = LoadAugmentations(cfg.AugmentFile)
		if err != nil {
			return fmt.Errorf("loading augmentations: %w", err)
		}
	}

	// 2. Load file map (optional).
	var fileMap *FileMap
	if cfg.FileMapFile != "" {
		var err error
		fileMap, err = LoadFileMap(cfg.FileMapFile)
		if err != nil {
			return fmt.Errorf("loading file map: %w", err)
		}
	}

	// 3. Create schema registry and type mapper.
	registry := NewSchemaRegistry(cfg.SchemaDir)
	mapper := NewTypeMapper(registry)

	// 4. Register and process entry points.
	entryPoints := DefaultEntryPoints()
	for _, ep := range entryPoints {
		mapper.RegisterEntryPoint(ep.SchemaPath, ep.GoTypeName)
	}
	for _, ep := range entryPoints {
		if err := mapper.ProcessEntryPoint(ep.SchemaPath); err != nil {
			return fmt.Errorf("processing %s: %w", ep.SchemaPath, err)
		}
	}

	// 5. Apply augmentations and base types.
	types := mapper.TypesByName()
	ApplyAugmentations(types, augConfig)
	ApplyBaseTypes(types, augConfig)

	// 6. Assign output files.
	if fileMap != nil {
		fileMap.AssignOutputFiles(types)
	} else {
		for _, t := range types {
			t.OutputFile = "types.go"
		}
	}

	// 7. Validate — check for duplicate names.
	if err := validate(types); err != nil {
		return err
	}

	// 8. Emit Go files.
	pkgName := cfg.PackageName
	if pkgName == "" {
		pkgName = "packagespec"
	}
	emitter := NewEmitter(pkgName, cfg.OutputDir)

	allTypes := mapper.Types()
	return emitter.Emit(allTypes)
}

// validate checks for issues in the generated types.
func validate(types map[string]*GoType) error {
	// Check for enum values that collide.
	enumValues := make(map[string]string) // goName → typeName
	for _, t := range types {
		if t.Kind != GoTypeEnum {
			continue
		}
		for _, ev := range t.EnumValues {
			if existing, ok := enumValues[ev.GoName]; ok {
				return fmt.Errorf("enum value %q conflicts between %s and %s",
					ev.GoName, existing, t.Name)
			}
			enumValues[ev.GoName] = t.Name
		}
	}
	return nil
}
