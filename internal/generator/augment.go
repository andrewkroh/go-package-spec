package generator

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// AugmentConfig holds type and field overrides loaded from augment.yml.
type AugmentConfig struct {
	Types     map[string]AugmentType     `yaml:"types"`
	BaseTypes map[string]AugmentBaseType `yaml:"base_types"`
}

// AugmentType holds overrides for a single Go type.
type AugmentType struct {
	Name        string                  `yaml:"name,omitempty"`
	Doc         string                  `yaml:"doc,omitempty"`
	Fields      map[string]AugmentField `yaml:"fields,omitempty"`
	ExtraFields []AugmentExtraField     `yaml:"extra_fields,omitempty"`
}

// AugmentField holds overrides for a single field.
type AugmentField struct {
	Name string `yaml:"name,omitempty"`
	Doc  string `yaml:"doc,omitempty"`
	Type string `yaml:"type,omitempty"` // "any", "*bool", "[]string", "map[string]any"
}

// AugmentExtraField defines a field to inject that is not present in the JSON schema.
type AugmentExtraField struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // e.g., "*time.Time"
	Doc  string `yaml:"doc"`
	JSON string `yaml:"json"` // json tag value
	YAML string `yaml:"yaml"` // yaml tag value
}

// AugmentBaseType defines a base type to extract from multiple source types.
type AugmentBaseType struct {
	Doc        string   `yaml:"doc"`
	EmbedMeta  bool     `yaml:"embed_meta"`
	OutputFile string   `yaml:"output_file"`
	Sources    []string `yaml:"sources"`
	Fields     []string `yaml:"fields"` // JSON field names to extract
}

// LoadAugmentations reads and parses an augment.yml file.
func LoadAugmentations(path string) (*AugmentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading augment config: %w", err)
	}

	var config AugmentConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing augment config: %w", err)
	}
	return &config, nil
}

// ApplyAugmentations applies overrides from the augment config to the
// generated types. It modifies types in place.
func ApplyAugmentations(types map[string]*GoType, config *AugmentConfig) {
	if config == nil {
		return
	}

	// Sort type names for deterministic processing order. This matters
	// when augmentations rename types (e.g. RoutingRule→RoutingRuleSet and
	// RoutingRule2→RoutingRule) because the intermediate map state depends
	// on the order renames are applied.
	typeNames := make([]string, 0, len(config.Types))
	for typeName := range config.Types {
		typeNames = append(typeNames, typeName)
	}
	sort.Strings(typeNames)

	for _, typeName := range typeNames {
		aug := config.Types[typeName]
		goType, ok := types[typeName]
		if !ok {
			continue
		}

		// Rename type.
		if aug.Name != "" && aug.Name != typeName {
			delete(types, typeName)
			oldName := goType.Name
			goType.Name = aug.Name
			types[aug.Name] = goType

			// Update references in all types.
			for _, t := range types {
				for i := range t.Fields {
					updateTypeRef(&t.Fields[i].Type, oldName, aug.Name)
				}
			}

			// Update enum const name prefixes.
			if goType.Kind == GoTypeEnum {
				for i := range goType.EnumValues {
					ev := &goType.EnumValues[i]
					if strings.HasPrefix(ev.GoName, oldName) {
						ev.GoName = aug.Name + ev.GoName[len(oldName):]
					}
				}
			}
		}

		if aug.Doc != "" {
			goType.Doc = aug.Doc
		}

		// Apply field overrides.
		for jsonName, fieldAug := range aug.Fields {
			for i := range goType.Fields {
				if goType.Fields[i].JSONName == jsonName {
					if fieldAug.Name != "" {
						goType.Fields[i].Name = fieldAug.Name
					}
					if fieldAug.Doc != "" {
						goType.Fields[i].Doc = fieldAug.Doc
					}
					if fieldAug.Type != "" {
						goType.Fields[i].Type = parseTypeRef(fieldAug.Type)
					}
					break
				}
			}
		}

		// Append extra fields.
		for _, ef := range aug.ExtraFields {
			goType.Fields = append(goType.Fields, GoField{
				Name:     ef.Name,
				JSONName: ef.JSON,
				Doc:      ef.Doc,
				Type:     parseTypeRef(ef.Type),
				JSONTag:  ef.JSON,
				YAMLTag:  ef.YAML,
			})
		}
	}
}

// ApplyBaseTypes creates base types by extracting common fields from source
// types and embedding the base type in each source.
func ApplyBaseTypes(types map[string]*GoType, config *AugmentConfig) {
	if config == nil {
		return
	}

	for baseName, baseCfg := range config.BaseTypes {
		if len(baseCfg.Sources) == 0 || len(baseCfg.Fields) == 0 {
			continue
		}

		// Find the first source to copy field definitions from.
		firstSource, ok := types[baseCfg.Sources[0]]
		if !ok {
			continue
		}

		// Build a set of JSON field names to extract.
		extractFields := make(map[string]bool, len(baseCfg.Fields))
		for _, f := range baseCfg.Fields {
			extractFields[f] = true
		}

		// Create the base type with extracted fields.
		baseType := &GoType{
			Name: baseName,
			Doc:  baseCfg.Doc,
			Kind: GoTypeStruct,
		}
		if baseCfg.OutputFile != "" {
			baseType.OutputFile = baseCfg.OutputFile
		}

		// Copy matching fields from the first source type.
		for _, field := range firstSource.Fields {
			if extractFields[field.JSONName] {
				baseType.Fields = append(baseType.Fields, field)
			}
		}

		// If embed_meta is requested, add the FileMetadata embed field
		// directly. We do NOT set EmbedMeta=true on the base type because
		// that would also generate UnmarshalYAML, which would conflict
		// with the concrete types' YAML decoding.
		if baseCfg.EmbedMeta {
			metaField := GoField{
				Name:    "FileMetadata",
				Embed:   true,
				JSONTag: "-",
				YAMLTag: "-",
			}
			baseType.Fields = append([]GoField{metaField}, baseType.Fields...)
		}
		types[baseName] = baseType

		// Update each source type: remove extracted fields, remove EmbedMeta,
		// add embed of the base type.
		for _, srcName := range baseCfg.Sources {
			srcType, ok := types[srcName]
			if !ok {
				continue
			}

			// Remove extracted fields.
			var remaining []GoField
			for _, field := range srcType.Fields {
				if !extractFields[field.JSONName] {
					remaining = append(remaining, field)
				}
			}
			srcType.Fields = remaining

			// The source type no longer directly embeds FileMetadata
			// (it comes through the base type embed), but still needs
			// UnmarshalYAML to capture line/column on the promoted
			// FileMetadata.
			if srcType.EmbedMeta {
				srcType.EmbedMeta = false
				srcType.NeedsUnmarshalYAML = true
			}

			// Prepend embed field for the base type.
			embedField := GoField{
				Name:  baseName,
				Type:  GoTypeRef{Named: baseName},
				Embed: true,
			}
			srcType.Fields = append([]GoField{embedField}, srcType.Fields...)
		}
	}
}

// updateTypeRef replaces references to oldName with newName.
func updateTypeRef(ref *GoTypeRef, oldName, newName string) {
	if ref.Named == oldName {
		ref.Named = newName
	}
	if ref.Element != nil {
		updateTypeRef(ref.Element, oldName, newName)
	}
	if ref.MapValue != nil {
		updateTypeRef(ref.MapValue, oldName, newName)
	}
}

// parseTypeRef parses a type string like "any", "*bool", "[]string",
// "map[string]any" into a GoTypeRef.
func parseTypeRef(s string) GoTypeRef {
	s = strings.TrimSpace(s)

	if strings.HasPrefix(s, "*") {
		inner := parseTypeRef(s[1:])
		inner.Pointer = true
		return inner
	}

	if strings.HasPrefix(s, "[]") {
		elem := parseTypeRef(s[2:])
		return GoTypeRef{
			Slice:   true,
			Element: &elem,
		}
	}

	if strings.HasPrefix(s, "map[") {
		// Find closing bracket.
		depth := 0
		closeBracket := -1
		for i := 3; i < len(s); i++ {
			if s[i] == '[' {
				depth++
			} else if s[i] == ']' {
				depth--
				if depth == 0 {
					closeBracket = i
					break
				}
			}
		}
		if closeBracket < 0 {
			return GoTypeRef{Builtin: "any"}
		}
		keyStr := s[4:closeBracket]
		valStr := s[closeBracket+1:]
		key := parseTypeRef(keyStr)
		val := parseTypeRef(valStr)
		return GoTypeRef{
			Map:      true,
			MapKey:   &key,
			MapValue: &val,
		}
	}

	// Builtins.
	switch s {
	case "any", "string", "int", "bool", "float64", "int64":
		return GoTypeRef{Builtin: s}
	}

	// Qualified type (e.g., "time.Time").
	if dotIdx := strings.LastIndex(s, "."); dotIdx > 0 {
		pkg := s[:dotIdx]
		name := s[dotIdx+1:]
		if name != "" {
			return GoTypeRef{Package: pkg, QualName: name}
		}
	}

	// Named type.
	if s != "" {
		return GoTypeRef{Named: s}
	}

	return GoTypeRef{Builtin: "any"}
}
