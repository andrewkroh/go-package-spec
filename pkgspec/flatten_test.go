package pkgspec

import (
	"slices"
	"testing"
)

func TestFlattenFields_NestedGroups(t *testing.T) {
	fields := []Field{
		{
			Name: "test",
			Type: FieldTypeGroup,
			Fields: []Field{
				{Name: "message", Type: FieldTypeText},
				{Name: "level", Type: FieldTypeKeyword},
			},
		},
		{Name: "@timestamp", Type: FieldTypeDate},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"@timestamp", "test.level", "test.message"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFlattenFields_DottedNames(t *testing.T) {
	fields := []Field{
		{Name: "data_stream.type", Type: FieldTypeConstantKeyword},
		{Name: "data_stream.dataset", Type: FieldTypeConstantKeyword},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"data_stream.dataset", "data_stream.type"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFlattenFields_DeeplyNested(t *testing.T) {
	fields := []Field{
		{
			Name: "a",
			Type: FieldTypeGroup,
			Fields: []Field{
				{
					Name: "b",
					Type: FieldTypeGroup,
					Fields: []Field{
						{Name: "c", Type: FieldTypeKeyword},
					},
				},
			},
		},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"a.b.c"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if flat[0].Type != FieldTypeKeyword {
		t.Errorf("got type %q, want %q", flat[0].Type, FieldTypeKeyword)
	}
}

func TestFlattenFields_NonGroupWithChildren(t *testing.T) {
	// A field with type != group that has children should emit both
	// itself and its expanded children.
	fields := []Field{
		{
			Name: "weird",
			Type: FieldTypeObject,
			Fields: []Field{
				{Name: "child", Type: FieldTypeKeyword},
			},
		},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"weird", "weird.child"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFlattenFields_EmptyTypeGroupWithChildren(t *testing.T) {
	// Fields with empty type but children behave like groups.
	fields := []Field{
		{
			Name: "parent",
			Fields: []Field{
				{Name: "leaf", Type: FieldTypeKeyword},
			},
		},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"parent.leaf"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFlattenFields_ECSEnrichment(t *testing.T) {
	fields := []Field{
		{Name: "host.os.name", External: FieldExternalECS},
		{Name: "event.kind", External: FieldExternalECS},
		{Name: "custom.field", Type: FieldTypeKeyword},
	}

	lookup := func(name string) *ECSFieldDefinition {
		defs := map[string]*ECSFieldDefinition{
			"host.os.name": {DataType: "keyword", Description: "Operating system name."},
			"event.kind":   {DataType: "keyword", Description: "The kind of the event."},
		}
		return defs[name]
	}

	flat := FlattenFields(fields, lookup)

	if len(flat) != 3 {
		t.Fatalf("got %d fields, want 3", len(flat))
	}

	// custom.field should have no ECS data.
	customIdx := slices.IndexFunc(flat, func(f FlatField) bool { return f.Name == "custom.field" })
	if customIdx < 0 {
		t.Fatal("missing custom.field")
	}
	if flat[customIdx].ECS != nil {
		t.Error("custom.field should not have ECS data")
	}

	// ECS fields should be enriched.
	hostIdx := slices.IndexFunc(flat, func(f FlatField) bool { return f.Name == "host.os.name" })
	if hostIdx < 0 {
		t.Fatal("missing host.os.name")
	}
	if flat[hostIdx].ECS == nil {
		t.Fatal("host.os.name should have ECS data")
	}
	if flat[hostIdx].ECS.DataType != "keyword" {
		t.Errorf("got ECS DataType %q, want %q", flat[hostIdx].ECS.DataType, "keyword")
	}
	if flat[hostIdx].ECS.Description != "Operating system name." {
		t.Errorf("got ECS Description %q, want %q", flat[hostIdx].ECS.Description, "Operating system name.")
	}
}

func TestFlattenFields_NilLookup(t *testing.T) {
	fields := []Field{
		{Name: "host.os.name", External: FieldExternalECS},
	}

	flat := FlattenFields(fields, nil)

	if len(flat) != 1 {
		t.Fatalf("got %d fields, want 1", len(flat))
	}
	if flat[0].ECS != nil {
		t.Error("ECS should be nil when no lookup provided")
	}
}

func TestFlattenFields_Empty(t *testing.T) {
	flat := FlattenFields(nil, nil)
	if len(flat) != 0 {
		t.Errorf("got %d fields, want 0", len(flat))
	}
}

func TestFlattenFields_PreservesFileMetadata(t *testing.T) {
	fields := []Field{
		{
			Name: "test",
			Type: FieldTypeGroup,
			Fields: []Field{
				{
					Name: "message",
					Type: FieldTypeText,
					FileMetadata: FileMetadata{
						file:   "fields.yml",
						line:   10,
						column: 5,
					},
				},
			},
		},
	}

	flat := FlattenFields(fields, nil)

	if len(flat) != 1 {
		t.Fatalf("got %d fields, want 1", len(flat))
	}
	if flat[0].FilePath() != "fields.yml" {
		t.Errorf("got FilePath %q, want %q", flat[0].FilePath(), "fields.yml")
	}
	if flat[0].Line() != 10 {
		t.Errorf("got Line %d, want %d", flat[0].Line(), 10)
	}
}

func TestFlattenFields_DottedNameInGroup(t *testing.T) {
	// A group with a dotted name, containing children.
	fields := []Field{
		{
			Name: "a.b",
			Type: FieldTypeGroup,
			Fields: []Field{
				{Name: "c.d", Type: FieldTypeKeyword},
			},
		},
	}

	flat := FlattenFields(fields, nil)

	want := []string{"a.b.c.d"}
	got := flatFieldNames(flat)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func flatFieldNames(flat []FlatField) []string {
	names := make([]string, len(flat))
	for i, f := range flat {
		names[i] = f.Name
	}
	return names
}
