package reader

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// KibanaSavedObject represents a Kibana saved object loaded from a JSON file
// in the kibana/ directory. Common top-level fields are typed, while
// type-specific attributes are partially decoded to capture title and
// description with remaining fields stored in Extras.
type KibanaSavedObject struct {
	ID                   string                      `json:"id"`
	Type                 string                      `json:"type"`
	Attributes           KibanaSavedObjectAttributes `json:"attributes"`
	References           []KibanaReference           `json:"references,omitempty"`
	CoreMigrationVersion string                      `json:"coreMigrationVersion,omitempty"`
	TypeMigrationVersion map[string]string           `json:"typeMigrationVersion,omitempty"`
	MigrationVersion     map[string]string           `json:"migrationVersion,omitempty"`
	Managed              *bool                       `json:"managed,omitempty"`
	CreatedAt            string                      `json:"created_at,omitempty"`
	UpdatedAt            string                      `json:"updated_at,omitempty"`
	CreatedBy            string                      `json:"created_by,omitempty"`
	UpdatedBy            string                      `json:"updated_by,omitempty"`
	Version              string                      `json:"version,omitempty"`
	Namespaces           []string                    `json:"namespaces,omitempty"`
	OriginID             string                      `json:"originId,omitempty"`
	path                 string
}

// Path returns the file path relative to the package root.
func (o *KibanaSavedObject) Path() string {
	return o.path
}

// KibanaSavedObjectAttributes holds the common attributes shared across all
// Kibana saved object types. The Title and Description fields are extracted
// from the attributes object, and all other fields are stored in Extras.
type KibanaSavedObjectAttributes struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Extras      map[string]any `json:"-"`
}

// UnmarshalJSON decodes the attributes object, extracting title and description
// into their typed fields and storing all remaining keys in Extras.
func (a *KibanaSavedObjectAttributes) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if v, ok := raw["title"].(string); ok {
		a.Title = v
	}
	if v, ok := raw["description"].(string); ok {
		a.Description = v
	}

	delete(raw, "title")
	delete(raw, "description")

	if len(raw) > 0 {
		a.Extras = raw
	}

	return nil
}

// KibanaReference represents a reference from one Kibana saved object to another.
type KibanaReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// readKibanaObjects scans the kibana/ directory for saved object JSON files,
// returning them grouped by subdirectory name (the asset type, e.g.
// "dashboard", "visualization"). The tags.yml file is skipped since it is
// loaded separately.
func readKibanaObjects(fsys fs.FS, root string) (map[string][]*KibanaSavedObject, error) {
	kibanaDir := path.Join(root, "kibana")

	entries, err := fs.ReadDir(fsys, kibanaDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading kibana directory: %w", err)
	}

	var result map[string][]*KibanaSavedObject
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		assetType := entry.Name()
		assetDir := path.Join(kibanaDir, assetType)

		files, err := fs.ReadDir(fsys, assetDir)
		if err != nil {
			return nil, fmt.Errorf("reading kibana/%s directory: %w", assetType, err)
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}

			filePath := path.Join(assetDir, f.Name())
			data, err := fs.ReadFile(fsys, filePath)
			if err != nil {
				return nil, fmt.Errorf("reading kibana/%s/%s: %w", assetType, f.Name(), err)
			}

			var obj KibanaSavedObject
			if err := json.Unmarshal(data, &obj); err != nil {
				return nil, fmt.Errorf("parsing kibana/%s/%s: %w", assetType, f.Name(), err)
			}
			obj.path = filePath

			if result == nil {
				result = make(map[string][]*KibanaSavedObject)
			}
			result[assetType] = append(result[assetType], &obj)
		}
	}

	return result, nil
}
