package sqlgen

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for a SQL generator run.
type Config struct {
	TablesFile  string // Path to tables.yml
	OutputDir   string // Output directory for generated files
	PackageName string // Go package name
}

// Run executes the full SQL generation pipeline.
func Run(cfg Config) error {
	// 1. Load table configuration.
	tablesConfig, err := LoadConfig(cfg.TablesFile)
	if err != nil {
		return fmt.Errorf("loading tables config: %w", err)
	}

	// 2. Resolve types and generate column definitions.
	tableDefs := make(map[string]*TableDef, len(tablesConfig.Tables))
	for name, tc := range tablesConfig.Tables {
		cols, err := ResolveColumns(name, tc)
		if err != nil {
			return fmt.Errorf("resolving columns: %w", err)
		}
		tableDefs[name] = &TableDef{
			Name:    name,
			Comment: tc.Comment,
			Columns: cols,
			Parent:  tc.Parent,
			Config:  tc,
			GoType:  tc.Type,
		}
	}

	// 3. Topological sort by FK dependencies.
	sortedNames := SortTables(tableDefs)
	var sortedTables []*TableDef
	for _, name := range sortedNames {
		if td, ok := tableDefs[name]; ok {
			sortedTables = append(sortedTables, td)
		}
	}

	// 4. Create output directory.
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	// 5. Generate schema.sql.
	schemaSQL := GenerateSchemaSQL(sortedTables)
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "schema.sql"), []byte(schemaSQL), 0o644); err != nil {
		return fmt.Errorf("writing schema.sql: %w", err)
	}

	// 6. Generate query.sql.
	querySQL := GenerateQuerySQL(sortedTables)
	if err := os.WriteFile(filepath.Join(cfg.OutputDir, "query.sql"), []byte(querySQL), 0o644); err != nil {
		return fmt.Errorf("writing query.sql: %w", err)
	}

	// 7. Generate tables.go.
	pkgName := cfg.PackageName
	if pkgName == "" {
		pkgName = "pkgsql"
	}
	if err := EmitTablesGo(pkgName, cfg.OutputDir, sortedTables); err != nil {
		return fmt.Errorf("writing tables.go: %w", err)
	}

	// 8. Generate insert.go.
	if err := EmitInsertGo(pkgName, cfg.OutputDir, sortedTables); err != nil {
		return fmt.Errorf("writing insert.go: %w", err)
	}

	return nil
}
