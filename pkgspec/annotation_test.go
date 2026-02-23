package pkgspec

import "testing"

func TestAnnotateFieldPointers(t *testing.T) {
	fields := []Field{
		{
			Name: "@timestamp",
			Type: FieldTypeDate,
		},
		{
			Name: "log",
			Type: FieldTypeGroup,
			Fields: []Field{
				{Name: "level", Type: FieldTypeKeyword},
				{
					Name: "source",
					Type: FieldTypeGroup,
					Fields: []Field{
						{Name: "file", Type: FieldTypeKeyword},
					},
				},
			},
		},
	}

	AnnotateFieldPointers(fields)

	tests := []struct {
		name    string
		pointer string
	}{
		{"@timestamp", "/0"},
		{"log", "/1"},
		{"level", "/1/fields/0"},
		{"source", "/1/fields/1"},
		{"file", "/1/fields/1/fields/0"},
	}

	// Flatten for easy lookup.
	got := map[string]string{
		fields[0].Name:                     fields[0].JsonPointer,
		fields[1].Name:                     fields[1].JsonPointer,
		fields[1].Fields[0].Name:           fields[1].Fields[0].JsonPointer,
		fields[1].Fields[1].Name:           fields[1].Fields[1].JsonPointer,
		fields[1].Fields[1].Fields[0].Name: fields[1].Fields[1].Fields[0].JsonPointer,
	}

	for _, tt := range tests {
		if got[tt.name] != tt.pointer {
			t.Errorf("field %q: got pointer %q, want %q", tt.name, got[tt.name], tt.pointer)
		}
	}
}

func TestAnnotateFieldPointers_Empty(t *testing.T) {
	// Should not panic on nil/empty input.
	AnnotateFieldPointers(nil)
	AnnotateFieldPointers([]Field{})
}
