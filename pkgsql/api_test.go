package pkgsql_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	"github.com/andrewkroh/go-package-spec/pkgreader"
	"github.com/andrewkroh/go-package-spec/pkgsql"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTableSchemas(t *testing.T) {
	schemas := pkgsql.TableSchemas()
	if len(schemas) == 0 {
		t.Fatal("expected at least one table schema")
	}
	for _, s := range schemas {
		if !strings.HasPrefix(s, "CREATE TABLE IF NOT EXISTS") {
			t.Errorf("expected CREATE TABLE prefix, got: %s", s[:50])
		}
	}
}

func TestTableSchemasContainComments(t *testing.T) {
	schemas := pkgsql.TableSchemas()
	for _, s := range schemas {
		if !strings.Contains(s, "-- ") {
			t.Errorf("expected inline comments in schema: %s", s[:50])
		}
	}
}

func TestSqliteMasterPreservesComments(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, ddl := range pkgsql.TableSchemas() {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("executing DDL: %v", err)
		}
	}

	rows, err := db.QueryContext(ctx, "SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name, ddl string
		if err := rows.Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		count++
		if !strings.Contains(ddl, "-- ") {
			t.Errorf("table %s: expected comments in sqlite_master.sql", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no tables found in sqlite_master")
	}
}

func TestWritePackage(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-package
title: Test Package
version: 1.0.0
description: A test package.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
categories:
  - security
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
    - description: Bug fix
      type: bugfix
      link: https://github.com/test/2
`)},
		"data_stream/logs/manifest.yml": {Data: []byte(`
title: Log Events
type: logs
streams:
  - input: logfile
    title: Log Files
    description: Collect log files with Filebeat.
    vars:
      - name: paths
        type: text
        title: Paths
        multi: true
        required: true
        show_user: true
        default:
          - /var/log/*.log
`)},
		"data_stream/logs/fields/base-fields.yml": {Data: []byte(`
- name: "@timestamp"
  type: date
  description: Event timestamp.
- name: message
  type: text
  description: Log message.
- name: log
  type: group
  fields:
    - name: level
      type: keyword
      description: Log level.
`)},
	}

	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys))
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	err = pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg})
	if err != nil {
		t.Fatalf("writing packages: %v", err)
	}

	// Verify package was inserted.
	var name, version, pkgType string
	err = db.QueryRowContext(ctx, "SELECT name, version, type FROM packages WHERE name = 'test-package'").
		Scan(&name, &version, &pkgType)
	if err != nil {
		t.Fatalf("querying package: %v", err)
	}
	if name != "test-package" || version != "1.0.0" || pkgType != "integration" {
		t.Errorf("got name=%s version=%s type=%s", name, version, pkgType)
	}

	// Verify categories.
	var catCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM package_categories WHERE package_id = 1").Scan(&catCount)
	if err != nil {
		t.Fatalf("querying categories: %v", err)
	}
	if catCount != 1 {
		t.Errorf("expected 1 category, got %d", catCount)
	}

	// Verify changelog entries.
	var entryCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM changelog_entries").Scan(&entryCount)
	if err != nil {
		t.Fatalf("querying entries: %v", err)
	}
	if entryCount != 2 {
		t.Errorf("expected 2 changelog entries, got %d", entryCount)
	}

	// Verify data stream.
	var dsTitle, dsDirName string
	err = db.QueryRowContext(ctx, "SELECT title, dir_name FROM data_streams WHERE dir_name = 'logs'").
		Scan(&dsTitle, &dsDirName)
	if err != nil {
		t.Fatalf("querying data stream: %v", err)
	}
	if dsTitle != "Log Events" || dsDirName != "logs" {
		t.Errorf("got title=%s dir_name=%s", dsTitle, dsDirName)
	}

	// Verify streams.
	var streamCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM streams").Scan(&streamCount)
	if err != nil {
		t.Fatalf("querying streams: %v", err)
	}
	if streamCount != 1 {
		t.Errorf("expected 1 stream, got %d", streamCount)
	}

	// Verify stream vars.
	var varCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM stream_vars").Scan(&varCount)
	if err != nil {
		t.Fatalf("querying stream vars: %v", err)
	}
	if varCount != 1 {
		t.Errorf("expected 1 stream var, got %d", varCount)
	}

	// Verify flattened fields.
	var fieldCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM fields").Scan(&fieldCount)
	if err != nil {
		t.Fatalf("querying fields: %v", err)
	}
	if fieldCount != 3 {
		t.Errorf("expected 3 flattened fields (@timestamp, message, log.level), got %d", fieldCount)
	}

	// Verify field names are flattened.
	var fieldName string
	err = db.QueryRowContext(ctx, "SELECT name FROM fields WHERE name LIKE 'log.%'").Scan(&fieldName)
	if err != nil {
		t.Fatalf("querying flattened field: %v", err)
	}
	if fieldName != "log.level" {
		t.Errorf("expected log.level, got %s", fieldName)
	}

	// Verify data_stream_fields join.
	var joinCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM data_stream_fields").Scan(&joinCount)
	if err != nil {
		t.Fatalf("querying data_stream_fields: %v", err)
	}
	if joinCount != 3 {
		t.Errorf("expected 3 data_stream_fields, got %d", joinCount)
	}
}
