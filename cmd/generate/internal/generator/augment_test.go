package generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseTypeRef(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"any", "any"},
		{"string", "string"},
		{"int", "int"},
		{"bool", "bool"},
		{"*bool", "*bool"},
		{"[]string", "[]string"},
		{"map[string]any", "map[string]any"},
		{"map[string][]string", "map[string][]string"},
		{"*MyType", "*MyType"},
		{"[]MyType", "[]MyType"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ref := parseTypeRef(tt.input)
			got := ref.String()
			if got != tt.want {
				t.Errorf("parseTypeRef(%q).String() = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadAndApplyAugmentations(t *testing.T) {
	dir := t.TempDir()
	augmentYML := `types:
  MyType:
    doc: "Updated doc."
    fields:
      name:
        doc: "The name field."
      value:
        type: any
  OldName:
    name: NewName
`
	if err := os.WriteFile(filepath.Join(dir, "augment.yml"), []byte(augmentYML), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := LoadAugmentations(filepath.Join(dir, "augment.yml"))
	if err != nil {
		t.Fatal(err)
	}

	types := map[string]*GoType{
		"MyType": {
			Name: "MyType",
			Doc:  "Original doc.",
			Kind: GoTypeStruct,
			Fields: []GoField{
				{Name: "Name", JSONName: "name", Type: GoTypeRef{Builtin: "string"}},
				{Name: "Value", JSONName: "value", Type: GoTypeRef{Builtin: "string"}},
			},
		},
		"OldName": {
			Name: "OldName",
			Kind: GoTypeStruct,
		},
		"Referrer": {
			Name: "Referrer",
			Kind: GoTypeStruct,
			Fields: []GoField{
				{Name: "Ref", JSONName: "ref", Type: GoTypeRef{Named: "OldName"}},
			},
		},
	}

	ApplyAugmentations(types, config)

	// Check doc update.
	if types["MyType"].Doc != "Updated doc." {
		t.Errorf("doc = %q, want %q", types["MyType"].Doc, "Updated doc.")
	}

	// Check field doc.
	if types["MyType"].Fields[0].Doc != "The name field." {
		t.Errorf("field doc = %q, want %q", types["MyType"].Fields[0].Doc, "The name field.")
	}

	// Check type override.
	if types["MyType"].Fields[1].Type.Builtin != "any" {
		t.Errorf("field type = %v, want any", types["MyType"].Fields[1].Type)
	}

	// Check rename.
	if _, ok := types["OldName"]; ok {
		t.Error("OldName should have been renamed")
	}
	if _, ok := types["NewName"]; !ok {
		t.Error("NewName should exist")
	}

	// Check ref update.
	if types["Referrer"].Fields[0].Type.Named != "NewName" {
		t.Errorf("ref = %q, want NewName", types["Referrer"].Fields[0].Type.Named)
	}
}
