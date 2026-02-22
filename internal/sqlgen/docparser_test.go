package sqlgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanComment(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple",
			input: "// Description of the field.\n",
			want:  "Description of the field.",
		},
		{
			name:  "multiline",
			input: "// First line\n// second line.\n",
			want:  "First line second line.",
		},
		{
			name:  "with link ref",
			input: "// Description of a feature.\n// [Some Link]: https://example.com\n",
			want:  "Description of a feature.",
		},
		{
			name:  "truncates long comment",
			input: "// " + strings.Repeat("x", 250) + "\n",
			want:  strings.Repeat("x", 197) + "...",
		},
		{
			name:  "empty",
			input: "//\n",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a minimal Go source with the comment to parse.
			src := "package test\n\ntype T struct {\n" + tt.input + "\tF int\n}\n"
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			docs, err := ParseDocComments([]string{dir})
			if err != nil {
				t.Fatal(err)
			}
			got := docs["T.F"]
			if got != tt.want {
				t.Errorf("cleanComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDocComments(t *testing.T) {
	src := `package example

// MyStruct is a test type.
type MyStruct struct {
	// Name is the display name.
	Name string
	// Count tracks the number of items.
	Count int
	// unexported fields are skipped.
	hidden bool
	NoDoc string
}

type Other struct {
	// Value is a value.
	Value string
}
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	docs, err := ParseDocComments([]string{dir})
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"MyStruct.Name":  "Name is the display name.",
		"MyStruct.Count": "Count tracks the number of items.",
		"Other.Value":    "Value is a value.",
	}

	for key, wantVal := range want {
		if got := docs[key]; got != wantVal {
			t.Errorf("docs[%q] = %q, want %q", key, got, wantVal)
		}
	}

	// Should not contain entries for unexported or undocumented fields.
	for _, key := range []string{"MyStruct.hidden", "MyStruct.NoDoc"} {
		if _, ok := docs[key]; ok {
			t.Errorf("docs should not contain %q", key)
		}
	}
}

func TestFindModuleRoot(t *testing.T) {
	// Should find the go.mod for this project.
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	// Verify go.mod exists at the root.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("go.mod not found at %s: %v", root, err)
	}
}

func TestReadModulePath(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	mod, err := readModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	if mod != "github.com/andrewkroh/go-package-spec" {
		t.Fatalf("unexpected module path: %s", mod)
	}
}

func TestSourceDirsFromRegistry(t *testing.T) {
	root, err := findModuleRoot(".")
	if err != nil {
		t.Fatal(err)
	}
	dirs, err := SourceDirsFromRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("expected at least one source dir")
	}
	// Verify each directory exists.
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("source dir %s does not exist: %v", d, err)
		}
	}
}

func TestRegisteredPkgPaths(t *testing.T) {
	paths := RegisteredPkgPaths()
	if len(paths) == 0 {
		t.Fatal("expected at least one registered package path")
	}
	// Should contain pkgspec at minimum.
	found := false
	for _, p := range paths {
		if strings.HasSuffix(p, "/pkgspec") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected pkgspec in registered paths: %v", paths)
	}
}
