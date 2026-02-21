package generator

import "testing"

func TestToGoName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"format_version", "FormatVersion"},
		{"id", "ID"},
		{"name", "Name"},
		{"title", "Title"},
		{"type", "Type"},
		{"url", "URL"},
		{"urls", "URLs"},
		{"api", "API"},
		{"ip", "IP"},
		{"cpu", "CPU"},
		{"ilm", "ILM"},
		{"ssl", "SSL"},
		{"tls", "TLS"},
		{"http", "HTTP"},
		{"ecs", "ECS"},
		{"ui", "UI"},
		{"svg", "SVG"},
		{"json", "JSON"},
		{"dns", "DNS"},
		{"os", "OS"},
		{"ca", "CA"},
		{"policy_templates", "PolicyTemplates"},
		{"data_stream", "DataStream"},
		{"var_groups", "VarGroups"},
		{"format-version", "FormatVersion"},
		{"elasticsearch", "Elasticsearch"},
		{"dynamic_dataset", "DynamicDataset"},
		{"dynamic_namespace", "DynamicNamespace"},
		{"index_mode", "IndexMode"},
		{"source_mode", "SourceMode"},
		{"ilm_policy", "ILMPolicy"},
		{"template_path", "TemplatePath"},
		{"max_age", "MaxAge"},
		{"default_field", "DefaultField"},
		{"metric_type", "MetricType"},
		{"object_type", "ObjectType"},
		{"scaling_factor", "ScalingFactor"},
		{"ignore_above", "IgnoreAbove"},
		{"copy_to", "CopyTo"},
		{"null_value", "NullValue"},
		{"multi_fields", "MultiFields"},
		{"date_detection", "DateDetection"},
		{"dynamic_date_formats", "DynamicDateFormats"},
		{"dynamic_templates", "DynamicTemplates"},
		{"content_media_type", "ContentMediaType"},
		{"asset_types", "AssetTypes"},
		{"asset_ids", "AssetIDs"},
		{"exclude_checks", "ExcludeChecks"},
		{"docs_structure_enforced", "DocsStructureEnforced"},
		{"data_retention", "DataRetention"},
		{"ssl_verification_mode", "SSLVerificationMode"},
		{"http_url", "HTTPURL"},
		{"api_key", "APIKey"},
		{"dns_server", "DNSServer"},
		{"os_type", "OSType"},
		{"ca_cert", "CACert"},
		{"policy_templates_behavior", "PolicyTemplatesBehavior"},
		{"format_version", "FormatVersion"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ToGoName(tt.input)
			if got != tt.want {
				t.Errorf("ToGoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToTypeName(t *testing.T) {
	tests := []struct {
		schemaFile string
		defName    string
		parentType string
		want       string
	}{
		{"integration/manifest.jsonschema.json", "owner", "", "Owner"},
		{"integration/manifest.jsonschema.json", "categories", "", "Categories"},
		{"integration/manifest.jsonschema.json", "", "", "Manifest"},
		{"integration/data_stream/manifest.jsonschema.json", "", "", "Manifest"},
		{"integration/data_stream/manifest.jsonschema.json", "vars", "", "Vars"},
		{"integration/changelog.jsonschema.json", "", "", "Changelog"},
		{"integration/data_stream/fields/fields.jsonschema.json", "", "", "Fields"},
		{"manifest.jsonschema.json", "", "", "Manifest"},
		{"integration/manifest.jsonschema.json", "", "Integration", "IntegrationManifest"},
	}

	for _, tt := range tests {
		t.Run(tt.defName+"_"+tt.schemaFile, func(t *testing.T) {
			got := ToTypeName(tt.schemaFile, tt.defName, tt.parentType)
			if got != tt.want {
				t.Errorf("ToTypeName(%q, %q, %q) = %q, want %q",
					tt.schemaFile, tt.defName, tt.parentType, got, tt.want)
			}
		})
	}
}

func TestSplitWords(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"format_version", []string{"format", "version"}},
		{"formatVersion", []string{"format", "Version"}},
		{"format-version", []string{"format", "version"}},
		{"URLParser", []string{"URL", "Parser"}},
		{"myURL", []string{"my", "URL"}},
		{"simple", []string{"simple"}},
		{"a_b_c", []string{"a", "b", "c"}},
		{"HTTPSServer", []string{"HTTPS", "Server"}},
		{"getHTTPResponse", []string{"get", "HTTP", "Response"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitWords(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitWords(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitWords(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
