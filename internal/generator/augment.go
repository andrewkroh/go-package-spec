package generator

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AugmentConfig holds type and field overrides loaded from augment.yml.
type AugmentConfig struct {
	Types map[string]AugmentType `yaml:"types"`
}

// AugmentType holds overrides for a single Go type.
type AugmentType struct {
	Name   string                  `yaml:"name,omitempty"`
	Doc    string                  `yaml:"doc,omitempty"`
	Fields map[string]AugmentField `yaml:"fields,omitempty"`
}

// AugmentField holds overrides for a single field.
type AugmentField struct {
	Name string `yaml:"name,omitempty"`
	Doc  string `yaml:"doc,omitempty"`
	Type string `yaml:"type,omitempty"` // "any", "*bool", "[]string", "map[string]any"
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

	for typeName, aug := range config.Types {
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

	// Named type.
	if s != "" {
		return GoTypeRef{Named: s}
	}

	return GoTypeRef{Builtin: "any"}
}
