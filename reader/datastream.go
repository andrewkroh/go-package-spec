package reader

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/andrewkroh/go-package-spec/packagespec"
)

// DataStream represents a fully-loaded data stream within an integration package.
type DataStream struct {
	Manifest  packagespec.DataStreamManifest
	Fields    map[string]*FieldsFile   // keyed by filename
	Pipelines map[string]*PipelineFile // keyed by filename (e.g., "default.yml")
	Lifecycle *packagespec.Lifecycle   // nil if absent
	path      string
}

// Path returns the data stream's directory path relative to the package root.
func (ds *DataStream) Path() string {
	return ds.path
}

// AllFields returns all fields from all field files in the data stream.
func (ds *DataStream) AllFields() []packagespec.Field {
	var all []packagespec.Field
	for _, ff := range ds.Fields {
		all = append(all, ff.Fields...)
	}
	return all
}

// FieldsFile represents a single fields YAML file.
type FieldsFile struct {
	Fields []packagespec.Field
	path   string
}

// Path returns the file path relative to the package root.
func (ff *FieldsFile) Path() string {
	return ff.path
}

// PipelineFile represents a single ingest pipeline YAML file.
type PipelineFile struct {
	Pipeline packagespec.IngestPipeline
	path     string
}

// Path returns the file path relative to the package root.
func (pf *PipelineFile) Path() string {
	return pf.path
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
	packagespec.AnnotateFileMetadata(manifestPath, &ds.Manifest)

	// Read lifecycle (optional).
	lifecyclePath := path.Join(dsPath, "lifecycle.yml")
	lifecycle, err := readOptionalYAML[packagespec.Lifecycle](fsys, lifecyclePath, cfg.knownFields)
	if err != nil {
		return nil, fmt.Errorf("reading lifecycle: %w", err)
	}
	if lifecycle != nil {
		packagespec.AnnotateFileMetadata(lifecyclePath, lifecycle)
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
	var fields []packagespec.Field
	if err := decodeYAML(fsys, filePath, &fields, cfg.knownFields); err != nil {
		return nil, err
	}

	packagespec.AnnotateFileMetadata(filePath, &fields)

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
		var pipeline packagespec.IngestPipeline
		// Pipelines contain arbitrary ES DSL; always decode with knownFields=false.
		if err := decodeYAML(fsys, filePath, &pipeline, false); err != nil {
			return nil, fmt.Errorf("reading pipeline file %s: %w", name, err)
		}
		packagespec.AnnotateFileMetadata(filePath, &pipeline)

		result[name] = &PipelineFile{
			Pipeline: pipeline,
			path:     filePath,
		}
	}

	return result, nil
}

func isNotExist(err error) bool {
	return err != nil && errors.Is(err, fs.ErrNotExist)
}
