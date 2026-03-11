package pkgspec

import (
	"encoding/json"
	"testing"

	yamlv3 "gopkg.in/yaml.v3"
)

func TestField_Extras_CapturesUnknownAttributes(t *testing.T) {
	input := `
name: message
type: text
norms: true
title: Message Field
footnote: some note
`
	var f Field
	if err := yamlv3.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.Name != "message" {
		t.Errorf("got Name %q, want %q", f.Name, "message")
	}
	if f.Type != FieldTypeText {
		t.Errorf("got Type %q, want %q", f.Type, FieldTypeText)
	}

	if len(f.Extras) != 3 {
		t.Fatalf("got %d extras, want 3: %v", len(f.Extras), f.Extras)
	}

	want := map[string]any{
		"norms":    true,
		"title":    "Message Field",
		"footnote": "some note",
	}
	for k, wantVal := range want {
		gotVal, ok := f.Extras[k]
		if !ok {
			t.Errorf("missing extra %q", k)
			continue
		}
		if gotVal != wantVal {
			t.Errorf("extra %q: got %v (%T), want %v (%T)", k, gotVal, gotVal, wantVal, wantVal)
		}
	}
}

func TestField_Extras_NilWhenNoUnknownAttributes(t *testing.T) {
	input := `
name: message
type: text
description: A message field.
`
	var f Field
	if err := yamlv3.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.Extras != nil {
		t.Errorf("got Extras %v, want nil", f.Extras)
	}
}

func TestField_Extras_ExcludedFromJSON(t *testing.T) {
	f := Field{
		Name: "test",
		Type: FieldTypeKeyword,
		Extras: map[string]any{
			"norms": false,
			"title": "Test",
		},
	}

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal JSON: %v", err)
	}

	if _, ok := m["norms"]; ok {
		t.Error("Extras key 'norms' should not appear in JSON output")
	}
	if _, ok := m["title"]; ok {
		t.Error("Extras key 'title' should not appear in JSON output")
	}
}

func TestField_Extras_NestedFields(t *testing.T) {
	input := `
name: parent
type: group
fields:
  - name: child
    type: keyword
    default_field: false
`
	var f Field
	if err := yamlv3.Unmarshal([]byte(input), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if f.Extras != nil {
		t.Errorf("parent should have no extras, got %v", f.Extras)
	}

	if len(f.Fields) != 1 {
		t.Fatalf("got %d children, want 1", len(f.Fields))
	}

	child := f.Fields[0]
	if child.Extras == nil {
		t.Fatal("child should have extras")
	}
	if got, ok := child.Extras["default_field"]; !ok || got != false {
		t.Errorf("child extras[default_field]: got %v, want false", got)
	}
}
