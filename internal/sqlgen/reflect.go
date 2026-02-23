package sqlgen

import (
	"fmt"
	"reflect"
	"strings"
)

// ColumnDef describes a single SQL column derived from a Go struct field
// or from extra_columns configuration.
type ColumnDef struct {
	Name      string // SQL column name
	SQLType   string // TEXT, INTEGER, REAL, BOOLEAN
	NotNull   bool
	Unique    bool
	PK        bool   // PRIMARY KEY
	AutoInc   bool   // AUTOINCREMENT
	FK        string // foreign key table name (e.g. "packages")
	Comment   string // inline column comment
	GoField   string // Go field access path (e.g. "Owner.Github")
	IsJSON    bool   // column stores JSON-serialized value
	IsExtra   bool   // not derived from struct field
	IsEnum    bool   // Go type is a named string type (enum)
	IsPointer bool   // Go type is a pointer (always nullable)
	IsSlice   bool   // Go type is a slice (JSON serialized)
	IsMethod  bool   // value accessed via method call, not field
}

// TableDef describes a SQL table.
type TableDef struct {
	Name    string
	Comment string
	Columns []ColumnDef
	Parent  string // parent table name for FK
	Config  *TableConfig
	GoType  string // Go type name from config
}

// ResolveColumns walks the Go type for a table and produces column definitions.
// It validates that all exported fields are accounted for in the configuration.
func ResolveColumns(tableName string, tc *TableConfig, docs DocMap) ([]ColumnDef, error) {
	var cols []ColumnDef

	// Add id column.
	cols = append(cols, ColumnDef{
		Name:    "id",
		SQLType: "INTEGER",
		NotNull: true,
		PK:      true,
		AutoInc: true,
		Comment: "unique identifier",
	})

	// Add parent FK column if configured.
	if tc.Parent != "" {
		fkCol := tc.Parent + "_id"
		cols = append(cols, ColumnDef{
			Name:    fkCol,
			SQLType: "INTEGER",
			NotNull: true,
			FK:      tc.Parent,
			Comment: "foreign key to " + tc.Parent,
			IsExtra: true,
		})
	}

	// Add extra columns from config (before struct columns).
	for name, ec := range tc.ExtraColumns {
		// Skip parent FK if already added above.
		if tc.Parent != "" && name == tc.Parent+"_id" {
			continue
		}
		col := ColumnDef{
			Name:    name,
			SQLType: ec.Type,
			NotNull: ec.NotNull,
			Unique:  ec.Unique,
			Comment: ec.Comment,
			IsExtra: true,
		}
		if ec.FK != "" {
			col.FK = ec.FK
		}
		cols = append(cols, col)
	}

	// If there's no Go type, we're done (join tables with only extra columns).
	if tc.Type == "" {
		return cols, nil
	}

	// Special case: Processor has no standard struct tags.
	if tc.Type == "Processor" {
		return cols, nil
	}

	rt, ok := LookupType(tc.Type)
	if !ok {
		return nil, fmt.Errorf("unknown type %q for table %q", tc.Type, tableName)
	}

	// Build lookup sets.
	inlineSet := toSet(tc.Inline)
	jsonColSet := toSet(tc.JSONColumns)
	excludeSet := toSet(tc.Exclude)

	// Walk struct fields.
	structCols, err := walkStruct(rt, "", "", inlineSet, jsonColSet, excludeSet, tc.Columns, docs)
	if err != nil {
		return nil, fmt.Errorf("table %q: %w", tableName, err)
	}
	cols = append(cols, structCols...)

	// Validate: check all exported fields are accounted for.
	if err := validateFieldCoverage(rt, "", inlineSet, jsonColSet, excludeSet, tc); err != nil {
		return nil, fmt.Errorf("table %q: %w", tableName, err)
	}

	return cols, nil
}

// walkStruct recursively walks a struct type and produces column definitions.
func walkStruct(rt reflect.Type, prefix, goPrefix string, inline, jsonCols, exclude map[string]bool, overrides map[string]*ColumnOverride, docs DocMap) ([]ColumnDef, error) {
	var cols []ColumnDef

	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)

		// Skip unexported fields.
		if !sf.IsExported() {
			continue
		}

		fieldName := sf.Name
		fullFieldName := fieldName
		if goPrefix != "" {
			fullFieldName = goPrefix + fieldName
		}

		// Skip excluded fields.
		if exclude[fieldName] || exclude[fullFieldName] {
			continue
		}

		// Emit source location columns for FileMetadata.
		if sf.Type.Name() == "FileMetadata" {
			cols = append(cols,
				ColumnDef{Name: "file_path", SQLType: "TEXT", Comment: "source file path", GoField: "FilePath", IsMethod: true},
				ColumnDef{Name: "file_line", SQLType: "INTEGER", Comment: "source file line number", GoField: "Line", IsMethod: true},
				ColumnDef{Name: "file_column", SQLType: "INTEGER", Comment: "source file column number", GoField: "Column", IsMethod: true},
			)
			continue
		}

		// Handle Go embedded (anonymous) struct fields — promote their
		// fields into the current table without a prefix.
		if sf.Anonymous {
			fieldType := sf.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				embeddedCols, err := walkStruct(fieldType, prefix, goPrefix, inline, jsonCols, exclude, overrides, docs)
				if err != nil {
					return nil, err
				}
				cols = append(cols, embeddedCols...)
				continue
			}
		}

		// Check if this is an inline field.
		if inline[fieldName] || inline[fullFieldName] {
			fieldType := sf.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() != reflect.Struct {
				return nil, fmt.Errorf("inline field %q is not a struct", fullFieldName)
			}
			inlineCols, err := walkStruct(fieldType, prefix+ToSQLName(fieldName)+"_", fullFieldName+".", inline, jsonCols, exclude, overrides, docs)
			if err != nil {
				return nil, err
			}
			cols = append(cols, inlineCols...)
			continue
		}

		// Check if this should be a JSON column.
		if jsonCols[fieldName] || jsonCols[fullFieldName] {
			sqlName := prefix + jsonColumnName(fieldName)
			comment := "JSON-encoded " + fieldName
			if doc := docs[rt.Name()+"."+fieldName]; doc != "" {
				comment = doc
			}
			if override, ok := overrides[sqlName]; ok && override.Comment != "" {
				comment = override.Comment
			}
			cols = append(cols, ColumnDef{
				Name:    sqlName,
				SQLType: "JSON",
				Comment: comment,
				GoField: fullFieldName,
				IsJSON:  true,
			})
			continue
		}

		// Determine the JSON tag for column naming.
		jsonTag := getJSONName(sf)
		if jsonTag == "-" {
			continue
		}
		sqlName := prefix + jsonTag
		omitempty := hasOmitempty(sf)
		docComment := docs[rt.Name()+"."+fieldName]

		col, err := goTypeToColumn(sf.Type, sqlName, fullFieldName, docComment, omitempty, overrides)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", fullFieldName, err)
		}
		cols = append(cols, col)
	}

	return cols, nil
}

// goTypeToColumn maps a Go type to a ColumnDef.
func goTypeToColumn(t reflect.Type, sqlName, goField, docComment string, omitempty bool, overrides map[string]*ColumnOverride) (ColumnDef, error) {
	col := ColumnDef{
		Name:    sqlName,
		GoField: goField,
		Comment: goField,
	}

	// Apply Go doc comment if available.
	if docComment != "" {
		col.Comment = docComment
	}

	// Apply column overrides (highest priority).
	if override, ok := overrides[sqlName]; ok {
		if override.Comment != "" {
			col.Comment = override.Comment
		}
		if override.Type != "" {
			col.SQLType = override.Type
		}
		if override.NotNull != nil {
			col.NotNull = *override.NotNull
		}
		col.Unique = override.Unique
	}

	// Handle pointer types.
	isPointer := false
	if t.Kind() == reflect.Pointer {
		isPointer = true
		t = t.Elem()
	}
	col.IsPointer = isPointer

	// If type was overridden, use it.
	if col.SQLType != "" {
		if !isPointer && !omitempty {
			col.NotNull = true
		}
		return col, nil
	}

	switch t.Kind() {
	case reflect.String:
		col.SQLType = "TEXT"
		if isNamedStringType(t) {
			col.IsEnum = true
		}
		if !isPointer && !omitempty {
			col.NotNull = true
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		col.SQLType = "INTEGER"
		if !isPointer && !omitempty {
			col.NotNull = true
		}

	case reflect.Float32, reflect.Float64:
		col.SQLType = "REAL"
		if !isPointer && !omitempty {
			col.NotNull = true
		}

	case reflect.Bool:
		col.SQLType = "BOOLEAN"
		if isPointer {
			// *bool is always nullable
		} else if !omitempty {
			col.NotNull = true
		}

	case reflect.Slice:
		// Slices are stored as JSON.
		col.SQLType = "JSON"
		col.IsJSON = true
		col.IsSlice = true

	case reflect.Map:
		// Maps are stored as JSON.
		col.SQLType = "JSON"
		col.IsJSON = true

	case reflect.Interface:
		// any / interface{} stored as JSON.
		col.SQLType = "JSON"
		col.IsJSON = true

	case reflect.Struct:
		// Check for time.Time.
		if t.PkgPath() == "time" && t.Name() == "Time" {
			col.SQLType = "TEXT"
			if isPointer {
				// *time.Time is always nullable
			} else if !omitempty {
				col.NotNull = true
			}
			return col, nil
		}
		// Other structs stored as JSON.
		col.SQLType = "JSON"
		col.IsJSON = true

	default:
		return col, fmt.Errorf("unsupported Go type %v for column %q", t, sqlName)
	}

	return col, nil
}

// validateFieldCoverage checks that every exported field in the Go type
// is accounted for in the table configuration.
func validateFieldCoverage(rt reflect.Type, prefix string, inline, jsonCols, exclude map[string]bool, tc *TableConfig) error {
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}

		fieldName := sf.Name
		fullFieldName := fieldName
		if prefix != "" {
			fullFieldName = prefix + fieldName
		}

		// FileMetadata is auto-handled by walkStruct (not user-configured).
		if sf.Type.Name() == "FileMetadata" {
			continue
		}

		// Handle Go embedded (anonymous) struct fields — validate their
		// promoted fields.
		if sf.Anonymous {
			fieldType := sf.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				if err := validateFieldCoverage(fieldType, prefix, inline, jsonCols, exclude, tc); err != nil {
					return err
				}
				continue
			}
		}

		// Check if accounted for.
		if exclude[fieldName] || exclude[fullFieldName] {
			continue
		}
		if inline[fieldName] || inline[fullFieldName] {
			// Recurse into inline struct to validate its fields too.
			fieldType := sf.Type
			if fieldType.Kind() == reflect.Pointer {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				if err := validateFieldCoverage(fieldType, fullFieldName+".", inline, jsonCols, exclude, tc); err != nil {
					return err
				}
			}
			continue
		}
		if jsonCols[fieldName] || jsonCols[fullFieldName] {
			continue
		}

		// Check if it has a json tag (meaning it's a regular mapped field).
		jsonTag := getJSONName(sf)
		if jsonTag == "-" {
			continue
		}

		// The field should be auto-mapped. Verify it's a supported type.
		ft := sf.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		switch ft.Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Float32, reflect.Float64, reflect.Bool,
			reflect.Slice, reflect.Map, reflect.Interface:
			// Supported.
		case reflect.Struct:
			if ft.PkgPath() == "time" && ft.Name() == "Time" {
				continue // time.Time is supported.
			}
			// Struct fields that aren't inlined, excluded, or JSON-column'd are unmapped.
			return fmt.Errorf("field %q on %s is not mapped in tables.yml — add it to columns, json_columns, inline, or exclude", fullFieldName, rt.Name())
		default:
			return fmt.Errorf("field %q on %s has unsupported type %v", fullFieldName, rt.Name(), ft)
		}
	}
	return nil
}

// getJSONName extracts the JSON field name from a struct field's tag.
func getJSONName(sf reflect.StructField) string {
	tag := sf.Tag.Get("json")
	if tag == "" {
		return ToSQLName(sf.Name)
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		return ToSQLName(sf.Name)
	}
	return name
}

// hasOmitempty checks if a struct field's json tag includes omitempty.
func hasOmitempty(sf reflect.StructField) bool {
	tag := sf.Tag.Get("json")
	_, rest, _ := strings.Cut(tag, ",")
	return strings.Contains(rest, "omitempty")
}

// isNamedStringType returns true if the type is a named string type (enum).
func isNamedStringType(t reflect.Type) bool {
	return t.Kind() == reflect.String && t.Name() != "string" && t.PkgPath() != ""
}

// jsonColumnName returns the SQL column name for a JSON-stored field.
func jsonColumnName(fieldName string) string {
	return ToSQLName(fieldName)
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}
