// Package pkgreader loads Elastic packages from disk into pkgspec types.
//
// The primary entry point is [Read], which accepts a package directory path
// and returns a fully-populated [Package] value. It detects the package type
// (integration, input, content) from the manifest and loads the appropriate
// components.
//
// The reader uses [io/fs.FS] for filesystem abstraction, which allows
// testing with in-memory filesystems. By default it uses [os.DirFS] for
// the provided path.
package pkgreader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// Package represents a fully-loaded Elastic package.
type Package struct {
	manifest any // *pkgspec.IntegrationManifest, *pkgspec.InputManifest, or *pkgspec.ContentManifest

	Changelog  []pkgspec.Changelog
	Validation *pkgspec.Validation    // nil if absent
	Build      *pkgspec.BuildManifest // type:integration only, nil if absent

	DataStreams    map[string]*DataStream          // type:integration only
	Fields         map[string]*FieldsFile          // type:input only
	Pipelines      map[string]*PipelineFile        // package-level elasticsearch/ingest_pipeline/
	Transforms     map[string]*TransformData       // nil if absent
	Tags           []pkgspec.Tag                   // nil if absent
	Lifecycle      *pkgspec.Lifecycle              // type:input only, nil if absent
	SampleEvent    json.RawMessage                 // type:input only, nil if absent
	KibanaObjects  map[string][]*KibanaSavedObject // type:integration and type:content only, keyed by asset type
	AgentTemplates map[string]*AgentTemplate       // type:integration and type:input only, nil unless WithAgentTemplates used
	Images         map[string]*ImageFile           // nil unless WithImageMetadata used
	Docs           []*DocFile                      // documentation files from docs/

	TestConfig      *pkgspec.TestConfig      // type:integration only, nil unless WithTestConfigs used
	InputTestConfig *pkgspec.InputTestConfig // type:input only, nil unless WithTestConfigs used
	InputTests      *InputPackageTests       // type:input only, nil unless WithTestConfigs used

	Commit string // git HEAD commit ID, empty unless WithGitMetadata used

	path string
}

// Path returns the package's directory path as provided to Read.
func (p *Package) Path() string {
	return p.path
}

// Manifest returns the common manifest fields regardless of package type.
func (p *Package) Manifest() *pkgspec.Manifest {
	switch m := p.manifest.(type) {
	case *pkgspec.IntegrationManifest:
		return &m.Manifest
	case *pkgspec.InputManifest:
		return &m.Manifest
	case *pkgspec.ContentManifest:
		return &m.Manifest
	}
	return nil
}

// IntegrationManifest returns the full integration manifest, or nil if the
// package is not of type "integration".
func (p *Package) IntegrationManifest() *pkgspec.IntegrationManifest {
	m, _ := p.manifest.(*pkgspec.IntegrationManifest)
	return m
}

// InputManifest returns the full input manifest, or nil if the package is
// not of type "input".
func (p *Package) InputManifest() *pkgspec.InputManifest {
	m, _ := p.manifest.(*pkgspec.InputManifest)
	return m
}

// ContentManifest returns the full content manifest, or nil if the package is
// not of type "content".
func (p *Package) ContentManifest() *pkgspec.ContentManifest {
	m, _ := p.manifest.(*pkgspec.ContentManifest)
	return m
}

// Option configures the behavior of Read.
type Option func(*config)

type config struct {
	fsys           fs.FS
	knownFields    bool
	gitMetadata    bool
	agentTemplates bool
	imageMetadata  bool
	testConfigs    bool
	pathPrefix     string // prefix prepended to all FileMetadata file paths
	packagePath    string // original OS path, needed for git operations
	codeownersPath string // path to CODEOWNERS file for data stream ownership
}

// WithFS provides a custom filesystem for reading package files. When set,
// the path argument to Read is interpreted relative to this filesystem.
func WithFS(fsys fs.FS) Option {
	return func(c *config) {
		c.fsys = fsys
	}
}

// WithKnownFields enables strict YAML validation where only fields defined
// in the model types are allowed. By default, unknown fields are silently
// ignored for forward compatibility.
func WithKnownFields() Option {
	return func(c *config) {
		c.knownFields = true
	}
}

// WithAgentTemplates enables loading of agent Handlebars template files
// (.yml.hbs) from agent/ directories. These are skipped by default to
// avoid unnecessary memory usage when templates are not needed.
func WithAgentTemplates() Option {
	return func(c *config) {
		c.agentTemplates = true
	}
}

// WithImageMetadata enables loading of image files from the img/ directory.
// When set, the reader decodes image dimensions (width, height) and records
// byte sizes for PNG, JPEG, and SVG files. SVG files only have byte size
// recorded since they are vector images.
func WithImageMetadata() Option {
	return func(c *config) {
		c.imageMetadata = true
	}
}

// WithTestConfigs enables loading of _dev/test/ configuration files. When
// set, the reader populates test config fields on Package and DataStream.
// This is skipped by default to avoid unnecessary overhead when test
// configurations are not needed.
func WithTestConfigs() Option {
	return func(c *config) {
		c.testConfigs = true
	}
}

// WithGitMetadata enables git metadata enrichment. When set, the reader
// populates Package.Commit with the HEAD commit ID and uses git blame to
// populate Changelog.Date fields.
func WithGitMetadata() Option {
	return func(c *config) {
		c.gitMetadata = true
	}
}

// WithPathPrefix sets a prefix that is prepended to all [pkgspec.FileMetadata]
// file paths after loading. This is useful when analyzing packages within a
// larger repository, allowing file paths to be repo-relative (e.g.
// "packages/nginx/manifest.yml") rather than package-relative.
func WithPathPrefix(prefix string) Option {
	return func(c *config) {
		c.pathPrefix = prefix
	}
}

// WithCodeowners provides a path to a CODEOWNERS file for enriching data
// stream manifests with their GitHub team owner. This is useful when the
// CODEOWNERS file assigns per-data-stream ownership that is more granular
// than the package-level owner.
func WithCodeowners(path string) Option {
	return func(c *config) {
		c.codeownersPath = path
	}
}

// Read loads an Elastic package from the given directory path. It detects
// the package type from the manifest and loads all associated components.
func Read(pkgPath string, opts ...Option) (*Package, error) {
	cfg := &config{
		packagePath: pkgPath,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var root string
	if cfg.fsys != nil {
		root = pkgPath
	} else {
		cfg.fsys = os.DirFS(pkgPath)
		root = "."
	}

	// Detect package type from manifest.
	manifestPath := path.Join(root, "manifest.yml")
	pkgType, err := detectManifestType(cfg.fsys, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("detecting package type: %w", err)
	}

	pkg := &Package{
		path: pkgPath,
	}

	// Decode manifest into the correct type.
	switch pkgType {
	case "integration":
		var m pkgspec.IntegrationManifest
		if err := decodeYAML(cfg.fsys, manifestPath, &m, cfg.knownFields); err != nil {
			return nil, fmt.Errorf("reading manifest: %w", err)
		}
		pkgspec.AnnotateFileMetadata(manifestPath, &m)
		pkg.manifest = &m
	case "input":
		var m pkgspec.InputManifest
		if err := decodeYAML(cfg.fsys, manifestPath, &m, cfg.knownFields); err != nil {
			return nil, fmt.Errorf("reading manifest: %w", err)
		}
		pkgspec.AnnotateFileMetadata(manifestPath, &m)
		pkg.manifest = &m
	case "content":
		var m pkgspec.ContentManifest
		if err := decodeYAML(cfg.fsys, manifestPath, &m, cfg.knownFields); err != nil {
			return nil, fmt.Errorf("reading manifest: %w", err)
		}
		pkgspec.AnnotateFileMetadata(manifestPath, &m)
		pkg.manifest = &m
	default:
		return nil, fmt.Errorf("unsupported package type: %q", pkgType)
	}

	// Read changelog.
	changelogPath := path.Join(root, "changelog.yml")
	if err := decodeYAML(cfg.fsys, changelogPath, &pkg.Changelog, false); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading changelog: %w", err)
		}
	} else {
		pkgspec.AnnotateFileMetadata(changelogPath, &pkg.Changelog)
	}

	// Read validation (optional).
	validationPath := path.Join(root, "validation.yml")
	validation, err := readOptionalYAML[pkgspec.Validation](cfg.fsys, validationPath, cfg.knownFields)
	if err != nil {
		return nil, fmt.Errorf("reading validation: %w", err)
	}
	if validation != nil {
		pkgspec.AnnotateFileMetadata(validationPath, validation)
		pkg.Validation = validation
	}

	// Read tags (optional).
	tagsPath := path.Join(root, "kibana", "tags.yml")
	if err := decodeYAML(cfg.fsys, tagsPath, &pkg.Tags, cfg.knownFields); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading tags: %w", err)
		}
	} else {
		pkgspec.AnnotateFileMetadata(tagsPath, &pkg.Tags)
	}

	// Read images (optional, requires WithImageMetadata).
	if cfg.imageMetadata {
		imgDir := path.Join(root, "img")
		images, err := readImages(cfg.fsys, imgDir)
		if err != nil {
			return nil, fmt.Errorf("reading images: %w", err)
		}
		pkg.Images = images
	}

	// Read documentation file metadata.
	docs, err := readDocs(cfg.fsys, root)
	if err != nil {
		return nil, fmt.Errorf("reading docs: %w", err)
	}
	pkg.Docs = docs

	// Type-specific components.
	switch pkgType {
	case "integration":
		// Read data streams.
		ds, err := readDataStreams(cfg.fsys, root, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading data streams: %w", err)
		}
		pkg.DataStreams = ds

		// Read package-level ingest pipelines.
		pipelinesDir := path.Join(root, "elasticsearch", "ingest_pipeline")
		pipelines, err := readPipelines(cfg.fsys, pipelinesDir)
		if err != nil {
			return nil, fmt.Errorf("reading pipelines: %w", err)
		}
		pkg.Pipelines = pipelines

		// Read transforms.
		transforms, err := readTransforms(cfg.fsys, root, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading transforms: %w", err)
		}
		pkg.Transforms = transforms

		// Read agent templates (optional, requires WithAgentTemplates).
		if cfg.agentTemplates {
			agentDir := path.Join(root, "agent")
			templates, err := readAgentTemplates(cfg.fsys, agentDir)
			if err != nil {
				return nil, fmt.Errorf("reading agent templates: %w", err)
			}
			pkg.AgentTemplates = templates
		}

		// Read Kibana saved objects.
		kibanaObjects, err := readKibanaObjects(cfg.fsys, root)
		if err != nil {
			return nil, fmt.Errorf("reading kibana objects: %w", err)
		}
		pkg.KibanaObjects = kibanaObjects

		// Read build manifest (optional).
		buildPath := path.Join(root, "_dev", "build", "build.yml")
		build, err := readOptionalYAML[pkgspec.BuildManifest](cfg.fsys, buildPath, cfg.knownFields)
		if err != nil {
			return nil, fmt.Errorf("reading build manifest: %w", err)
		}
		if build != nil {
			pkgspec.AnnotateFileMetadata(buildPath, build)
			pkg.Build = build
		}

		// Read test config (optional, requires WithTestConfigs).
		if cfg.testConfigs {
			testConfigPath := path.Join(root, "_dev", "test", "config.yml")
			tc, err := readOptionalYAML[pkgspec.TestConfig](cfg.fsys, testConfigPath, cfg.knownFields)
			if err != nil {
				return nil, fmt.Errorf("reading test config: %w", err)
			}
			if tc != nil {
				pkgspec.AnnotateFileMetadata(testConfigPath, tc)
				pkg.TestConfig = tc
			}
		}

	case "input":
		// Read fields from package-root fields/ directory.
		fieldsDir := path.Join(root, "fields")
		fields, err := readFieldsDir(cfg.fsys, fieldsDir, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading fields: %w", err)
		}
		pkg.Fields = fields

		// Read lifecycle (optional).
		lifecyclePath := path.Join(root, "lifecycle.yml")
		lifecycle, err := readOptionalYAML[pkgspec.Lifecycle](cfg.fsys, lifecyclePath, cfg.knownFields)
		if err != nil {
			return nil, fmt.Errorf("reading lifecycle: %w", err)
		}
		if lifecycle != nil {
			pkgspec.AnnotateFileMetadata(lifecyclePath, lifecycle)
			pkg.Lifecycle = lifecycle
		}

		// Read sample event (optional).
		sampleEventPath := path.Join(root, "sample_event.json")
		sampleEvent, err := readOptionalFile(cfg.fsys, sampleEventPath)
		if err != nil {
			return nil, fmt.Errorf("reading sample event: %w", err)
		}
		pkg.SampleEvent = sampleEvent

		// Read agent templates (optional, requires WithAgentTemplates).
		if cfg.agentTemplates {
			agentDir := path.Join(root, "agent")
			templates, err := readAgentTemplates(cfg.fsys, agentDir)
			if err != nil {
				return nil, fmt.Errorf("reading agent templates: %w", err)
			}
			pkg.AgentTemplates = templates
		}

		// Read test config and test cases (optional, requires WithTestConfigs).
		if cfg.testConfigs {
			testConfigPath := path.Join(root, "_dev", "test", "config.yml")
			tc, err := readOptionalYAML[pkgspec.InputTestConfig](cfg.fsys, testConfigPath, cfg.knownFields)
			if err != nil {
				return nil, fmt.Errorf("reading test config: %w", err)
			}
			if tc != nil {
				pkgspec.AnnotateFileMetadata(testConfigPath, tc)
				pkg.InputTestConfig = tc
			}

			testDir := path.Join(root, "_dev", "test")
			inputTests, err := readInputPackageTests(cfg.fsys, testDir, cfg)
			if err != nil {
				return nil, fmt.Errorf("reading input tests: %w", err)
			}
			pkg.InputTests = inputTests
		}

	case "content":
		// Read Kibana saved objects.
		kibanaObjects, err := readKibanaObjects(cfg.fsys, root)
		if err != nil {
			return nil, fmt.Errorf("reading kibana objects: %w", err)
		}
		pkg.KibanaObjects = kibanaObjects
	}

	// Git metadata enrichment.
	if cfg.gitMetadata {
		commit, err := gitRevParseHEAD(cfg.packagePath)
		if err != nil {
			return nil, fmt.Errorf("reading git commit: %w", err)
		}
		pkg.Commit = commit

		if len(pkg.Changelog) > 0 {
			if err := annotateChangelogDates(pkg.Changelog, cfg.packagePath, "changelog.yml"); err != nil {
				return nil, fmt.Errorf("annotating changelog dates: %w", err)
			}
		}
	}

	// CODEOWNERS enrichment.
	if cfg.codeownersPath != "" && len(pkg.DataStreams) > 0 {
		cf, err := loadCodeowners(cfg.codeownersPath)
		if err != nil {
			return nil, fmt.Errorf("loading CODEOWNERS: %w", err)
		}
		dirName := path.Base(cfg.packagePath)
		for dsName, ds := range pkg.DataStreams {
			dsPath := "/packages/" + dirName + "/data_stream/" + dsName
			if owner := cf.matchOwner(dsPath); owner != "" {
				ds.Manifest.GithubCodeOwner = owner
			}
		}
	}

	// Prefix all FileMetadata file paths.
	if cfg.pathPrefix != "" {
		pkgspec.PrefixFileMetadata(cfg.pathPrefix, pkg.manifest)
		pkgspec.PrefixFileMetadata(cfg.pathPrefix, pkg)
		for _, d := range pkg.Docs {
			d.path = path.Join(cfg.pathPrefix, d.path)
		}
	}

	return pkg, nil
}

// manifestTypeDetector is used to extract only the "type" field from a manifest.
type manifestTypeDetector struct {
	Type string `yaml:"type"`
}

// detectManifestType reads the manifest file just enough to determine the
// package type.
func detectManifestType(fsys fs.FS, manifestPath string) (string, error) {
	var detector manifestTypeDetector
	if err := decodeYAML(fsys, manifestPath, &detector, false); err != nil {
		return "", err
	}
	if detector.Type == "" {
		return "", fmt.Errorf("manifest at %s has no type field", manifestPath)
	}
	return detector.Type, nil
}
