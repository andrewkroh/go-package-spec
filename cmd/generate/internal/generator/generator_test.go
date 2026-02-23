package generator

import "testing"

func TestExtractSpecVersion(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{
			id:   "https://schemas.elastic.dev/package-spec/3.5.7/integration/manifest.jsonschema.json",
			want: "3.5.7",
		},
		{
			id:   "https://schemas.elastic.dev/package-spec/4.0.0/content/manifest.jsonschema.json",
			want: "4.0.0",
		},
		{
			id:   "",
			want: "",
		},
		{
			id:   "https://example.com/other-schema/1.0.0/foo.json",
			want: "",
		},
		{
			id:   "https://schemas.elastic.dev/package-spec/3.5.7",
			want: "3.5.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := extractSpecVersion(tt.id)
			if got != tt.want {
				t.Errorf("extractSpecVersion(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
