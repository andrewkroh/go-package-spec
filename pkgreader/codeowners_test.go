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
