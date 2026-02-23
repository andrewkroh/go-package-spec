package sqlgen

import (
	"reflect"

	"github.com/andrewkroh/go-package-spec/pkgreader"
	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// typeRegistry maps pkgspec type names to their reflect.Type.
var typeRegistry = map[string]reflect.Type{
	// Manifest types.
	"Manifest":            reflect.TypeOf(pkgspec.Manifest{}),
	"IntegrationManifest": reflect.TypeOf(pkgspec.IntegrationManifest{}),
	"InputManifest":       reflect.TypeOf(pkgspec.InputManifest{}),
	"ContentManifest":     reflect.TypeOf(pkgspec.ContentManifest{}),

	// Policy templates.
	"PolicyTemplate":      reflect.TypeOf(pkgspec.PolicyTemplate{}),
	"InputPolicyTemplate": reflect.TypeOf(pkgspec.InputPolicyTemplate{}),
	"PolicyTemplateInput": reflect.TypeOf(pkgspec.PolicyTemplateInput{}),

	// Data streams.
	"DataStreamManifest":      reflect.TypeOf(pkgspec.DataStreamManifest{}),
	"DataStreamStream":        reflect.TypeOf(pkgspec.DataStreamStream{}),
	"DataStreamElasticsearch": reflect.TypeOf(pkgspec.DataStreamElasticsearch{}),

	// Fields.
	"Field":     reflect.TypeOf(pkgspec.Field{}),
	"FlatField": reflect.TypeOf(pkgspec.FlatField{}),

	// Variables.
	"Var":      reflect.TypeOf(pkgspec.Var{}),
	"VarGroup": reflect.TypeOf(pkgspec.VarGroup{}),

	// Changelog.
	"Changelog":      reflect.TypeOf(pkgspec.Changelog{}),
	"ChangelogEntry": reflect.TypeOf(pkgspec.ChangelogEntry{}),

	// Ingest pipelines.
	"IngestPipeline": reflect.TypeOf(pkgspec.IngestPipeline{}),
	"Processor":      reflect.TypeOf(pkgspec.Processor{}),

	// Transforms.
	"Transform":                reflect.TypeOf(pkgspec.Transform{}),
	"TransformManifest":        reflect.TypeOf(pkgspec.TransformManifest{}),
	"TransformSource":          reflect.TypeOf(pkgspec.TransformSource{}),
	"TransformDest":            reflect.TypeOf(pkgspec.TransformDest{}),
	"TransformPivot":           reflect.TypeOf(pkgspec.TransformPivot{}),
	"TransformLatest":          reflect.TypeOf(pkgspec.TransformLatest{}),
	"TransformSettings":        reflect.TypeOf(pkgspec.TransformSettings{}),
	"TransformRetentionPolicy": reflect.TypeOf(pkgspec.TransformRetentionPolicy{}),
	"TransformSync":            reflect.TypeOf(pkgspec.TransformSync{}),

	// Routing rules.
	"RoutingRuleSet": reflect.TypeOf(pkgspec.RoutingRuleSet{}),
	"RoutingRule":    reflect.TypeOf(pkgspec.RoutingRule{}),

	// Tests.
	"SystemTestConfig":   reflect.TypeOf(pkgspec.SystemTestConfig{}),
	"StaticTestConfig":   reflect.TypeOf(pkgspec.StaticTestConfig{}),
	"PolicyTestConfig":   reflect.TypeOf(pkgspec.PolicyTestConfig{}),
	"PipelineTestConfig": reflect.TypeOf(pkgspec.PipelineTestConfig{}),

	// Tags.
	"Tag": reflect.TypeOf(pkgspec.Tag{}),

	// Build.
	"BuildManifest":        reflect.TypeOf(pkgspec.BuildManifest{}),
	"BuildDependencies":    reflect.TypeOf(pkgspec.BuildDependencies{}),
	"BuildDependenciesECS": reflect.TypeOf(pkgspec.BuildDependenciesECS{}),

	// Misc.
	"Owner":           reflect.TypeOf(pkgspec.Owner{}),
	"Source":          reflect.TypeOf(pkgspec.Source{}),
	"Conditions":      reflect.TypeOf(pkgspec.Conditions{}),
	"Deprecated":      reflect.TypeOf(pkgspec.Deprecated{}),
	"Icon":            reflect.TypeOf(pkgspec.Icon{}),
	"Screenshot":      reflect.TypeOf(pkgspec.Screenshot{}),
	"IndexTemplate":   reflect.TypeOf(pkgspec.IndexTemplate{}),
	"DeploymentModes": reflect.TypeOf(pkgspec.DeploymentModes{}),

	// pkgreader types.
	"DataStream":    reflect.TypeOf(pkgreader.DataStream{}),
	"TransformData": reflect.TypeOf(pkgreader.TransformData{}),
}

// LookupType returns the reflect.Type for a pkgspec type name.
func LookupType(name string) (reflect.Type, bool) {
	t, ok := typeRegistry[name]
	return t, ok
}

// RegisteredPkgPaths returns the unique package import paths referenced
// by types in the registry.
func RegisteredPkgPaths() []string {
	seen := make(map[string]bool)
	for _, rt := range typeRegistry {
		if p := rt.PkgPath(); p != "" {
			seen[p] = true
		}
	}

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	return paths
}
