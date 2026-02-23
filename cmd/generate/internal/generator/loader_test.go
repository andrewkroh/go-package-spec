package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaType_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want []string
	}{
		{"single string", `"string"`, []string{"string"}},
		{"array", `["string","null"]`, []string{"string", "null"}},
		{"object", `"object"`, []string{"object"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var st SchemaType
			if err := st.UnmarshalJSON([]byte(tt.json)); err != nil {
				t.Fatal(err)
			}
			if len(st.Values()) != len(tt.want) {
				t.Fatalf("got %v, want %v", st.Values(), tt.want)
			}
			for i, v := range st.Values() {
				if v != tt.want[i] {
					t.Errorf("value[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestAdditionalProperties_UnmarshalJSON(t *testing.T) {
	t.Run("false", func(t *testing.T) {
		var ap AdditionalProperties
		if err := ap.UnmarshalJSON([]byte("false")); err != nil {
			t.Fatal(err)
		}
		if !ap.IsFalse() {
			t.Error("expected IsFalse() to be true")
		}
	})

	t.Run("true", func(t *testing.T) {
		var ap AdditionalProperties
		if err := ap.UnmarshalJSON([]byte("true")); err != nil {
			t.Fatal(err)
		}
		if ap.IsFalse() {
			t.Error("expected IsFalse() to be false")
		}
	})

	t.Run("schema", func(t *testing.T) {
		var ap AdditionalProperties
		if err := ap.UnmarshalJSON([]byte(`{"type":"string"}`)); err != nil {
			t.Fatal(err)
		}
		if ap.Schema == nil {
			t.Fatal("expected Schema to be non-nil")
		}
		if ap.Schema.Type.Single() != "string" {
			t.Errorf("got type %q, want %q", ap.Schema.Type.Single(), "string")
		}
	})
}

func TestSchemaRegistry_LoadSchema(t *testing.T) {
	dir := setupTestSchemas(t)

	reg := NewSchemaRegistry(dir)
	s, err := reg.LoadSchema("simple.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.Type.Single() != "object" {
		t.Errorf("got type %q, want %q", s.Type.Single(), "object")
	}
	if len(s.Properties) != 2 {
		t.Errorf("got %d properties, want 2", len(s.Properties))
	}

	// Loading again should return same pointer (cached).
	s2, err := reg.LoadSchema("simple.json")
	if err != nil {
		t.Fatal(err)
	}
	if s != s2 {
		t.Error("expected cached schema to be the same pointer")
	}
}

func TestSchemaRegistry_ResolveRef(t *testing.T) {
	dir := setupTestSchemas(t)

	reg := NewSchemaRegistry(dir)

	t.Run("same-file definition", func(t *testing.T) {
		s, file, err := reg.ResolveRef("#/definitions/person", "with_defs.json")
		if err != nil {
			t.Fatal(err)
		}
		if file != "with_defs.json" {
			t.Errorf("got file %q, want %q", file, "with_defs.json")
		}
		if s.Type.Single() != "object" {
			t.Errorf("got type %q, want object", s.Type.Single())
		}
		if _, ok := s.Properties["name"]; !ok {
			t.Error("expected property 'name' in person definition")
		}
	})

	t.Run("self-reference #", func(t *testing.T) {
		s, file, err := reg.ResolveRef("#", "simple.json")
		if err != nil {
			t.Fatal(err)
		}
		if file != "simple.json" {
			t.Errorf("got file %q, want %q", file, "simple.json")
		}
		if s.Type.Single() != "object" {
			t.Errorf("got type %q, want object", s.Type.Single())
		}
	})

	t.Run("cross-file ref", func(t *testing.T) {
		s, file, err := reg.ResolveRef("./with_defs.json#/definitions/person", "simple.json")
		if err != nil {
			t.Fatal(err)
		}
		if file != "with_defs.json" {
			t.Errorf("got file %q, want %q", file, "with_defs.json")
		}
		if _, ok := s.Properties["name"]; !ok {
			t.Error("expected property 'name'")
		}
	})

	t.Run("cross-file nested", func(t *testing.T) {
		s, file, err := reg.ResolveRef("../sub/nested.json#/definitions/item", "sub/nested.json")
		if err != nil {
			t.Fatal(err)
		}
		if file != "sub/nested.json" {
			t.Errorf("got file %q, want %q", file, "sub/nested.json")
		}
		if s.Type.Single() != "object" {
			t.Errorf("got type %q, want object", s.Type.Single())
		}
	})

	t.Run("deep path", func(t *testing.T) {
		s, _, err := reg.ResolveRef("#/definitions/person/properties/name", "with_defs.json")
		if err != nil {
			t.Fatal(err)
		}
		if s.Type.Single() != "string" {
			t.Errorf("got type %q, want string", s.Type.Single())
		}
	})
}

func TestSchemaRegistry_LoadRealSchemas(t *testing.T) {
	schemaDir := filepath.Join("..", "..", "..", "package-spec-schema", "3.5.7", "jsonschema")
	if _, err := os.Stat(schemaDir); os.IsNotExist(err) {
		t.Skip("package-spec-schema not available")
	}

	reg := NewSchemaRegistry(schemaDir)

	// Load integration manifest.
	s, err := reg.LoadSchema("integration/manifest.jsonschema.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.Type.Single() != "object" {
		t.Errorf("integration manifest type = %q, want object", s.Type.Single())
	}

	// Resolve cross-file ref to data_stream vars.
	vars, file, err := reg.ResolveRef(
		"./data_stream/manifest.jsonschema.json#/definitions/vars",
		"integration/manifest.jsonschema.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if file != "integration/data_stream/manifest.jsonschema.json" {
		t.Errorf("vars file = %q, want integration/data_stream/manifest.jsonschema.json", file)
	}
	if vars.Type.Single() != "array" {
		t.Errorf("vars type = %q, want array", vars.Type.Single())
	}

	// Resolve version ref from changelog.
	version, _, err := reg.ResolveRef(
		"./manifest.jsonschema.json#/definitions/version",
		"integration/changelog.jsonschema.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if version.Type.Single() != "string" {
		t.Errorf("version type = %q, want string", version.Type.Single())
	}

	// Resolve self-ref in fields schema.
	fields, _, err := reg.ResolveRef(
		"#",
		"integration/data_stream/fields/fields.jsonschema.json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if fields.Type.Single() != "array" {
		t.Errorf("fields type = %q, want array", fields.Type.Single())
	}

	// Resolve root manifest $defs.
	rootManifest, err := reg.LoadSchema("manifest.jsonschema.json")
	if err != nil {
		t.Fatal(err)
	}
	if rootManifest.Defs == nil {
		t.Fatal("expected $defs in root manifest")
	}

	// Resolve transform manifest deep ref.
	_, _, err = reg.ResolveRef(
		"../../data_stream/manifest.jsonschema.json#/definitions/elasticsearch_index_template/properties/mappings",
		"integration/elasticsearch/transform/manifest.jsonschema.json",
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnumStrings(t *testing.T) {
	dir := setupTestSchemas(t)
	reg := NewSchemaRegistry(dir)

	s, err := reg.LoadSchema("with_defs.json")
	if err != nil {
		t.Fatal(err)
	}
	colorProp := s.Definitions["person"].Properties["color"]
	if colorProp == nil {
		t.Fatal("expected color property")
	}
	enums := colorProp.EnumStrings()
	want := []string{"red", "green", "blue"}
	if len(enums) != len(want) {
		t.Fatalf("got %v, want %v", enums, want)
	}
	for i := range enums {
		if enums[i] != want[i] {
			t.Errorf("enum[%d] = %q, want %q", i, enums[i], want[i])
		}
	}
}

func TestSplitRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantFile string
		wantFrag string
	}{
		{"#/definitions/foo", "", "/definitions/foo"},
		{"./other.json#/definitions/x", "./other.json", "/definitions/x"},
		{"#", "", ""},
		{"./other.json", "./other.json", ""},
		{"../../foo.json#/definitions/bar", "../../foo.json", "/definitions/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			file, frag := splitRef(tt.ref)
			if file != tt.wantFile {
				t.Errorf("file = %q, want %q", file, tt.wantFile)
			}
			if frag != tt.wantFrag {
				t.Errorf("fragment = %q, want %q", frag, tt.wantFrag)
			}
		})
	}
}

// setupTestSchemas creates temporary test schema files and returns
// the directory path.
func setupTestSchemas(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	simple := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"}
		},
		"required": ["name"]
	}`
	writeFile(t, dir, "simple.json", simple)

	withDefs := `{
		"type": "object",
		"properties": {
			"person": {"$ref": "#/definitions/person"}
		},
		"definitions": {
			"person": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"color": {
						"type": "string",
						"enum": ["red", "green", "blue"]
					}
				}
			}
		}
	}`
	writeFile(t, dir, "with_defs.json", withDefs)

	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := `{
		"type": "object",
		"definitions": {
			"item": {
				"type": "object",
				"properties": {
					"value": {"type": "string"}
				}
			}
		}
	}`
	writeFile(t, dir, "sub/nested.json", nested)

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
