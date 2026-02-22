package pkgspec

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestUnmarshalIntegrationManifest(t *testing.T) {
	manifestPath := filepath.Join("..", "..", "integrations", "packages", "1password", "manifest.yml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Skip("integrations/packages/1password not available")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	var manifest IntegrationManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if manifest.Name != "1password" {
		t.Errorf("name = %q, want 1password", manifest.Name)
	}
	if manifest.Title != "1Password" {
		t.Errorf("title = %q, want 1Password", manifest.Title)
	}
	if manifest.Type != "integration" {
		t.Errorf("type = %q, want integration", manifest.Type)
	}
	if manifest.Version == "" {
		t.Error("version is empty")
	}
	if manifest.FormatVersion == "" {
		t.Error("format_version is empty")
	}
	if len(manifest.Categories) == 0 {
		t.Error("categories is empty")
	}
	if manifest.Owner.Github == "" {
		t.Error("owner.github is empty")
	}
	if len(manifest.PolicyTemplates) == 0 {
		t.Error("policy_templates is empty")
	}

	t.Logf("Loaded: %s v%s (format %s)", manifest.Name, manifest.Version, manifest.FormatVersion)
	t.Logf("  Categories: %v", manifest.Categories)
	t.Logf("  Owner: %s (type=%s)", manifest.Owner.Github, manifest.Owner.Type)
	t.Logf("  Policy templates: %d", len(manifest.PolicyTemplates))
}

func TestUnmarshalChangelog(t *testing.T) {
	changelogPath := filepath.Join("..", "..", "integrations", "packages", "1password", "changelog.yml")
	if _, err := os.Stat(changelogPath); os.IsNotExist(err) {
		t.Skip("integrations/packages/1password not available")
	}

	data, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatal(err)
	}

	var changelog []Changelog
	if err := yaml.Unmarshal(data, &changelog); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(changelog) == 0 {
		t.Fatal("changelog is empty")
	}

	first := changelog[0]
	if first.Version == "" {
		t.Error("first version is empty")
	}
	if len(first.Changes) == 0 {
		t.Error("first changes is empty")
	}

	t.Logf("Changelog has %d releases, latest: %s", len(changelog), first.Version)
}

func TestUnmarshalFields(t *testing.T) {
	fieldsPath := filepath.Join("..", "..", "integrations", "packages", "1password", "data_stream", "signin_attempts", "fields", "fields.yml")
	if _, err := os.Stat(fieldsPath); os.IsNotExist(err) {
		t.Skip("1password fields not available")
	}

	data, err := os.ReadFile(fieldsPath)
	if err != nil {
		t.Fatal(err)
	}

	var fields []Field
	if err := yaml.Unmarshal(data, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(fields) == 0 {
		t.Fatal("fields is empty")
	}

	t.Logf("Loaded %d fields", len(fields))
	for _, f := range fields {
		t.Logf("  %s (type=%s)", f.Name, f.Type)
	}
}

func TestUnmarshalValidation(t *testing.T) {
	validationPath := filepath.Join("..", "..", "integrations", "packages", "1password", "validation.yml")
	if _, err := os.Stat(validationPath); os.IsNotExist(err) {
		t.Skip("1password validation not available")
	}

	data, err := os.ReadFile(validationPath)
	if err != nil {
		t.Fatal(err)
	}

	var validation Validation
	if err := yaml.Unmarshal(data, &validation); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	t.Logf("Validation: %+v", validation)
}

func TestFileMetadata(t *testing.T) {
	yamlData := `name: test_field
type: keyword
description: A test field.
`
	var field Field
	if err := yaml.Unmarshal([]byte(yamlData), &field); err != nil {
		t.Fatal(err)
	}

	if field.Name != "test_field" {
		t.Errorf("name = %q, want test_field", field.Name)
	}

	// FileMetadata line/column should be set by UnmarshalYAML.
	if field.Line() != 1 {
		t.Errorf("line = %d, want 1", field.Line())
	}
	if field.Column() != 1 {
		t.Errorf("column = %d, want 1", field.Column())
	}

	// annotateFileMetadata should set the file path.
	annotateFileMetadata("test.yml", &field)
	if field.FilePath() != "test.yml" {
		t.Errorf("path = %q, want test.yml", field.FilePath())
	}
}
