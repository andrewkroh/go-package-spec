package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypeMapper_SimpleObject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "simple.json", `{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string", "description": "The name"},
			"age": {"type": "integer"},
			"active": {"type": "boolean"}
		}
	}`)

	reg := NewSchemaRegistry(dir)
	mapper := NewTypeMapper(reg)
	mapper.RegisterEntryPoint("simple.json", "Simple")

	if err := mapper.ProcessEntryPoint("simple.json"); err != nil {
		t.Fatal(err)
	}

	types := mapper.Types()
	if len(types) != 1 {
		t.Fatalf("got %d types, want 1", len(types))
	}

	st := types[0]
	if st.Name != "Simple" {
		t.Errorf("name = %q, want Simple", st.Name)
	}
	if st.Kind != GoTypeStruct {
		t.Errorf("kind = %v, want GoTypeStruct", st.Kind)
	}
	if len(st.Fields) != 3 {
		t.Fatalf("got %d fields, want 3", len(st.Fields))
	}

	// Check fields by name.
	fieldMap := make(map[string]GoField)
	for _, f := range st.Fields {
		fieldMap[f.JSONName] = f
	}

	nameField := fieldMap["name"]
	if nameField.Type.Builtin != "string" {
		t.Errorf("name type = %v, want string", nameField.Type)
	}
	if !nameField.Required {
		t.Error("name should be required")
	}

	ageField := fieldMap["age"]
	if ageField.Type.Builtin != "int" {
		t.Errorf("age type = %v, want int", ageField.Type)
	}

	activeField := fieldMap["active"]
	if !activeField.Type.Pointer {
		t.Error("optional boolean should be a pointer")
	}
}

func TestTypeMapper_Enum(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "enum.json", `{
		"type": "object",
		"properties": {
			"color": {
				"type": "string",
				"enum": ["red", "green", "blue"],
				"description": "The color."
			}
		}
	}`)

	reg := NewSchemaRegistry(dir)
	mapper := NewTypeMapper(reg)
	mapper.RegisterEntryPoint("enum.json", "Item")

	if err := mapper.ProcessEntryPoint("enum.json"); err != nil {
		t.Fatal(err)
	}

	types := mapper.Types()
	// Should have Item struct + ItemColor enum.
	enumFound := false
	for _, tp := range types {
		if tp.Kind == GoTypeEnum {
			enumFound = true
			if len(tp.EnumValues) != 3 {
				t.Errorf("got %d enum values, want 3", len(tp.EnumValues))
			}
		}
	}
	if !enumFound {
		t.Error("expected an enum type")
	}
}

func TestTypeMapper_Array(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "array.json", `{
		"type": "array",
		"items": {
			"type": "object",
			"required": ["name"],
			"properties": {
				"name": {"type": "string"}
			}
		}
	}`)

	reg := NewSchemaRegistry(dir)
	mapper := NewTypeMapper(reg)
	mapper.RegisterEntryPoint("array.json", "Items")

	if err := mapper.ProcessEntryPoint("array.json"); err != nil {
		t.Fatal(err)
	}

	types := mapper.Types()
	if len(types) != 1 {
		t.Fatalf("got %d types, want 1 (the element type)", len(types))
	}
}

func TestTypeMapper_Ref(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.json", `{
		"type": "object",
		"properties": {
			"owner": {"$ref": "#/definitions/owner"}
		},
		"definitions": {
			"owner": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"type": {
						"type": "string",
						"enum": ["elastic", "partner", "community"]
					}
				}
			}
		}
	}`)

	reg := NewSchemaRegistry(dir)
	mapper := NewTypeMapper(reg)
	mapper.RegisterEntryPoint("main.json", "Manifest")

	if err := mapper.ProcessEntryPoint("main.json"); err != nil {
		t.Fatal(err)
	}

	types := mapper.TypesByName()
	// The definition "owner" should produce type "Owner".
	if _, ok := types["Owner"]; !ok {
		t.Error("expected Owner type")
		for name := range types {
			t.Logf("  have type: %s", name)
		}
	}
	// The Owner's "type" field with enum should produce an enum type.
	foundEnum := false
	for _, tp := range mapper.Types() {
		if tp.Kind == GoTypeEnum {
			foundEnum = true
		}
	}
	if !foundEnum {
		t.Error("expected an enum type for owner.type")
	}
}

func TestTypeMapper_RealSchemas(t *testing.T) {
	schemaDir := filepath.Join("..", "..", "..", "package-spec-schema", "3.5.7", "jsonschema")
	if _, err := os.Stat(schemaDir); os.IsNotExist(err) {
		t.Skip("package-spec-schema not available")
	}

	reg := NewSchemaRegistry(schemaDir)
	mapper := NewTypeMapper(reg)

	// Register entry points.
	mapper.RegisterEntryPoint("integration/manifest.jsonschema.json", "IntegrationManifest")
	mapper.RegisterEntryPoint("input/manifest.jsonschema.json", "InputManifest")
	mapper.RegisterEntryPoint("content/manifest.jsonschema.json", "ContentManifest")
	mapper.RegisterEntryPoint("integration/data_stream/manifest.jsonschema.json", "DataStreamManifest")
	mapper.RegisterEntryPoint("integration/data_stream/fields/fields.jsonschema.json", "Fields")
	mapper.RegisterEntryPoint("integration/changelog.jsonschema.json", "Changelog")
	mapper.RegisterEntryPoint("integration/validation.jsonschema.json", "Validation")
	mapper.RegisterEntryPoint("integration/elasticsearch/transform/manifest.jsonschema.json", "TransformManifest")
	mapper.RegisterEntryPoint("integration/elasticsearch/transform/transform.jsonschema.json", "Transform")
	mapper.RegisterEntryPoint("integration/kibana/tags.jsonschema.json", "Tags")
	mapper.RegisterEntryPoint("integration/data_stream/lifecycle.jsonschema.json", "Lifecycle")

	// Process all entry points.
	entryPoints := []string{
		"integration/manifest.jsonschema.json",
		"input/manifest.jsonschema.json",
		"content/manifest.jsonschema.json",
		"integration/data_stream/manifest.jsonschema.json",
		"integration/data_stream/fields/fields.jsonschema.json",
		"integration/changelog.jsonschema.json",
		"integration/validation.jsonschema.json",
		"integration/elasticsearch/transform/manifest.jsonschema.json",
		"integration/elasticsearch/transform/transform.jsonschema.json",
		"integration/kibana/tags.jsonschema.json",
		"integration/data_stream/lifecycle.jsonschema.json",
	}

	for _, ep := range entryPoints {
		if err := mapper.ProcessEntryPoint(ep); err != nil {
			t.Fatalf("processing %s: %v", ep, err)
		}
	}

	types := mapper.Types()
	t.Logf("Generated %d types:", len(types))
	for _, tp := range types {
		var kind string
		switch tp.Kind {
		case GoTypeStruct:
			kind = "struct"
		case GoTypeEnum:
			kind = "enum"
		case GoTypeAlias:
			kind = "alias"
		case GoTypeMap:
			kind = "map"
		}
		fields := ""
		if tp.Kind == GoTypeStruct && len(tp.Fields) > 0 {
			var fieldNames []string
			for _, f := range tp.Fields {
				fieldNames = append(fieldNames, f.Name)
			}
			fields = " {" + strings.Join(fieldNames, ", ") + "}"
		}
		if tp.Kind == GoTypeEnum {
			var vals []string
			for _, v := range tp.EnumValues {
				vals = append(vals, v.Value)
			}
			fields = " [" + strings.Join(vals, ", ") + "]"
		}
		t.Logf("  %s (%s, embed=%v)%s", tp.Name, kind, tp.EmbedMeta, fields)
	}

	// Verify expected types exist.
	expected := []string{
		"IntegrationManifest",
		"DataStreamManifest",
		"Owner",
		"Conditions",
		"Deprecated",
		"Validation",
		"Lifecycle",
	}
	byName := mapper.TypesByName()
	for _, name := range expected {
		if _, ok := byName[name]; !ok {
			t.Errorf("expected type %q not found", name)
		}
	}
}

func TestSingularize(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Items", "Item"},
		{"Categories", "Category"},
		{"Fields", "Field"},
		{"Changes", "Change"},
		{"Addresses", "Address"},
		{"Status", "Status"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := singularize(tt.input)
			if got != tt.want {
				t.Errorf("singularize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
