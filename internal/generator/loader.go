package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Schema represents a JSON Schema document. Only the keywords needed for
// code generation are included; validation-only keywords are ignored.
//
// In JSON Schema, a schema can be either a boolean (true = accept anything,
// false = reject everything) or an object. The BooleanSchema field captures
// this; when non-nil, all other fields are ignored.
type Schema struct {
	// BooleanSchema is non-nil when the schema is a bare boolean value.
	// true means "accept anything" (equivalent to {}), false means "reject everything".
	BooleanSchema *bool `json:"-"`

	// Core
	Ref         string             `json:"$ref,omitempty"`
	Defs        map[string]*Schema `json:"$defs,omitempty"`
	Definitions map[string]*Schema `json:"definitions,omitempty"`

	// Metadata
	ID          string `json:"$id,omitempty"`
	SchemaURI   string `json:"$schema,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`

	// Type
	Type  SchemaType        `json:"type,omitempty"`
	Const json.RawMessage   `json:"const,omitempty"`
	Enum  []json.RawMessage `json:"enum,omitempty"`

	// Object
	Properties           map[string]*Schema    `json:"properties,omitempty"`
	PatternProperties    map[string]*Schema    `json:"patternProperties,omitempty"`
	AdditionalProperties *AdditionalProperties `json:"additionalProperties,omitempty"`
	Required             []string              `json:"required,omitempty"`

	// Array
	Items    *Schema `json:"items,omitempty"`
	MinItems *int    `json:"minItems,omitempty"`
	MaxItems *int    `json:"maxItems,omitempty"`

	// Numeric
	Minimum          *float64 `json:"minimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	MultipleOf       *float64 `json:"multipleOf,omitempty"`

	// String
	Pattern   string `json:"pattern,omitempty"`
	Format    string `json:"format,omitempty"`
	MinLength *int   `json:"minLength,omitempty"`
	MaxLength *int   `json:"maxLength,omitempty"`

	// Composition
	AllOf []*Schema `json:"allOf,omitempty"`
	AnyOf []*Schema `json:"anyOf,omitempty"`
	OneOf []*Schema `json:"oneOf,omitempty"`
	Not   *Schema   `json:"not,omitempty"`

	// Conditional
	If   *Schema `json:"if,omitempty"`
	Then *Schema `json:"then,omitempty"`
	Else *Schema `json:"else,omitempty"`

	// Defaults
	Default json.RawMessage `json:"default,omitempty"`

	// Examples
	Examples []json.RawMessage `json:"examples,omitempty"`

	// Deprecated
	Deprecated bool `json:"deprecated,omitempty"`
}

// UnmarshalJSON handles both boolean schemas (true/false) and object schemas.
func (s *Schema) UnmarshalJSON(data []byte) error {
	// Try boolean first.
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		s.BooleanSchema = &b
		return nil
	}

	// Use an alias to avoid infinite recursion.
	type schemaAlias Schema
	var sa schemaAlias
	if err := json.Unmarshal(data, &sa); err != nil {
		return err
	}
	*s = Schema(sa)
	return nil
}

// IsBooleanSchema returns true if this schema is a bare boolean value.
func (s *Schema) IsBooleanSchema() bool {
	return s.BooleanSchema != nil
}

// SchemaType handles JSON Schema "type" which can be a string or array of strings.
type SchemaType struct {
	values []string
}

// Single returns the single type string if there is exactly one, otherwise "".
func (t SchemaType) Single() string {
	if len(t.values) == 1 {
		return t.values[0]
	}
	return ""
}

// Values returns all type values.
func (t SchemaType) Values() []string {
	return t.values
}

// IsEmpty returns true if no type is specified.
func (t SchemaType) IsEmpty() bool {
	return len(t.values) == 0
}

// Contains returns true if the type list contains the given type.
func (t SchemaType) Contains(typ string) bool {
	for _, v := range t.values {
		if v == typ {
			return true
		}
	}
	return false
}

// UnmarshalJSON handles "type": "string" and "type": ["string", "null"].
func (t *SchemaType) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		t.values = []string{single}
		return nil
	}
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		t.values = arr
		return nil
	}
	return fmt.Errorf("cannot unmarshal type: %s", string(data))
}

// MarshalJSON encodes the type as a string or array.
func (t SchemaType) MarshalJSON() ([]byte, error) {
	if len(t.values) == 1 {
		return json.Marshal(t.values[0])
	}
	return json.Marshal(t.values)
}

// AdditionalProperties handles the JSON Schema additionalProperties keyword,
// which can be a boolean or a schema.
type AdditionalProperties struct {
	Bool   *bool
	Schema *Schema
}

// UnmarshalJSON handles both boolean and schema forms.
func (ap *AdditionalProperties) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		ap.Bool = &b
		return nil
	}
	var s Schema
	if err := json.Unmarshal(data, &s); err == nil {
		ap.Schema = &s
		return nil
	}
	return fmt.Errorf("cannot unmarshal additionalProperties: %s", string(data))
}

// MarshalJSON encodes additional properties.
func (ap AdditionalProperties) MarshalJSON() ([]byte, error) {
	if ap.Bool != nil {
		return json.Marshal(*ap.Bool)
	}
	return json.Marshal(ap.Schema)
}

// IsFalse returns true if additionalProperties is explicitly false.
func (ap *AdditionalProperties) IsFalse() bool {
	return ap != nil && ap.Bool != nil && !*ap.Bool
}

// SchemaRegistry loads and caches JSON Schema files, providing $ref resolution
// across files within a schema directory tree.
type SchemaRegistry struct {
	baseDir string
	schemas map[string]*Schema
}

// NewSchemaRegistry creates a registry rooted at the given base directory.
// All schema file paths are resolved relative to this directory.
func NewSchemaRegistry(baseDir string) *SchemaRegistry {
	return &SchemaRegistry{
		baseDir: baseDir,
		schemas: make(map[string]*Schema),
	}
}

// LoadSchema reads and parses a JSON Schema file at the given path relative
// to the registry base directory. Schemas are cached by relative path.
func (r *SchemaRegistry) LoadSchema(relPath string) (*Schema, error) {
	relPath = filepath.Clean(relPath)
	if s, ok := r.schemas[relPath]; ok {
		return s, nil
	}

	absPath := filepath.Join(r.baseDir, relPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("loading schema %s: %w", relPath, err)
	}

	var s Schema
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing schema %s: %w", relPath, err)
	}

	r.schemas[relPath] = &s
	return &s, nil
}

// ResolveRef resolves a JSON Schema $ref string relative to a context file.
// It returns the resolved schema and the file it was loaded from.
//
// Supported $ref patterns:
//   - "#/definitions/foo"                — same-file definition
//   - "#/$defs/bar"                      — same-file $defs
//   - "#"                                — self-reference (root of current file)
//   - "./other.json#/definitions/baz"    — cross-file with fragment
//   - "../../foo.json#/definitions/x"    — parent traversal cross-file
//   - "#/definitions/a/properties/b"     — deep path within definitions
func (r *SchemaRegistry) ResolveRef(ref, contextFile string) (*Schema, string, error) {
	filePart, fragmentPart := splitRef(ref)

	// Determine which file the ref points to.
	targetFile := contextFile
	if filePart != "" {
		// Resolve file path relative to contextFile's directory.
		contextDir := filepath.Dir(contextFile)
		targetFile = filepath.Clean(filepath.Join(contextDir, filePart))
	}

	schema, err := r.LoadSchema(targetFile)
	if err != nil {
		return nil, "", fmt.Errorf("resolving $ref %q from %s: %w", ref, contextFile, err)
	}

	// No fragment means the entire schema.
	if fragmentPart == "" {
		return schema, targetFile, nil
	}

	// Walk the JSON Pointer path.
	resolved, err := walkJSONPointer(schema, fragmentPart)
	if err != nil {
		return nil, "", fmt.Errorf("resolving $ref %q from %s: %w", ref, contextFile, err)
	}
	return resolved, targetFile, nil
}

// splitRef splits a $ref into file part and fragment part.
// Examples:
//
//	"#/definitions/foo"           → ("", "/definitions/foo")
//	"./other.json#/definitions/x" → ("./other.json", "/definitions/x")
//	"#"                           → ("", "")
//	"./other.json"                → ("./other.json", "")
func splitRef(ref string) (filePart, fragmentPart string) {
	idx := strings.Index(ref, "#")
	if idx < 0 {
		return ref, ""
	}
	filePart = ref[:idx]
	fragmentPart = ref[idx+1:]
	return filePart, fragmentPart
}

// walkJSONPointer traverses a Schema following a JSON Pointer path
// (e.g. "/definitions/foo/properties/bar").
func walkJSONPointer(schema *Schema, pointer string) (*Schema, error) {
	if pointer == "" || pointer == "/" {
		return schema, nil
	}

	// Remove leading "/".
	pointer = strings.TrimPrefix(pointer, "/")
	parts := strings.Split(pointer, "/")

	current := schema
	for i := 0; i < len(parts); i++ {
		segment := unescapeJSONPointer(parts[i])

		switch segment {
		case "definitions":
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("incomplete pointer: missing definition name after /definitions")
			}
			i++
			name := unescapeJSONPointer(parts[i])
			next, ok := current.Definitions[name]
			if !ok {
				return nil, fmt.Errorf("definition %q not found", name)
			}
			current = next

		case "$defs":
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("incomplete pointer: missing def name after /$defs")
			}
			i++
			name := unescapeJSONPointer(parts[i])
			next, ok := current.Defs[name]
			if !ok {
				return nil, fmt.Errorf("$def %q not found", name)
			}
			current = next

		case "properties":
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("incomplete pointer: missing property name after /properties")
			}
			i++
			name := unescapeJSONPointer(parts[i])
			next, ok := current.Properties[name]
			if !ok {
				return nil, fmt.Errorf("property %q not found", name)
			}
			current = next

		case "items":
			if current.Items == nil {
				return nil, fmt.Errorf("items is nil")
			}
			current = current.Items

		case "allOf", "anyOf", "oneOf":
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("incomplete pointer: missing index after /%s", segment)
			}
			i++
			idx, err := strconv.Atoi(parts[i])
			if err != nil {
				return nil, fmt.Errorf("invalid array index %q in /%s", parts[i], segment)
			}
			var arr []*Schema
			switch segment {
			case "allOf":
				arr = current.AllOf
			case "anyOf":
				arr = current.AnyOf
			case "oneOf":
				arr = current.OneOf
			}
			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("index %d out of range for /%s (len=%d)", idx, segment, len(arr))
			}
			current = arr[idx]

		case "then":
			if current.Then == nil {
				return nil, fmt.Errorf("then is nil")
			}
			current = current.Then

		case "else":
			if current.Else == nil {
				return nil, fmt.Errorf("else is nil")
			}
			current = current.Else

		case "if":
			if current.If == nil {
				return nil, fmt.Errorf("if is nil")
			}
			current = current.If

		case "additionalProperties":
			if current.AdditionalProperties == nil || current.AdditionalProperties.Schema == nil {
				return nil, fmt.Errorf("additionalProperties is not a schema")
			}
			current = current.AdditionalProperties.Schema

		case "not":
			if current.Not == nil {
				return nil, fmt.Errorf("not is nil")
			}
			current = current.Not

		default:
			// Try looking up as a property.
			if current.Properties != nil {
				if next, ok := current.Properties[segment]; ok {
					current = next
					continue
				}
			}
			// Try looking up as a definition.
			if current.Definitions != nil {
				if next, ok := current.Definitions[segment]; ok {
					current = next
					continue
				}
			}
			if current.Defs != nil {
				if next, ok := current.Defs[segment]; ok {
					current = next
					continue
				}
			}
			return nil, fmt.Errorf("cannot resolve pointer segment %q", segment)
		}
	}

	return current, nil
}

// unescapeJSONPointer reverses the JSON Pointer escaping.
// ~1 → /, ~0 → ~
func unescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}

// AllDefinitions returns a merged map of all definitions from both
// "definitions" and "$defs" in a schema.
func (s *Schema) AllDefinitions() map[string]*Schema {
	result := make(map[string]*Schema)
	for k, v := range s.Definitions {
		result[k] = v
	}
	for k, v := range s.Defs {
		result[k] = v
	}
	return result
}

// IsRequired returns true if the given property name is in the Required list.
func (s *Schema) IsRequired(prop string) bool {
	for _, r := range s.Required {
		if r == prop {
			return true
		}
	}
	return false
}

// HasProperties returns true if the schema defines object properties.
func (s *Schema) HasProperties() bool {
	return len(s.Properties) > 0
}

// EnumStrings returns the enum values as strings. Non-string enum values
// are returned as their JSON representation.
func (s *Schema) EnumStrings() []string {
	var result []string
	for _, raw := range s.Enum {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			result = append(result, str)
			continue
		}
		// Fall back to raw JSON for non-string values.
		result = append(result, strings.TrimSpace(string(raw)))
	}
	return result
}
