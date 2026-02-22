package pkgreader

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// TransformData represents a fully-loaded Elasticsearch transform within a package.
type TransformData struct {
	Transform pkgspec.Transform
	Manifest  *pkgspec.TransformManifest // nil if absent
	Fields    map[string]*FieldsFile
	path      string
}

// Path returns the transform's directory path relative to the package root.
func (td *TransformData) Path() string {
	return td.path
}

func readTransforms(fsys fs.FS, root string, cfg *config) (map[string]*TransformData, error) {
	transformDir := path.Join(root, "elasticsearch", "transform")

	entries, err := fs.ReadDir(fsys, transformDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading transform directory: %w", err)
	}

	result := make(map[string]*TransformData, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		tPath := path.Join(transformDir, name)

		td, err := readTransform(fsys, tPath, cfg)
		if err != nil {
			return nil, fmt.Errorf("reading transform %s: %w", name, err)
		}
		result[name] = td
	}

	return result, nil
}

func readTransform(fsys fs.FS, tPath string, cfg *config) (*TransformData, error) {
	td := &TransformData{
		path: tPath,
	}

	// Read transform.yml (always with knownFields=false since it contains
	// arbitrary Elasticsearch DSL).
	transformPath := path.Join(tPath, "transform.yml")
	if err := decodeYAML(fsys, transformPath, &td.Transform, false); err != nil {
		return nil, fmt.Errorf("reading transform.yml: %w", err)
	}
	pkgspec.AnnotateFileMetadata(transformPath, &td.Transform)

	// Read manifest.yml (optional).
	manifestPath := path.Join(tPath, "manifest.yml")
	manifest, err := readOptionalYAML[pkgspec.TransformManifest](fsys, manifestPath, cfg.knownFields)
	if err != nil {
		return nil, fmt.Errorf("reading transform manifest: %w", err)
	}
	if manifest != nil {
		pkgspec.AnnotateFileMetadata(manifestPath, manifest)
		td.Manifest = manifest
	}

	// Read fields.
	fieldsDir := path.Join(tPath, "fields")
	fields, err := readFieldsDir(fsys, fieldsDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("reading transform fields: %w", err)
	}
	td.Fields = fields

	return td, nil
}
