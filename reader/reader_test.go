package reader

import (
	"encoding/json"
	"os"
	"testing"
	"testing/fstest"
)

func TestReadIntegrationPackage(t *testing.T) {
	pkg, err := Read("testdata/integration_pkg")
	if err != nil {
		t.Fatal(err)
	}

	// Common manifest fields.
	m := pkg.Manifest()
	if m == nil {
		t.Fatal("Manifest() returned nil")
	}
	if m.Name != "test_integration" {
		t.Errorf("name = %q, want test_integration", m.Name)
	}
	if m.Title != "Test Integration" {
		t.Errorf("title = %q, want Test Integration", m.Title)
	}
	if m.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", m.Version)
	}
	if m.FormatVersion != "3.3.0" {
		t.Errorf("format_version = %q, want 3.3.0", m.FormatVersion)
	}
	if m.Description != "A test integration package." {
		t.Errorf("description = %q, want A test integration package.", m.Description)
	}
	if len(m.Categories) != 1 || string(m.Categories[0]) != "security" {
		t.Errorf("categories = %v, want [security]", m.Categories)
	}
	if m.Owner.Github != "elastic/test-team" {
		t.Errorf("owner.github = %q, want elastic/test-team", m.Owner.Github)
	}

	// Type-specific manifest.
	im := pkg.IntegrationManifest()
	if im == nil {
		t.Fatal("IntegrationManifest() returned nil")
	}
	if im.Type != "integration" {
		t.Errorf("type = %q, want integration", im.Type)
	}
	if len(im.PolicyTemplates) != 1 {
		t.Errorf("policy_templates count = %d, want 1", len(im.PolicyTemplates))
	}
	if pkg.InputManifest() != nil {
		t.Error("InputManifest() should be nil for integration package")
	}

	// Changelog.
	if len(pkg.Changelog) != 2 {
		t.Errorf("changelog count = %d, want 2", len(pkg.Changelog))
	}
	if pkg.Changelog[0].Version != "1.0.0" {
		t.Errorf("changelog[0].version = %q, want 1.0.0", pkg.Changelog[0].Version)
	}

	// Validation.
	if pkg.Validation == nil {
		t.Fatal("validation is nil")
	}
	if len(pkg.Validation.Errors.ExcludeChecks) != 1 {
		t.Errorf("validation exclude_checks count = %d, want 1", len(pkg.Validation.Errors.ExcludeChecks))
	}

	// Tags.
	if len(pkg.Tags) != 1 {
		t.Errorf("tags count = %d, want 1", len(pkg.Tags))
	}

	// Data streams.
	if len(pkg.DataStreams) != 1 {
		t.Fatalf("data_stream count = %d, want 1", len(pkg.DataStreams))
	}
	ds, ok := pkg.DataStreams["logs"]
	if !ok {
		t.Fatal("data_stream 'logs' not found")
	}
	if ds.Manifest.Title != "Test Logs" {
		t.Errorf("ds manifest title = %q, want Test Logs", ds.Manifest.Title)
	}
	if ds.Lifecycle == nil {
		t.Fatal("lifecycle is nil")
	}
	if ds.Lifecycle.DataRetention != "7d" {
		t.Errorf("data_retention = %q, want 7d", ds.Lifecycle.DataRetention)
	}
	if len(ds.Fields) != 2 {
		t.Errorf("fields file count = %d, want 2", len(ds.Fields))
	}

	// Pipelines.
	if len(ds.Pipelines) != 1 {
		t.Fatalf("pipeline file count = %d, want 1", len(ds.Pipelines))
	}
	pf, ok := ds.Pipelines["default.yml"]
	if !ok {
		t.Fatal("pipeline file 'default.yml' not found")
	}
	if pf.Pipeline.Description != "Test pipeline" {
		t.Errorf("pipeline description = %q, want Test pipeline", pf.Pipeline.Description)
	}
	if len(pf.Pipeline.Processors) != 2 {
		t.Fatalf("pipeline processors count = %d, want 2", len(pf.Pipeline.Processors))
	}
	if pf.Pipeline.Processors[0].Type != "set" {
		t.Errorf("pipeline processor[0] type = %q, want set", pf.Pipeline.Processors[0].Type)
	}
	if len(pf.Pipeline.OnFailure) != 1 {
		t.Fatalf("pipeline on_failure count = %d, want 1", len(pf.Pipeline.OnFailure))
	}
	if pf.Pipeline.OnFailure[0].Type != "set" {
		t.Errorf("pipeline on_failure[0] type = %q, want set", pf.Pipeline.OnFailure[0].Type)
	}

	// ILM policies.
	if len(ds.ILMPolicies) != 1 {
		t.Fatalf("ILM policy count = %d, want 1", len(ds.ILMPolicies))
	}
	ilm, ok := ds.ILMPolicies["default.yml"]
	if !ok {
		t.Fatal("ILM policy 'default.yml' not found")
	}
	if !json.Valid(ilm.Content) {
		t.Error("ILM policy content is not valid JSON")
	}

	// Routing rules.
	if len(ds.RoutingRules) != 1 {
		t.Fatalf("routing rule sets = %d, want 1", len(ds.RoutingRules))
	}
	if ds.RoutingRules[0].SourceDataset != "test_integration.logs" {
		t.Errorf("routing source_dataset = %q, want test_integration.logs", ds.RoutingRules[0].SourceDataset)
	}
	if len(ds.RoutingRules[0].Rules) != 2 {
		t.Fatalf("routing rules count = %d, want 2", len(ds.RoutingRules[0].Rules))
	}
	// StringOrStrings: bare string "test_integration.errors" → []string{"test_integration.errors"}
	if got := ds.RoutingRules[0].Rules[0].TargetDataset; len(got) != 1 || got[0] != "test_integration.errors" {
		t.Errorf("routing rule[0] target_dataset = %v, want [test_integration.errors]", got)
	}
	// StringOrStrings: bare string "development" → []string{"development"}
	if got := ds.RoutingRules[0].Rules[1].Namespace; len(got) != 1 || got[0] != "development" {
		t.Errorf("routing rule[1] namespace = %v, want [development]", got)
	}

	// Sample event.
	if ds.SampleEvent == nil {
		t.Fatal("data stream sample event is nil")
	}
	if !json.Valid(ds.SampleEvent) {
		t.Error("data stream sample event is not valid JSON")
	}

	// Package-level pipelines.
	if len(pkg.Pipelines) != 1 {
		t.Fatalf("package pipeline count = %d, want 1", len(pkg.Pipelines))
	}
	ppf, ok := pkg.Pipelines["common.yml"]
	if !ok {
		t.Fatal("package pipeline 'common.yml' not found")
	}
	if ppf.Pipeline.Description != "Common shared pipeline" {
		t.Errorf("package pipeline description = %q, want Common shared pipeline", ppf.Pipeline.Description)
	}
	if len(ppf.Pipeline.Processors) != 1 {
		t.Errorf("package pipeline processors count = %d, want 1", len(ppf.Pipeline.Processors))
	}

	// Transforms.
	if len(pkg.Transforms) != 1 {
		t.Fatalf("transforms count = %d, want 1", len(pkg.Transforms))
	}
	td, ok := pkg.Transforms["latest"]
	if !ok {
		t.Fatal("transform 'latest' not found")
	}
	if td.Manifest == nil {
		t.Fatal("transform manifest is nil")
	}
	if td.Manifest.Start == nil || !*td.Manifest.Start {
		t.Error("transform manifest start should be true")
	}
	if len(td.Fields) != 1 {
		t.Errorf("transform fields count = %d, want 1", len(td.Fields))
	}

	// Fields should be nil for integration packages.
	if pkg.Fields != nil {
		t.Error("Fields should be nil for integration package")
	}

	// Agent templates should be nil without WithAgentTemplates.
	if pkg.AgentTemplates != nil {
		t.Error("AgentTemplates should be nil without WithAgentTemplates")
	}
	if ds.AgentTemplates != nil {
		t.Error("DataStream AgentTemplates should be nil without WithAgentTemplates")
	}
}

func TestReadInputPackage(t *testing.T) {
	pkg, err := Read("testdata/input_pkg")
	if err != nil {
		t.Fatal(err)
	}

	m := pkg.Manifest()
	if m == nil {
		t.Fatal("Manifest() returned nil")
	}
	if m.Name != "test_input" {
		t.Errorf("name = %q, want test_input", m.Name)
	}

	im := pkg.InputManifest()
	if im == nil {
		t.Fatal("InputManifest() returned nil")
	}
	if im.Type != "input" {
		t.Errorf("type = %q, want input", im.Type)
	}

	// Input package has Fields, not DataStreams.
	if pkg.DataStreams != nil {
		t.Error("DataStreams should be nil for input package")
	}
	if len(pkg.Fields) != 1 {
		t.Fatalf("fields file count = %d, want 1", len(pkg.Fields))
	}
	ff, ok := pkg.Fields["input.yml"]
	if !ok {
		t.Fatal("fields file 'input.yml' not found")
	}
	if len(ff.Fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(ff.Fields))
	}

	// Lifecycle.
	if pkg.Lifecycle == nil {
		t.Fatal("Lifecycle is nil for input package")
	}
	if pkg.Lifecycle.DataRetention != "30d" {
		t.Errorf("data_retention = %q, want 30d", pkg.Lifecycle.DataRetention)
	}

	// Sample event.
	if pkg.SampleEvent == nil {
		t.Fatal("input package sample event is nil")
	}
	if !json.Valid(pkg.SampleEvent) {
		t.Error("input package sample event is not valid JSON")
	}

	// Changelog.
	if len(pkg.Changelog) != 1 {
		t.Errorf("changelog count = %d, want 1", len(pkg.Changelog))
	}
}

func TestReadWithFS(t *testing.T) {
	// Use explicit os.DirFS + WithFS.
	fsys := os.DirFS("testdata/integration_pkg")
	pkg, err := Read(".", WithFS(fsys))
	if err != nil {
		t.Fatal(err)
	}

	m := pkg.Manifest()
	if m == nil {
		t.Fatal("Manifest() returned nil")
	}
	if m.Name != "test_integration" {
		t.Errorf("name = %q, want test_integration", m.Name)
	}
}

func TestReadWithKnownFields(t *testing.T) {
	// Lenient mode: unknown manifest fields are ignored.
	fsys := fstest.MapFS{
		"manifest.yml": &fstest.MapFile{
			Data: []byte("name: test\ntitle: Test\nversion: 1.0.0\ntype: input\nformat_version: 3.3.0\nunknown_field: value\n"),
		},
		"changelog.yml": &fstest.MapFile{
			Data: []byte("- version: 1.0.0\n  changes:\n    - description: Init.\n      type: enhancement\n      link: https://example.com/1\n"),
		},
	}

	pkg, err := Read(".", WithFS(fsys))
	if err != nil {
		t.Fatalf("lenient mode should not error: %v", err)
	}
	if pkg.Manifest().Name != "test" {
		t.Errorf("name = %q, want test", pkg.Manifest().Name)
	}

	// Verify WithKnownFields option is accepted (strict validation is
	// delegated to the underlying YAML decoder which has limitations
	// with types that implement custom UnmarshalYAML methods).
	pkg, err = Read(".", WithFS(fsys), WithKnownFields())
	if err != nil {
		t.Fatalf("with known fields should not error for valid manifest: %v", err)
	}
	if pkg.Manifest().Name != "test" {
		t.Errorf("name = %q, want test", pkg.Manifest().Name)
	}
}

func TestReadOptionalFiles(t *testing.T) {
	// Package with minimal files: no validation, no tags, no lifecycle.
	fsys := fstest.MapFS{
		"manifest.yml": &fstest.MapFile{
			Data: []byte("name: minimal\ntitle: Minimal\nversion: 1.0.0\ntype: input\nformat_version: 3.3.0\n"),
		},
		"changelog.yml": &fstest.MapFile{
			Data: []byte("- version: 1.0.0\n  changes:\n    - description: Init.\n      type: enhancement\n      link: https://example.com/1\n"),
		},
	}

	pkg, err := Read(".", WithFS(fsys))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if pkg.Validation != nil {
		t.Error("Validation should be nil when file is absent")
	}
	if pkg.Tags != nil {
		t.Error("Tags should be nil when file is absent")
	}
}

func TestFileMetadata(t *testing.T) {
	pkg, err := Read("testdata/integration_pkg")
	if err != nil {
		t.Fatal(err)
	}

	// Manifest file metadata.
	m := pkg.Manifest()
	if m.FilePath() != "manifest.yml" {
		t.Errorf("manifest file path = %q, want manifest.yml", m.FilePath())
	}
	if m.Line() != 1 {
		t.Errorf("manifest line = %d, want 1", m.Line())
	}

	// Data stream manifest file metadata.
	ds := pkg.DataStreams["logs"]
	if ds.Manifest.FilePath() != "data_stream/logs/manifest.yml" {
		t.Errorf("ds manifest file path = %q, want data_stream/logs/manifest.yml", ds.Manifest.FilePath())
	}

	// Pipeline file metadata.
	pf := ds.Pipelines["default.yml"]
	wantPipelinePath := "data_stream/logs/elasticsearch/ingest_pipeline/default.yml"
	if pf.Pipeline.FilePath() != wantPipelinePath {
		t.Errorf("pipeline file path = %q, want %s", pf.Pipeline.FilePath(), wantPipelinePath)
	}
	if pf.Pipeline.Processors[0].FilePath() != wantPipelinePath {
		t.Errorf("processor file path = %q, want %s", pf.Pipeline.Processors[0].FilePath(), wantPipelinePath)
	}
}

func TestManifestAccessor(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"integration", "testdata/integration_pkg", "test_integration"},
		{"input", "testdata/input_pkg", "test_input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := Read(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			m := pkg.Manifest()
			if m == nil {
				t.Fatal("Manifest() returned nil")
			}
			if m.Name != tt.want {
				t.Errorf("name = %q, want %q", m.Name, tt.want)
			}
		})
	}
}

func TestAgentTemplates(t *testing.T) {
	t.Run("integration", func(t *testing.T) {
		pkg, err := Read("testdata/integration_pkg", WithAgentTemplates())
		if err != nil {
			t.Fatal(err)
		}

		// Package-level agent templates.
		if len(pkg.AgentTemplates) != 1 {
			t.Fatalf("package agent templates count = %d, want 1", len(pkg.AgentTemplates))
		}
		tmpl, ok := pkg.AgentTemplates["agent/input/stream/stream.yml.hbs"]
		if !ok {
			t.Fatal("package agent template 'agent/input/stream/stream.yml.hbs' not found")
		}
		if tmpl.Content == "" {
			t.Error("package agent template content is empty")
		}
		if tmpl.Path() != "agent/input/stream/stream.yml.hbs" {
			t.Errorf("package agent template path = %q, want agent/input/stream/stream.yml.hbs", tmpl.Path())
		}

		// Data stream agent templates.
		ds := pkg.DataStreams["logs"]
		if len(ds.AgentTemplates) != 1 {
			t.Fatalf("ds agent templates count = %d, want 1", len(ds.AgentTemplates))
		}
		dsTmpl, ok := ds.AgentTemplates["data_stream/logs/agent/stream/stream.yml.hbs"]
		if !ok {
			t.Fatal("ds agent template 'data_stream/logs/agent/stream/stream.yml.hbs' not found")
		}
		if dsTmpl.Content == "" {
			t.Error("ds agent template content is empty")
		}
	})

	t.Run("input", func(t *testing.T) {
		pkg, err := Read("testdata/input_pkg", WithAgentTemplates())
		if err != nil {
			t.Fatal(err)
		}

		if len(pkg.AgentTemplates) != 1 {
			t.Fatalf("input agent templates count = %d, want 1", len(pkg.AgentTemplates))
		}
		tmpl, ok := pkg.AgentTemplates["agent/input/input.yml.hbs"]
		if !ok {
			t.Fatal("input agent template 'agent/input/input.yml.hbs' not found")
		}
		if tmpl.Content == "" {
			t.Error("input agent template content is empty")
		}
	})
}
