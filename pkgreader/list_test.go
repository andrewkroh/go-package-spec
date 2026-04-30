package pkgreader

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestListPackagesFS_Flat(t *testing.T) {
	fsys := fstest.MapFS{
		"packages/aws/manifest.yml":   {Data: []byte(makeManifest("aws"))},
		"packages/nginx/manifest.yml": {Data: []byte(makeManifest("nginx"))},
		"packages/notes.txt":          {Data: []byte("ignore me")},
	}

	got, err := ListPackagesFS(fsys, "packages")
	if err != nil {
		t.Fatalf("ListPackagesFS: %v", err)
	}
	want := []string{"packages/aws", "packages/nginx"}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListPackagesFS_Nested(t *testing.T) {
	fsys := fstest.MapFS{
		// Flat package alongside nested groups.
		"packages/aws/manifest.yml": {Data: []byte(makeManifest("aws"))},
		// Nested under technology directory.
		"packages/microsoft/defender_endpoint/manifest.yml": {Data: []byte(makeManifest("microsoft_defender_endpoint"))},
		"packages/microsoft/sentinel/manifest.yml":          {Data: []byte(makeManifest("microsoft_sentinel"))},
		// Three levels deep.
		"packages/cloud/google/gcp/manifest.yml": {Data: []byte(makeManifest("gcp"))},
	}

	got, err := ListPackagesFS(fsys, "packages")
	if err != nil {
		t.Fatalf("ListPackagesFS: %v", err)
	}
	want := []string{
		"packages/aws",
		"packages/cloud/google/gcp",
		"packages/microsoft/defender_endpoint",
		"packages/microsoft/sentinel",
	}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListPackagesFS_InvalidManifestNotMaskingNested(t *testing.T) {
	// A stray manifest.yml at packages/microsoft/ (without the required
	// fields) must not stop the walk from descending into the nested
	// packages below.
	fsys := fstest.MapFS{
		"packages/microsoft/manifest.yml":                   {Data: []byte("# placeholder, no required fields\n")},
		"packages/microsoft/defender_endpoint/manifest.yml": {Data: []byte(makeManifest("microsoft_defender_endpoint"))},
		"packages/microsoft/sentinel/manifest.yml":          {Data: []byte(makeManifest("microsoft_sentinel"))},
	}

	got, err := ListPackagesFS(fsys, "packages")
	if err != nil {
		t.Fatalf("ListPackagesFS: %v", err)
	}
	want := []string{
		"packages/microsoft/defender_endpoint",
		"packages/microsoft/sentinel",
	}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListPackagesFS_NoNestedPackagesInsidePackage(t *testing.T) {
	// Once a directory is recognized as a package, the walker must not
	// descend into it. Even if a sub-tree contains another manifest.yml,
	// it should not be returned.
	fsys := fstest.MapFS{
		"packages/outer/manifest.yml":                  {Data: []byte(makeManifest("outer"))},
		"packages/outer/data_stream/logs/manifest.yml": {Data: []byte("title: Logs\ntype: logs\n")},
	}

	got, err := ListPackagesFS(fsys, "packages")
	if err != nil {
		t.Fatalf("ListPackagesFS: %v", err)
	}
	want := []string{"packages/outer"}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListPackages_OS(t *testing.T) {
	tmp := t.TempDir()

	mustWrite := func(rel, content string) {
		t.Helper()
		full := filepath.Join(tmp, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("packages/aws/manifest.yml", makeManifest("aws"))
	mustWrite("packages/microsoft/defender/manifest.yml", makeManifest("microsoft_defender"))

	got, err := ListPackages(filepath.Join(tmp, "packages"))
	if err != nil {
		t.Fatalf("ListPackages: %v", err)
	}
	want := []string{
		filepath.Join(tmp, "packages", "aws"),
		filepath.Join(tmp, "packages", "microsoft", "defender"),
	}
	if !equalSlices(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func makeManifest(name string) string {
	return `format_version: 3.5.7
name: ` + name + `
type: integration
version: 1.0.0
title: ` + name + `
description: Test package.
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
