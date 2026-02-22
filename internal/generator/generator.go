package generator

import (
	"fmt"
	"strings"
)

// Config holds all configuration for a generator run.
type Config struct {
	SchemaDir   string
	AugmentFile string
	FileMapFile string
	OutputDir   string
	PackageName string
	SpecVersion string // Package-spec version override (auto-detected from schema $id if empty).
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
		{"integration/_dev/build/build.jsonschema.json", "BuildManifest"},
		{"integration/_dev/test/config.jsonschema.json", "TestConfig"},
		{"input/_dev/test/config.jsonschema.json", "InputTestConfig"},
		{"integration/data_stream/_dev/test/pipeline/common_config.jsonschema.json", "PipelineTestCommonConfig"},
		{"integration/data_stream/_dev/test/pipeline/config_json.jsonschema.json", "PipelineTestJSONConfig"},
		{"integration/data_stream/_dev/test/pipeline/config_raw.jsonschema.json", "PipelineTestRawConfig"},
		{"integration/data_stream/_dev/test/pipeline/event.jsonschema.json", "PipelineTestEvent"},
		{"integration/data_stream/_dev/test/pipeline/expected.jsonschema.json", "PipelineTestExpected"},
		{"integration/data_stream/_dev/test/policy/config.jsonschema.json", "PolicyTestConfig"},
		{"integration/data_stream/_dev/test/static/config.jsonschema.json", "StaticTestConfig"},
		{"integration/data_stream/_dev/test/system/config.jsonschema.json", "SystemTestConfig"},
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

	// 5. Auto-detect spec version from schema $id if not provided.
	if cfg.SpecVersion == "" {
		for _, s := range registry.schemas {
			if v := extractSpecVersion(s.ID); v != "" {
				cfg.SpecVersion = v
				break
			}
		}
	}
	if cfg.SpecVersion == "" {
		return fmt.Errorf("could not determine spec version: use -spec-version flag or ensure schemas contain a $id with a version")
	}

	// 6. Apply augmentations and base types.
	types := mapper.TypesByName()
	ApplyAugmentations(types, augConfig)
	ApplyBaseTypes(types, augConfig)

	// 7. Assign output files.
	if fileMap != nil {
		fileMap.AssignOutputFiles(types)
	} else {
		for _, t := range types {
			t.OutputFile = "types.go"
		}
	}

	// 8. Validate — check for duplicate names.
	if err := validate(types); err != nil {
		return err
	}

	// 9. Emit Go files.
	pkgName := cfg.PackageName
	if pkgName == "" {
		pkgName = "pkgspec"
	}
	emitter := NewEmitter(pkgName, cfg.OutputDir, cfg.SpecVersion)

	allTypes := mapper.Types()
	return emitter.Emit(allTypes)
}

// extractSpecVersion extracts the version from a JSON Schema $id URL.
// The expected format is "https://schemas.elastic.dev/package-spec/{VERSION}/...".
func extractSpecVersion(id string) string {
	const prefix = "package-spec/"
	idx := strings.Index(id, prefix)
	if idx < 0 {
		return ""
	}
	rest := id[idx+len(prefix):]
	if end := strings.Index(rest, "/"); end > 0 {
		return rest[:end]
	}
	return rest
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
