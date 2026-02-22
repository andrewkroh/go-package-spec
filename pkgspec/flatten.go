package pkgspec

import (
	"slices"
	"strings"
)

// ECSFieldDefinition contains ECS field metadata resolved from an external
// reference. Callers populate this via the ecsLookup callback passed to
// [FlattenFields].
type ECSFieldDefinition struct {
	DataType    string
	Description string
	Pattern     string
	Array       bool
}

// FlatField represents a field with its fully-qualified dotted name.
// For fields with [FieldExternalECS], the ECS definition is attached
// when an ecsLookup function is provided to [FlattenFields].
type FlatField struct {
	Field

	// ECS contains the resolved ECS field definition when the field
	// declares external: ecs and the lookup succeeds. Otherwise nil.
	ECS *ECSFieldDefinition
}

// FlattenFields returns a flat, sorted slice of fields with dot-joined names.
// Nested group fields are expanded; the group parents themselves are omitted.
// Non-group fields that have children are included alongside their expanded
// children (this handles unusual but real-world definitions).
//
// If ecsLookup is non-nil, fields with External == "ecs" are enriched by
// calling ecsLookup with the flattened field name. This allows callers to
// plug in any ECS version without adding a direct dependency:
//
//	flat := pkgspec.FlattenFields(fields, func(name string) *pkgspec.ECSFieldDefinition {
//	    f, err := ecs.Lookup(name, "8.17")
//	    if err != nil {
//	        return nil
//	    }
//	    return &pkgspec.ECSFieldDefinition{
//	        DataType:    f.DataType,
//	        Description: f.Description,
//	        Pattern:     f.Pattern,
//	        Array:       f.Array,
//	    }
//	})
func FlattenFields(fields []Field, ecsLookup func(name string) *ECSFieldDefinition) []FlatField {
	var flat []FlatField
	for _, f := range fields {
		flat = append(flat, flattenField(nil, f)...)
	}

	// Enrich ECS fields.
	if ecsLookup != nil {
		for i := range flat {
			if flat[i].External == FieldExternalECS {
				flat[i].ECS = ecsLookup(flat[i].Name)
			}
		}
	}

	slices.SortFunc(flat, func(a, b FlatField) int {
		return strings.Compare(a.Name, b.Name)
	})

	return flat
}

func flattenField(key []string, f Field) []FlatField {
	leafName := strings.Split(f.Name, ".")

	// Leaf node — no children.
	if len(f.Fields) == 0 {
		name := make([]string, len(key)+len(leafName))
		copy(name, key)
		copy(name[len(key):], leafName)

		f.Name = strings.Join(name, ".")
		return []FlatField{{Field: f}}
	}

	// Recurse into children.
	parentName := append(key, leafName...)
	var flat []FlatField
	for _, child := range f.Fields {
		flat = append(flat, flattenField(parentName, child)...)
	}

	// Non-group fields with children: also emit the parent itself.
	if f.Type != "" && f.Type != FieldTypeGroup {
		parent := f
		parent.Name = strings.Join(parentName, ".")
		parent.Fields = nil
		flat = append(flat, FlatField{Field: parent})
	}

	return flat
}
