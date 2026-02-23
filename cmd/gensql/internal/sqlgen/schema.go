package sqlgen

import (
	"fmt"
	"strings"
)

// sqliteReservedWords contains SQL keywords that must be quoted when used
// as column names.
var sqliteReservedWords = map[string]bool{
	"abort": true, "action": true, "add": true, "after": true, "all": true,
	"alter": true, "analyze": true, "and": true, "as": true, "asc": true,
	"attach": true, "autoincrement": true, "before": true, "begin": true,
	"between": true, "by": true, "cascade": true, "case": true, "cast": true,
	"check": true, "collate": true, "column": true, "commit": true,
	"conflict": true, "constraint": true, "create": true, "cross": true,
	"current": true, "current_date": true, "current_time": true,
	"current_timestamp": true, "database": true, "default": true,
	"deferrable": true, "deferred": true, "delete": true, "desc": true,
	"detach": true, "distinct": true, "do": true, "drop": true, "each": true,
	"else": true, "end": true, "escape": true, "except": true, "exclude": true,
	"exclusive": true, "exists": true, "explain": true, "fail": true,
	"filter": true, "first": true, "following": true, "for": true,
	"foreign": true, "from": true, "full": true, "glob": true, "group": true,
	"groups": true, "having": true, "if": true, "ignore": true,
	"immediate": true, "in": true, "index": true, "indexed": true,
	"initially": true, "inner": true, "insert": true, "instead": true,
	"intersect": true, "into": true, "is": true, "isnull": true, "join": true,
	"key": true, "last": true, "left": true, "like": true, "limit": true,
	"match": true, "natural": true, "no": true, "not": true, "nothing": true,
	"notnull": true, "null": true, "nulls": true, "of": true, "offset": true,
	"on": true, "or": true, "order": true, "others": true, "outer": true,
	"over": true, "partition": true, "plan": true, "pragma": true,
	"preceding": true, "primary": true, "query": true, "raise": true,
	"range": true, "recursive": true, "references": true, "regexp": true,
	"reindex": true, "release": true, "rename": true, "replace": true,
	"restrict": true, "right": true, "rollback": true, "row": true,
	"rows": true, "savepoint": true, "select": true, "set": true,
	"table": true, "temp": true, "temporary": true, "then": true, "ties": true,
	"to": true, "transaction": true, "trigger": true, "unbounded": true,
	"union": true, "unique": true, "update": true, "using": true,
	"vacuum": true, "values": true, "view": true, "virtual": true,
	"when": true, "where": true, "window": true, "with": true, "without": true,
}

// GenerateSchemaSQL generates the schema.sql content with CREATE TABLE statements.
// Comments are placed inside the CREATE TABLE body per R10 so they're preserved
// in sqlite_master.
func GenerateSchemaSQL(tables []*TableDef) string {
	var b strings.Builder

	for i, td := range tables {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(generateCreateTable(td))
	}

	return b.String()
}

// quoteName returns the column name quoted with double quotes if it's a
// reserved SQL word, otherwise returns it unchanged.
func quoteName(name string) string {
	if sqliteReservedWords[strings.ToLower(name)] {
		return `"` + name + `"`
	}
	return name
}

func generateCreateTable(td *TableDef) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n", td.Name))

	// Table description as first comment inside body.
	if td.Comment != "" {
		b.WriteString(fmt.Sprintf("  -- %s\n", td.Comment))
	}

	for i, col := range td.Columns {
		b.WriteString("  ")
		b.WriteString(quoteName(col.Name))
		b.WriteString(" ")
		b.WriteString(col.SQLType)

		if col.PK {
			b.WriteString(" PRIMARY KEY")
			if col.AutoInc {
				b.WriteString(" AUTOINCREMENT")
			}
		}
		if col.NotNull && !col.PK {
			b.WriteString(" NOT NULL")
		}
		if col.Unique {
			b.WriteString(" UNIQUE")
		}
		if col.FK != "" {
			b.WriteString(fmt.Sprintf(" REFERENCES %s(id)", col.FK))
		}

		// Trailing comma unless last column.
		if i < len(td.Columns)-1 {
			b.WriteString(",")
		}

		// Inline comment.
		if col.Comment != "" {
			b.WriteString(fmt.Sprintf(" -- %s", col.Comment))
		}

		b.WriteString("\n")
	}

	b.WriteString(");\n")
	return b.String()
}
