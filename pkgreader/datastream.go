package pkgreader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// DataStream represents a fully-loaded data stream within an integration package.
type DataStream struct {
	Manifest       pkgspec.DataStreamManifest
	Fields         map[string]*FieldsFile    // keyed by filename
	Pipelines      map[string]*PipelineFile  // keyed by filename (e.g., "default.yml")
	ILMPolicies    map[string]*ILMPolicy     // keyed by filename, nil if absent
	Lifecycle      *pkgspec.Lifecycle        // nil if absent
	RoutingRules   []pkgspec.RoutingRuleSet  // nil if absent
	SampleEvent    json.RawMessage           // nil if absent
	AgentTemplates map[string]*AgentTemplate // nil unless WithAgentTemplates used
	Tests          *DataStreamTests          // nil unless WithTestConfigs used
	path           string
}

// Path returns the data stream's directory path relative to the package root.
func (ds *DataStream) Path() string {
	return ds.path
}

// AllFields returns all fields from all field files in the data stream.
func (ds *DataStream) AllFields() []pkgspec.Field {
	var all []pkgspec.Field
	for _, ff := range ds.Fields {
		all = append(all, ff.Fields...)
	}
	return all
}

// FieldsFile represents a single fields YAML file.
type FieldsFile struct {
	Fields []pkgspec.Field
	path   string
}

// Path returns the file path relative to the package root.
func (ff *FieldsFile) Path() string {
	return ff.path
}

// PipelineFile represents a single ingest pipeline YAML file.
type PipelineFile struct {
	Pipeline pkgspec.IngestPipeline
	path     string
}

// Path returns the file path relative to the package root.
func (pf *PipelineFile) Path() string {
	return pf.path
}

// ILMPolicy represents a single ILM policy file. The contents are opaque
// YAML/JSON with no typed schema defined by package-spec.
type ILMPolicy struct {
	Content json.RawMessage // raw JSON representation of the policy
	path    string
}

// Path returns the file path relative to the package root.
func (p *ILMPolicy) Path() string {
	return p.path
}

func readDataStreams(fsys fs.FS, root string, cfg *config) (map[string]*DataStream, error) {
	dsDir := path.Join(root, "data_stream")

	entries, err := fs.ReadDir(fsys, dsDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading data_stream directory: %w", err)
	}

	result := make(map[string]*DataStream, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		dsPath := path.Join(dsDir, name)

		ds, err := readDataStream(fsys, dsPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading data stream %s: %w", name, err)
		}
		result[name] = ds
	}

	return result, nil
}

func readDataStream(fsys fs.FS, dsPath string, cfg *config) (*DataStream, error) {
	ds := &DataStream{
		path: dsPath,
	}

	// Read manifest.
	manifestPath := path.Join(dsPath, "manifest.yml")
	if err := decodeYAML(fsys, manifestPath, &ds.Manifest, cfg.knownFields); err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	pkgspec.AnnotateFileMetadata(manifestPath, &ds.Manifest)

	// Read lifecycle (optional).
	lifecyclePath := path.Join(dsPath, "lifecycle.yml")
	lifecycle, err := readOptionalYAML[pkgspec.Lifecycle](fsys, lifecyclePath, cfg.knownFields)
	if err != nil {
		return nil, fmt.Errorf("reading lifecycle: %w", err)
	}
	if lifecycle != nil {
		pkgspec.AnnotateFileMetadata(lifecyclePath, lifecycle)
		ds.Lifecycle = lifecycle
	}

	// Read fields.
	fieldsDir := path.Join(dsPath, "fields")
	fields, err := readFieldsDir(fsys, fieldsDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("reading fields: %w", err)
	}
	ds.Fields = fields

	// Read ingest pipelines.
	pipelinesDir := path.Join(dsPath, "elasticsearch", "ingest_pipeline")
	pipelines, err := readPipelines(fsys, pipelinesDir)
	if err != nil {
		return nil, fmt.Errorf("reading pipelines: %w", err)
	}
	ds.Pipelines = pipelines

	// Read ILM policies.
	ilmDir := path.Join(dsPath, "elasticsearch", "ilm")
	ilmPolicies, err := readILMPolicies(fsys, ilmDir)
	if err != nil {
		return nil, fmt.Errorf("reading ILM policies: %w", err)
	}
	ds.ILMPolicies = ilmPolicies

	// Read routing rules (optional).
	routingRulesPath := path.Join(dsPath, "routing_rules.yml")
	var routingRules []pkgspec.RoutingRuleSet
	if err := decodeYAML(fsys, routingRulesPath, &routingRules, false); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("reading routing rules: %w", err)
		}
	}
	ds.RoutingRules = routingRules

	// Read sample event (optional).
	sampleEventPath := path.Join(dsPath, "sample_event.json")
	sampleEvent, err := readOptionalFile(fsys, sampleEventPath)
	if err != nil {
		return nil, fmt.Errorf("reading sample event: %w", err)
	}
	ds.SampleEvent = sampleEvent

	// Read agent templates (optional, requires WithAgentTemplates).
	if cfg.agentTemplates {
		agentDir := path.Join(dsPath, "agent", "stream")
		templates, err := readAgentTemplates(fsys, agentDir)
		if err != nil {
			return nil, fmt.Errorf("reading agent templates: %w", err)
		}
		ds.AgentTemplates = templates
	}

	// Read test configs (optional, requires WithTestConfigs).
	if cfg.testConfigs {
		tests, err := readDataStreamTests(fsys, dsPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading tests: %w", err)
		}
		ds.Tests = tests
	}

	return ds, nil
}

func readFieldsDir(fsys fs.FS, dir string, cfg *config) (map[string]*FieldsFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading fields directory %s: %w", dir, err)
	}

	result := make(map[string]*FieldsFile, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		filePath := path.Join(dir, name)
		ff, err := readFieldsFile(fsys, filePath, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading fields file %s: %w", name, err)
		}
		result[name] = ff
	}

	return result, nil
}

func readFieldsFile(fsys fs.FS, filePath string, cfg *config) (*FieldsFile, error) {
	var fields []pkgspec.Field
	if err := decodeYAML(fsys, filePath, &fields, cfg.knownFields); err != nil {
		return nil, err
	}

	pkgspec.AnnotateFileMetadata(filePath, &fields)

	return &FieldsFile{
		Fields: fields,
		path:   filePath,
	}, nil
}

func readPipelines(fsys fs.FS, dir string) (map[string]*PipelineFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading pipeline directory %s: %w", dir, err)
	}

	result := make(map[string]*PipelineFile, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		filePath := path.Join(dir, name)
		var pipeline pkgspec.IngestPipeline
		// Pipelines contain arbitrary ES DSL; always decode with knownFields=false.
		if err := decodeYAML(fsys, filePath, &pipeline, false); err != nil {
			return nil, fmt.Errorf("reading pipeline file %s: %w", name, err)
		}
		pkgspec.AnnotateFileMetadata(filePath, &pipeline)

		result[name] = &PipelineFile{
			Pipeline: pipeline,
			path:     filePath,
		}
	}

	return result, nil
}

func readILMPolicies(fsys fs.FS, dir string) (map[string]*ILMPolicy, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading ILM directory %s: %w", dir, err)
	}

	result := make(map[string]*ILMPolicy, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".json") {
			continue
		}

		filePath := path.Join(dir, name)
		data, err := fs.ReadFile(fsys, filePath)
		if err != nil {
			return nil, fmt.Errorf("reading ILM policy %s: %w", name, err)
		}

		// Convert YAML to JSON for uniform storage.
		var raw any
		if strings.HasSuffix(name, ".json") {
			if err := json.Unmarshal(data, &raw); err != nil {
				return nil, fmt.Errorf("parsing ILM policy %s: %w", name, err)
			}
		} else {
			if err := decodeYAMLBytes(data, &raw); err != nil {
				return nil, fmt.Errorf("parsing ILM policy %s: %w", name, err)
			}
		}

		content, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("marshaling ILM policy %s: %w", name, err)
		}

		result[name] = &ILMPolicy{
			Content: content,
			path:    filePath,
		}
	}

	return result, nil
}

func readOptionalFile(fsys fs.FS, filePath string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

func isNotExist(err error) bool {
	return err != nil && errors.Is(err, fs.ErrNotExist)
}
