package sqlgen

import (
	"fmt"
	"strings"
)

// GenerateQuerySQL generates the query.sql content with named INSERT queries
// for sqlc. Entity tables use :one with RETURNING id. Join tables use :exec.
func GenerateQuerySQL(tables []*TableDef) string {
	var b strings.Builder

	for i, td := range tables {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(generateInsertQuery(td))
	}

	return b.String()
}

func generateInsertQuery(td *TableDef) string {
	// Collect non-PK columns for the INSERT.
	var colNames []string
	for _, col := range td.Columns {
		if col.PK {
			continue
		}
		colNames = append(colNames, quoteName(col.Name))
	}

	if len(colNames) == 0 {
		return ""
	}

	funcName := "Insert" + sqlNameToGoName(td.Name)

	// Determine if this is an entity table (has PK with AUTOINCREMENT) or join table.
	hasAutoID := false
	for _, col := range td.Columns {
		if col.PK && col.AutoInc {
			hasAutoID = true
			break
		}
	}

	var b strings.Builder

	if hasAutoID {
		b.WriteString(fmt.Sprintf("-- name: %s :one\n", funcName))
	} else {
		b.WriteString(fmt.Sprintf("-- name: %s :exec\n", funcName))
	}

	b.WriteString(fmt.Sprintf("INSERT INTO %s (\n  ", td.Name))
	b.WriteString(strings.Join(colNames, ",\n  "))
	b.WriteString("\n) VALUES (\n  ")

	placeholders := make([]string, len(colNames))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	b.WriteString(strings.Join(placeholders, ",\n  "))
	b.WriteString("\n)")

	if hasAutoID {
		b.WriteString(" RETURNING id")
	}
	b.WriteString(";\n")

	return b.String()
}

// sqlNameToGoName converts a SQL table name (e.g. "policy_templates") to a
// Go identifier (e.g. "PolicyTemplate"). It singularizes trailing "s".
func sqlNameToGoName(sqlName string) string {
	parts := strings.Split(sqlName, "_")
	var b strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		b.WriteString(string(runes))
	}
	return b.String()
}
