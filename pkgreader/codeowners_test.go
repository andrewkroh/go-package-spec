package pkgreader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCodeowners(t *testing.T) {
	content := `# Comment line
* @elastic/integrations

/packages/aws @elastic/obs-ds-hosted-services @elastic/security-service-integrations @elastic/obs-infraobs-integrations
/packages/aws/data_stream/cloudtrail @elastic/security-service-integrations

# Blank lines above and below are ignored

/packages/nginx @elastic/obs-ds-hosted-services
`
	path := writeTemp(t, content)

	cf, err := parseCodeowners(path)
	if err != nil {
		t.Fatalf("parseCodeowners: %v", err)
	}

	if len(cf.rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(cf.rules))
	}

	// Verify @ prefix is stripped.
	for _, rule := range cf.rules {
		for _, owner := range rule.owners {
			if owner[0] == '@' {
				t.Errorf("owner %q still has @ prefix", owner)
			}
		}
	}

	// Verify first rule.
	if cf.rules[0].pattern != "*" {
		t.Errorf("expected pattern *, got %q", cf.rules[0].pattern)
	}
	if cf.rules[0].owners[0] != "elastic/integrations" {
		t.Errorf("expected owner elastic/integrations, got %q", cf.rules[0].owners[0])
	}

	// Verify rule with multiple owners.
	if len(cf.rules[1].owners) != 3 {
		t.Errorf("expected 3 owners, got %d", len(cf.rules[1].owners))
	}
}

func TestMatchOwner(t *testing.T) {
	cf := &codeownersFile{
		rules: []codeownersRule{
			{pattern: "*", owners: []string{"elastic/integrations"}},
			{pattern: "/packages/aws", owners: []string{"elastic/obs-ds-hosted-services", "elastic/security-service-integrations"}},
			{pattern: "/packages/aws/data_stream/cloudtrail", owners: []string{"elastic/security-service-integrations"}},
			{pattern: "/packages/nginx", owners: []string{"elastic/obs-ds-hosted-services"}},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "exact data stream match",
			path:     "/packages/aws/data_stream/cloudtrail",
			expected: "elastic/security-service-integrations",
		},
		{
			name:     "prefix match for other data stream falls back to package rule",
			path:     "/packages/aws/data_stream/s3access",
			expected: "elastic/obs-ds-hosted-services",
		},
		{
			name:     "prefix match with deeper path",
			path:     "/packages/aws/data_stream/cloudtrail/fields/base.yml",
			expected: "elastic/security-service-integrations",
		},
		{
			name:     "nginx data stream",
			path:     "/packages/nginx/data_stream/access",
			expected: "elastic/obs-ds-hosted-services",
		},
		{
			name:     "no match returns empty",
			path:     "/some/other/path",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cf.matchOwner(tt.path)
			if got != tt.expected {
				t.Errorf("matchOwner(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern  string
		filePath string
		want     bool
	}{
		{"/packages/aws", "/packages/aws", true},
		{"/packages/aws", "/packages/aws/data_stream/cloudtrail", true},
		{"/packages/aws", "/packages/awslogs", false},
		{"/packages/aws/", "/packages/aws/manifest.yml", true},
		{"*", "/anything", false}, // * is not a real prefix pattern
	}

	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.filePath)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.filePath, got, tt.want)
		}
	}
}

func TestLoadCodeownersCache(t *testing.T) {
	content := "/packages/test @elastic/test-team\n"
	path := writeTemp(t, content)

	cf1, err := loadCodeowners(path)
	if err != nil {
		t.Fatalf("first loadCodeowners: %v", err)
	}

	cf2, err := loadCodeowners(path)
	if err != nil {
		t.Fatalf("second loadCodeowners: %v", err)
	}

	if cf1 != cf2 {
		t.Error("expected cached result to return same pointer")
	}

	// Clean up cache entry to avoid polluting other tests.
	codeownersCache.Delete(path)
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "CODEOWNERS")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadAppliesCodeownersForFlatPackage(t *testing.T) {
	pkgDir := t.TempDir()
	writePackageWithDataStream(t, pkgDir, "aws", "cloudtrail")
	codeowners := writeTemp(t, `
* @elastic/integrations
/packages/aws @elastic/obs-team
/packages/aws/data_stream/cloudtrail @elastic/security-team
`)

	pkg, err := Read(
		pkgDir,
		WithCodeowners(codeowners),
		WithRepoRelativePath("packages/aws"),
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	ds := pkg.DataStreams["cloudtrail"]
	if ds == nil {
		t.Fatal("data stream cloudtrail not loaded")
	}
	if got, want := ds.Manifest.GithubCodeOwner, "elastic/security-team"; got != want {
		t.Errorf("GithubCodeOwner = %q, want %q", got, want)
	}
}

func TestReadAppliesCodeownersForNestedPackage(t *testing.T) {
	// A nested package layout — packages/microsoft/defender_endpoint —
	// must look up CODEOWNERS using the full repo-relative path, not the
	// directory basename.
	pkgDir := t.TempDir()
	writePackageWithDataStream(t, pkgDir, "microsoft_defender_endpoint", "alerts")
	codeowners := writeTemp(t, `
* @elastic/integrations
/packages/microsoft/defender_endpoint @elastic/sec-eng
/packages/microsoft/defender_endpoint/data_stream/alerts @elastic/sec-detections
`)

	pkg, err := Read(
		pkgDir,
		WithCodeowners(codeowners),
		WithRepoRelativePath("packages/microsoft/defender_endpoint"),
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	ds := pkg.DataStreams["alerts"]
	if ds == nil {
		t.Fatal("data stream alerts not loaded")
	}
	if got, want := ds.Manifest.GithubCodeOwner, "elastic/sec-detections"; got != want {
		t.Errorf("GithubCodeOwner = %q, want %q", got, want)
	}
}

func TestReadAppliesCodeownersFromPathPrefix(t *testing.T) {
	// When WithRepoRelativePath is not provided, WithPathPrefix should be
	// used as the fallback so existing callers keep working on the nested
	// layout.
	pkgDir := t.TempDir()
	writePackageWithDataStream(t, pkgDir, "microsoft_defender_endpoint", "alerts")
	codeowners := writeTemp(t, `
* @elastic/integrations
/packages/microsoft/defender_endpoint @elastic/sec-eng
`)

	pkg, err := Read(
		pkgDir,
		WithCodeowners(codeowners),
		WithPathPrefix("packages/microsoft/defender_endpoint"),
	)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	ds := pkg.DataStreams["alerts"]
	if ds == nil {
		t.Fatal("data stream alerts not loaded")
	}
	if got, want := ds.Manifest.GithubCodeOwner, "elastic/sec-eng"; got != want {
		t.Errorf("GithubCodeOwner = %q, want %q", got, want)
	}
}

// writePackageWithDataStream lays down a minimal but valid integration
// package on disk under dir, with a single data stream named dsName.
func writePackageWithDataStream(t *testing.T, dir, pkgName, dsName string) {
	t.Helper()

	files := map[string]string{
		"manifest.yml": `format_version: 3.5.7
name: ` + pkgName + `
type: integration
version: 1.0.0
title: ` + pkgName + `
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
`,
		"changelog.yml": `- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://example.com/1
`,
		filepath.Join("data_stream", dsName, "manifest.yml"): `title: ` + dsName + `
type: logs
streams:
  - input: logfile
    title: ` + dsName + `
    description: Collect logs.
`,
		filepath.Join("data_stream", dsName, "fields", "base-fields.yml"): `- name: "@timestamp"
  type: date
`,
	}

	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
