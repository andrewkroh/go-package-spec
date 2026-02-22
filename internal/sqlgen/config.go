package sqlgen

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// TablesConfig holds the full table configuration loaded from tables.yml.
type TablesConfig struct {
	Tables map[string]*TableConfig `yaml:"tables"`
}

// TableConfig defines how a Go type maps to a SQL table.
type TableConfig struct {
	// Type is the pkgspec Go type name (e.g. "Manifest", "Field").
	// Empty for join tables that have no source type.
	Type string `yaml:"type"`

	// Comment is the table description, emitted inside the CREATE TABLE body.
	Comment string `yaml:"comment"`

	// Parent is the name of the parent table for FK relationships.
	Parent string `yaml:"parent"`

	// ExtraColumns defines columns not derived from struct fields.
	ExtraColumns map[string]*ExtraColumnConfig `yaml:"extra_columns"`

	// Inline lists struct field names whose sub-fields should be flattened
	// into the parent table's columns with a prefix.
	Inline []string `yaml:"inline"`

	// JSONColumns lists struct field names that should be stored as a single
	// JSON TEXT column rather than normalized.
	JSONColumns []string `yaml:"json_columns"`

	// Exclude lists struct field names to skip during column generation.
	Exclude []string `yaml:"exclude"`

	// Columns provides per-column overrides (comment, type, etc.).
	Columns map[string]*ColumnOverride `yaml:"columns"`

	// Flatten indicates the type should be flattened before insertion
	// (e.g. fields via FlattenFields, processors via FlattenProcessors).
	Flatten bool `yaml:"flatten"`
}

// ExtraColumnConfig defines a column not derived from a struct field.
type ExtraColumnConfig struct {
	Type    string `yaml:"type"`
	NotNull bool   `yaml:"not_null"`
	Unique  bool   `yaml:"unique"`
	Comment string `yaml:"comment"`
	FK      string `yaml:"fk"`
}

// ColumnOverride provides per-column overrides for generated columns.
type ColumnOverride struct {
	Comment string `yaml:"comment"`
	Type    string `yaml:"type"`
	NotNull *bool  `yaml:"not_null"`
	Unique  bool   `yaml:"unique"`
}

// LoadConfig reads and parses the tables.yml configuration file.
func LoadConfig(path string) (*TablesConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg TablesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if cfg.Tables == nil {
		return nil, fmt.Errorf("no tables defined in %s", path)
	}

	return &cfg, nil
}
