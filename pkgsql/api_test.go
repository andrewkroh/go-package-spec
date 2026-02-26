package pkgsql_test

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
		if !strings.HasPrefix(s, "CREATE TABLE IF NOT EXISTS") && !strings.HasPrefix(s, "CREATE VIRTUAL TABLE IF NOT EXISTS") && !strings.HasPrefix(s, "CREATE VIEW IF NOT EXISTS") {
			t.Errorf("expected CREATE TABLE, CREATE VIRTUAL TABLE, or CREATE VIEW prefix, got: %s", s[:50])
		}
	}
}

func TestTableSchemasContainComments(t *testing.T) {
	schemas := pkgsql.TableSchemas()
	for _, s := range schemas {
		// FTS5 virtual tables and views don't have inline comments.
		if strings.HasPrefix(s, "CREATE VIRTUAL TABLE") || strings.HasPrefix(s, "CREATE VIEW") {
			continue
		}
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

	// Only check regular tables (not FTS5 virtual tables or FTS5 internal tables).
	rows, err := db.QueryContext(ctx, "SELECT name, sql FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '%_fts%' ORDER BY name")
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

func TestJSONColumnType(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	for _, ddl := range pkgsql.TableSchemas() {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("executing DDL: %v", err)
		}
	}

	// Verify that JSON columns use the JSON type in the schema.
	var schemaDDL string
	err := db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'fields'").Scan(&schemaDDL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(schemaDDL, "multi_fields JSON") {
		t.Error("expected multi_fields JSON in fields schema")
	}
	if !strings.Contains(schemaDDL, "example JSON") {
		t.Error("expected example JSON in fields schema")
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
conditions:
  kibana:
    version: ^8.0.0
  elastic:
    subscription: basic
agent:
  privileges:
    root: true
elasticsearch:
  privileges:
    cluster:
      - monitor
      - manage_ilm
policy_templates:
  - name: test-policy
    title: Test Policy
    description: A test policy template.
    icons:
      - src: /img/policy-icon.svg
        title: Policy Icon
    screenshots:
      - src: /img/policy-shot.png
        title: Policy Screenshot
    inputs:
      - type: logfile
        title: Log File
        description: Collect log files.
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
		"data_stream/logs/sample_event.json": {Data: []byte(`{"@timestamp": "2024-01-01T00:00:00Z", "message": "test"}`)},
		"docs/README.md":                     {Data: []byte("# Test Package\n")},
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

	// Verify conditions.
	var condKibana, condElastic sql.NullString
	err = db.QueryRowContext(ctx, "SELECT conditions_kibana_version, conditions_elastic_subscription FROM packages WHERE name = 'test-package'").
		Scan(&condKibana, &condElastic)
	if err != nil {
		t.Fatalf("querying conditions: %v", err)
	}
	if !condKibana.Valid || condKibana.String != "^8.0.0" {
		t.Errorf("expected conditions_kibana_version=^8.0.0, got %v", condKibana)
	}
	if !condElastic.Valid || condElastic.String != "basic" {
		t.Errorf("expected conditions_elastic_subscription=basic, got %v", condElastic)
	}

	// Verify agent privileges.
	var agentRoot sql.NullBool
	err = db.QueryRowContext(ctx, "SELECT agent_privileges_root FROM packages WHERE name = 'test-package'").
		Scan(&agentRoot)
	if err != nil {
		t.Fatalf("querying agent privileges: %v", err)
	}
	if !agentRoot.Valid || !agentRoot.Bool {
		t.Errorf("expected agent_privileges_root=true, got %v", agentRoot)
	}

	// Verify elasticsearch privileges.
	var esPrivs sql.NullString
	err = db.QueryRowContext(ctx, "SELECT elasticsearch_privileges_cluster FROM packages WHERE name = 'test-package'").
		Scan(&esPrivs)
	if err != nil {
		t.Fatalf("querying ES privileges: %v", err)
	}
	if !esPrivs.Valid || !strings.Contains(esPrivs.String, "monitor") {
		t.Errorf("expected elasticsearch_privileges_cluster to contain monitor, got %v", esPrivs)
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

	// Verify sample event.
	var sampleEvent string
	err = db.QueryRowContext(ctx, "SELECT event FROM sample_events").Scan(&sampleEvent)
	if err != nil {
		t.Fatalf("querying sample event: %v", err)
	}
	if !strings.Contains(sampleEvent, "test") {
		t.Errorf("expected sample event to contain 'test', got %s", sampleEvent)
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

	// Verify policy template icons.
	var ptIconCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM policy_template_icons").Scan(&ptIconCount)
	if err != nil {
		t.Fatalf("querying policy template icons: %v", err)
	}
	if ptIconCount != 1 {
		t.Errorf("expected 1 policy template icon, got %d", ptIconCount)
	}

	// Verify policy template screenshots.
	var ptScreenshotCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM policy_template_screenshots").Scan(&ptScreenshotCount)
	if err != nil {
		t.Fatalf("querying policy template screenshots: %v", err)
	}
	if ptScreenshotCount != 1 {
		t.Errorf("expected 1 policy template screenshot, got %d", ptScreenshotCount)
	}

	// Verify docs row inserted with NULL content (no WithDocContent).
	var docPath, docContentType string
	var docContent sql.NullString
	err = db.QueryRowContext(ctx, "SELECT file_path, content_type, content FROM docs WHERE file_path = 'docs/README.md'").
		Scan(&docPath, &docContentType, &docContent)
	if err != nil {
		t.Fatalf("querying doc: %v", err)
	}
	if docContentType != "readme" {
		t.Errorf("expected content_type=readme, got %s", docContentType)
	}
	if docContent.Valid {
		t.Errorf("expected NULL content without WithDocContent, got %q", docContent.String)
	}
}

// png1x1 is a minimal 1x1 red PNG image for testing.
var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x10, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x62, 0xfa, 0xcf, 0xc0, 0x00,
	0x08, 0x00, 0x00, 0xff, 0xff, 0x03, 0x09, 0x01, 0x02, 0x58, 0xb6, 0xd5, 0x50, 0x00, 0x00, 0x00,
	0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestWritePackageWithImages(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: img-test
title: Image Test
version: 1.0.0
description: A package with images.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
icons:
  - src: /img/icon.png
    title: Icon
screenshots:
  - src: /img/screenshot.png
    title: Screenshot
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"img/icon.png":       {Data: png1x1},
		"img/screenshot.png": {Data: png1x1},
	}

	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys), pkgreader.WithImageMetadata())
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	err = pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg})
	if err != nil {
		t.Fatalf("writing packages: %v", err)
	}

	// Verify images were inserted.
	var imgCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM images").Scan(&imgCount)
	if err != nil {
		t.Fatalf("querying images: %v", err)
	}
	if imgCount != 2 {
		t.Errorf("expected 2 images, got %d", imgCount)
	}

	// Verify image metadata.
	var src, sha256 string
	var width, height sql.NullInt64
	var byteSize int64
	err = db.QueryRowContext(ctx, "SELECT src, width, height, byte_size, sha256 FROM images WHERE src = '/img/icon.png'").
		Scan(&src, &width, &height, &byteSize, &sha256)
	if err != nil {
		t.Fatalf("querying image: %v", err)
	}
	if !width.Valid || width.Int64 != 1 {
		t.Errorf("expected width=1, got %v", width)
	}
	if !height.Valid || height.Int64 != 1 {
		t.Errorf("expected height=1, got %v", height)
	}
	if byteSize != int64(len(png1x1)) {
		t.Errorf("expected byte_size=%d, got %d", len(png1x1), byteSize)
	}
	if sha256 == "" || len(sha256) != 64 {
		t.Errorf("expected 64-char hex SHA256, got %q", sha256)
	}

	// Verify join with package_icons works via src.
	var joinCount int
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM package_icons i JOIN images img ON i.src = img.src AND i.packages_id = img.packages_id").
		Scan(&joinCount)
	if err != nil {
		t.Fatalf("querying icon-image join: %v", err)
	}
	if joinCount != 1 {
		t.Errorf("expected 1 icon-image join, got %d", joinCount)
	}

	// Verify join with package_screenshots works via src.
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM package_screenshots s JOIN images img ON s.src = img.src AND s.packages_id = img.packages_id").
		Scan(&joinCount)
	if err != nil {
		t.Fatalf("querying screenshot-image join: %v", err)
	}
	if joinCount != 1 {
		t.Errorf("expected 1 screenshot-image join, got %d", joinCount)
	}
}

func TestWriteInputPackagePolicyTemplates(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-input
title: Test Input
version: 1.0.0
description: A test input package.
format_version: 3.5.7
type: input
categories:
  - custom
conditions:
  kibana:
    version: ^8.0.0
  elastic:
    subscription: basic
policy_templates:
  - name: test-input-pt
    type: logs
    title: Test Input Policy
    description: Collect data from an API.
    input: httpjson
    template_path: input.yml.hbs
    vars:
      - name: url
        type: text
        title: API URL
        required: true
        show_user: true
        default: https://example.com/api
      - name: interval
        type: text
        title: Interval
        required: true
        show_user: true
        default: 1m
owner:
  github: elastic/integrations
  type: elastic
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"agent/input/input.yml.hbs": {Data: []byte(`# placeholder`)},
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

	// Verify package type is input.
	var pkgType string
	err = db.QueryRowContext(ctx, "SELECT type FROM packages WHERE name = 'test-input'").Scan(&pkgType)
	if err != nil {
		t.Fatalf("querying package: %v", err)
	}
	if pkgType != "input" {
		t.Errorf("expected type=input, got %s", pkgType)
	}

	// Verify policy template was inserted.
	var ptCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM policy_templates").Scan(&ptCount)
	if err != nil {
		t.Fatalf("querying policy_templates: %v", err)
	}
	if ptCount != 1 {
		t.Errorf("expected 1 policy template, got %d", ptCount)
	}

	// Verify input-specific fields.
	var ptName, ptDesc string
	var ptInput, ptTemplatePath, ptType sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT name, description, input, template_path, policy_template_type FROM policy_templates").
		Scan(&ptName, &ptDesc, &ptInput, &ptTemplatePath, &ptType)
	if err != nil {
		t.Fatalf("querying policy template: %v", err)
	}
	if ptName != "test-input-pt" {
		t.Errorf("expected name=test-input-pt, got %s", ptName)
	}
	if ptDesc != "Collect data from an API." {
		t.Errorf("expected description='Collect data from an API.', got %s", ptDesc)
	}
	if !ptInput.Valid || ptInput.String != "httpjson" {
		t.Errorf("expected input=httpjson, got %v", ptInput)
	}
	if !ptTemplatePath.Valid || ptTemplatePath.String != "agent/input/input.yml.hbs" {
		t.Errorf("expected template_path=agent/input/input.yml.hbs, got %v", ptTemplatePath)
	}
	if !ptType.Valid || ptType.String != "logs" {
		t.Errorf("expected policy_template_type=logs, got %v", ptType)
	}

	// Verify integration-only fields are NULL for input policy templates.
	var ptMultiple sql.NullBool
	var ptDataStreams sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT multiple, data_streams FROM policy_templates").
		Scan(&ptMultiple, &ptDataStreams)
	if err != nil {
		t.Fatalf("querying integration-only fields: %v", err)
	}
	if ptMultiple.Valid {
		t.Errorf("expected NULL multiple for input policy template, got %v", ptMultiple)
	}
	if ptDataStreams.Valid {
		t.Errorf("expected NULL data_streams for input policy template, got %v", ptDataStreams)
	}

	// Verify policy template vars were inserted.
	var varCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM policy_template_vars").Scan(&varCount)
	if err != nil {
		t.Fatalf("querying policy_template_vars: %v", err)
	}
	if varCount != 2 {
		t.Errorf("expected 2 policy template vars, got %d", varCount)
	}

	// Verify var names via join.
	var varName string
	err = db.QueryRowContext(ctx, `
		SELECT v.name
		FROM policy_template_vars ptv
		JOIN vars v ON v.id = ptv.var_id
		JOIN policy_templates pt ON pt.id = ptv.policy_template_id
		WHERE v.name = 'url'`).Scan(&varName)
	if err != nil {
		t.Fatalf("querying var join: %v", err)
	}
	if varName != "url" {
		t.Errorf("expected var name=url, got %s", varName)
	}

	// Verify join to packages works.
	var pkgName string
	err = db.QueryRowContext(ctx, `
		SELECT p.name
		FROM policy_templates pt
		JOIN packages p ON p.id = pt.packages_id
		WHERE pt.name = 'test-input-pt'`).Scan(&pkgName)
	if err != nil {
		t.Fatalf("querying package join: %v", err)
	}
	if pkgName != "test-input" {
		t.Errorf("expected test-input, got %s", pkgName)
	}
}

func TestWriteContentPackage(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-content
title: Test Content Package
version: 1.0.0
description: A test content package.
format_version: 3.5.7
type: content
owner:
  github: elastic/security
  type: elastic
conditions:
  kibana:
    version: ^8.12.0
  elastic:
    subscription: platinum
discovery:
  fields:
    - name: event.kind
    - name: event.category
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
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

	// Verify package type.
	var pkgType string
	err = db.QueryRowContext(ctx, "SELECT type FROM packages WHERE name = 'test-content'").Scan(&pkgType)
	if err != nil {
		t.Fatalf("querying package: %v", err)
	}
	if pkgType != "content" {
		t.Errorf("expected type=content, got %s", pkgType)
	}

	// Verify conditions.
	var condKibana, condElastic sql.NullString
	err = db.QueryRowContext(ctx, "SELECT conditions_kibana_version, conditions_elastic_subscription FROM packages WHERE name = 'test-content'").
		Scan(&condKibana, &condElastic)
	if err != nil {
		t.Fatalf("querying conditions: %v", err)
	}
	if !condKibana.Valid || condKibana.String != "^8.12.0" {
		t.Errorf("expected conditions_kibana_version=^8.12.0, got %v", condKibana)
	}
	if !condElastic.Valid || condElastic.String != "platinum" {
		t.Errorf("expected conditions_elastic_subscription=platinum, got %v", condElastic)
	}

	// Verify discovery fields.
	var dfCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM discovery_fields").Scan(&dfCount)
	if err != nil {
		t.Fatalf("querying discovery fields: %v", err)
	}
	if dfCount != 2 {
		t.Errorf("expected 2 discovery fields, got %d", dfCount)
	}

	// Verify discovery field names.
	var dfName string
	err = db.QueryRowContext(ctx, "SELECT name FROM discovery_fields ORDER BY name LIMIT 1").Scan(&dfName)
	if err != nil {
		t.Fatalf("querying discovery field name: %v", err)
	}
	if dfName != "event.category" {
		t.Errorf("expected event.category, got %s", dfName)
	}
}

func TestWritePackageWithDocContent(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: doc-test
title: Doc Test
version: 1.0.0
description: A package with docs.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"docs/README.md": {Data: []byte(`# Doc Test Package

This package provides authentication monitoring and troubleshooting guidance.

**Exported fields**

| Field | Description | Type |
|---|---|---|
| event.timeout | Timeout duration. | keyword |
| nginx.access.remote_ip_list | Remote IP list. | keyword |

An example event for ` + "`access`" + ` looks as following:

` + "```json" + `
{
    "@timestamp": "2022-12-09T10:39:23.000Z",
    "event.timeout": "30s"
}
` + "```" + `

## Troubleshooting

Check the timeout settings if connections fail.
`)},
		"docs/getting-started.md": {Data: []byte(`# Getting Started

Follow these steps to configure authentication monitoring.
`)},
		"docs/knowledge_base/troubleshooting.md": {Data: []byte(`# Troubleshooting

If you see a certificate error, check your TLS configuration.
`)},
	}

	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys))
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	// Use WithDocContent with a closure over fsys.
	docReader := func(_, docPath string) ([]byte, error) {
		return fs.ReadFile(fsys, docPath)
	}

	err = pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg}, pkgsql.WithDocContent(docReader))
	if err != nil {
		t.Fatalf("writing packages: %v", err)
	}

	// Verify all 3 docs were inserted.
	var docCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM docs").Scan(&docCount)
	if err != nil {
		t.Fatalf("querying docs: %v", err)
	}
	if docCount != 3 {
		t.Errorf("expected 3 docs, got %d", docCount)
	}

	// Verify content is non-NULL and field table was stripped.
	var content sql.NullString
	err = db.QueryRowContext(ctx, "SELECT content FROM docs WHERE file_path = 'docs/README.md'").Scan(&content)
	if err != nil {
		t.Fatalf("querying doc content: %v", err)
	}
	if !content.Valid {
		t.Fatal("expected non-NULL content with WithDocContent")
	}
	if !strings.Contains(content.String, "authentication") {
		t.Errorf("expected content to contain 'authentication', got %q", content.String)
	}
	if strings.Contains(content.String, "| Field | Description | Type |") {
		t.Error("expected field table to be stripped from content")
	}
	if strings.Contains(content.String, "nginx.access.remote_ip_list") {
		t.Error("expected field table rows to be stripped from content")
	}
	if strings.Contains(content.String, "\"event.timeout\": \"30s\"") {
		t.Error("expected example event JSON to be stripped from content")
	}
	// The prose "Troubleshooting" section should be preserved.
	if !strings.Contains(content.String, "Check the timeout settings") {
		t.Error("expected prose after stripped sections to be preserved")
	}

	// Verify FTS5 does NOT match a field name that only appeared in the table.
	var ftsFieldCount int
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM docs_fts WHERE docs_fts MATCH 'nginx'").
		Scan(&ftsFieldCount)
	if err != nil {
		t.Fatalf("FTS5 field search: %v", err)
	}
	if ftsFieldCount != 0 {
		t.Error("expected FTS not to match field name 'nginx' from stripped table")
	}

	// Verify FTS5 search finds the doc by keyword.
	var ftsFilePath string
	err = db.QueryRowContext(ctx,
		"SELECT d.file_path FROM docs_fts JOIN docs d ON d.id = docs_fts.rowid WHERE docs_fts MATCH 'certificate'").
		Scan(&ftsFilePath)
	if err != nil {
		t.Fatalf("FTS5 search: %v", err)
	}
	if ftsFilePath != "docs/knowledge_base/troubleshooting.md" {
		t.Errorf("expected troubleshooting doc, got %s", ftsFilePath)
	}

	// Verify FTS5 join back to packages.
	var pkgName string
	err = db.QueryRowContext(ctx, `
		SELECT p.name
		FROM docs_fts
		JOIN docs d ON d.id = docs_fts.rowid
		JOIN packages p ON p.id = d.packages_id
		WHERE docs_fts MATCH 'authentication'
		LIMIT 1`).Scan(&pkgName)
	if err != nil {
		t.Fatalf("FTS5 package join: %v", err)
	}
	if pkgName != "doc-test" {
		t.Errorf("expected doc-test, got %s", pkgName)
	}
}

func TestChangelogEntriesFTS(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: fts-changelog-test
title: FTS Changelog Test
version: 1.2.0
description: A package with changelog entries.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.2.0
  changes:
    - description: Fixed SSL handshake timeout when proxy is configured.
      type: bugfix
      link: https://github.com/test/3
    - description: Added dashboard for monitoring network traffic.
      type: enhancement
      link: https://github.com/test/4
- version: 1.1.0
  changes:
    - description: Improved certificate validation error messages.
      type: enhancement
      link: https://github.com/test/2
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
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

	// Verify FTS search finds changelog entries by keyword.
	var desc, entryType string
	err = db.QueryRowContext(ctx, `
		SELECT ce.description, ce.type
		FROM changelog_entries_fts
		JOIN changelog_entries ce ON ce.id = changelog_entries_fts.rowid
		WHERE changelog_entries_fts MATCH 'SSL timeout'
		ORDER BY rank
		LIMIT 1`).Scan(&desc, &entryType)
	if err != nil {
		t.Fatalf("FTS changelog search: %v", err)
	}
	if !strings.Contains(desc, "SSL handshake timeout") {
		t.Errorf("expected SSL handshake timeout entry, got %q", desc)
	}
	if entryType != "bugfix" {
		t.Errorf("expected type=bugfix, got %s", entryType)
	}

	// Verify join back to packages through changelogs.
	var pkgName, version string
	err = db.QueryRowContext(ctx, `
		SELECT p.name, c.version
		FROM changelog_entries_fts
		JOIN changelog_entries ce ON ce.id = changelog_entries_fts.rowid
		JOIN changelogs c ON c.id = ce.changelogs_id
		JOIN packages p ON p.id = c.packages_id
		WHERE changelog_entries_fts MATCH 'certificate'
		LIMIT 1`).Scan(&pkgName, &version)
	if err != nil {
		t.Fatalf("FTS changelog package join: %v", err)
	}
	if pkgName != "fts-changelog-test" {
		t.Errorf("expected fts-changelog-test, got %s", pkgName)
	}
	if version != "1.1.0" {
		t.Errorf("expected version 1.1.0, got %s", version)
	}

	// Verify search for "dashboard" finds the enhancement entry.
	var dashDesc string
	err = db.QueryRowContext(ctx, `
		SELECT ce.description
		FROM changelog_entries_fts
		JOIN changelog_entries ce ON ce.id = changelog_entries_fts.rowid
		WHERE changelog_entries_fts MATCH 'dashboard'
		LIMIT 1`).Scan(&dashDesc)
	if err != nil {
		t.Fatalf("FTS changelog dashboard search: %v", err)
	}
	if !strings.Contains(dashDesc, "dashboard") {
		t.Errorf("expected dashboard entry, got %q", dashDesc)
	}
}

func TestWritePackageWithKibanaObjects(t *testing.T) {
	dashboardJSON := `{
  "id": "overview-dash-1",
  "type": "dashboard",
  "attributes": {
    "title": "Overview Dashboard",
    "description": "Main overview of all events."
  },
  "references": [
    {
      "id": "vis-1",
      "name": "panel_0",
      "type": "visualization"
    }
  ],
  "coreMigrationVersion": "8.8.0",
  "typeMigrationVersion": "8.9.0",
  "managed": true
}`
	visualizationJSON := `{
  "id": "vis-1",
  "type": "visualization",
  "attributes": {
    "title": "Event Count"
  },
  "references": []
}`

	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: kibana-test
title: Kibana Test
version: 1.0.0
description: A package with Kibana objects.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"kibana/dashboard/overview.json":  {Data: []byte(dashboardJSON)},
		"kibana/visualization/vis-1.json": {Data: []byte(visualizationJSON)},
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

	// Verify kibana_saved_objects has 2 rows.
	var objCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM kibana_saved_objects").Scan(&objCount)
	if err != nil {
		t.Fatalf("querying kibana_saved_objects: %v", err)
	}
	if objCount != 2 {
		t.Errorf("expected 2 kibana saved objects, got %d", objCount)
	}

	// Verify dashboard row.
	var assetType, objectID, title string
	var refCount int
	err = db.QueryRowContext(ctx,
		"SELECT asset_type, object_id, title, reference_count FROM kibana_saved_objects WHERE object_id = 'overview-dash-1'").
		Scan(&assetType, &objectID, &title, &refCount)
	if err != nil {
		t.Fatalf("querying dashboard: %v", err)
	}
	if assetType != "dashboard" {
		t.Errorf("expected asset_type=dashboard, got %s", assetType)
	}
	if title != "Overview Dashboard" {
		t.Errorf("expected title=Overview Dashboard, got %s", title)
	}
	if refCount != 1 {
		t.Errorf("expected reference_count=1, got %d", refCount)
	}

	// Verify migration versions and managed flag on dashboard.
	var coreMigVer, typeMigVer sql.NullString
	var managed sql.NullBool
	err = db.QueryRowContext(ctx,
		"SELECT core_migration_version, type_migration_version, managed FROM kibana_saved_objects WHERE object_id = 'overview-dash-1'").
		Scan(&coreMigVer, &typeMigVer, &managed)
	if err != nil {
		t.Fatalf("querying migration versions: %v", err)
	}
	if !coreMigVer.Valid || coreMigVer.String != "8.8.0" {
		t.Errorf("expected core_migration_version=8.8.0, got %v", coreMigVer)
	}
	if !typeMigVer.Valid || typeMigVer.String != "8.9.0" {
		t.Errorf("expected type_migration_version=8.9.0, got %v", typeMigVer)
	}
	if !managed.Valid || !managed.Bool {
		t.Errorf("expected managed=true, got %v", managed)
	}

	// Verify kibana_references has 1 row.
	var refID, refName, refType string
	err = db.QueryRowContext(ctx,
		"SELECT ref_id, ref_name, ref_type FROM kibana_references").
		Scan(&refID, &refName, &refType)
	if err != nil {
		t.Fatalf("querying kibana_references: %v", err)
	}
	if refID != "vis-1" {
		t.Errorf("expected ref_id=vis-1, got %s", refID)
	}
	if refName != "panel_0" {
		t.Errorf("expected ref_name=panel_0, got %s", refName)
	}
	if refType != "visualization" {
		t.Errorf("expected ref_type=visualization, got %s", refType)
	}

	// Verify join to packages works.
	var pkgName string
	err = db.QueryRowContext(ctx, `
		SELECT p.name
		FROM kibana_saved_objects kso
		JOIN packages p ON p.id = kso.packages_id
		WHERE kso.object_id = 'overview-dash-1'`).Scan(&pkgName)
	if err != nil {
		t.Fatalf("querying package join: %v", err)
	}
	if pkgName != "kibana-test" {
		t.Errorf("expected kibana-test, got %s", pkgName)
	}

	// Verify visualization has no references.
	var visRefCount int
	err = db.QueryRowContext(ctx,
		"SELECT reference_count FROM kibana_saved_objects WHERE object_id = 'vis-1'").
		Scan(&visRefCount)
	if err != nil {
		t.Fatalf("querying visualization: %v", err)
	}
	if visRefCount != 0 {
		t.Errorf("expected reference_count=0 for visualization, got %d", visRefCount)
	}
}

func TestSystemTestVarsNullable(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-package
title: Test
version: 1.0.0
description: test
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"data_stream/logs/manifest.yml": {Data: []byte(`
title: Logs
type: logs
streams:
  - input: logfile
    title: Logs
    description: Collect logs.
`)},
		"data_stream/logs/fields/base-fields.yml": {Data: []byte(`
- name: "@timestamp"
  type: date
`)},
		// System test with no vars and no data_stream.
		"data_stream/logs/_dev/test/system/test-empty-config.yml": {Data: []byte(`{}
`)},
		// System test with extra unknown fields but no vars (the common case).
		// Decoded without knownFields so unknown keys are silently ignored.
		"data_stream/logs/_dev/test/system/test-typical-config.yml": {Data: []byte(`
service: some-service
input: http_endpoint
data_stream:
  vars:
    listen_address: 0.0.0.0
    listen_port: 8384
`)},
		// System test with vars set.
		"data_stream/logs/_dev/test/system/test-withvars-config.yml": {Data: []byte(`
vars:
  data_stream.dataset: custom_dataset
data_stream:
  vars:
    data_stream.dataset: ds_override
`)},
	}

	// Do not use WithKnownFields because real system test configs contain
	// extra fields (service, input, assert) that are not in SystemTestConfig.
	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys), pkgreader.WithTestConfigs())
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	if err := pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg}); err != nil {
		t.Fatalf("writing package: %v", err)
	}

	// The empty config should have NULL for vars and data_stream.
	var vars, dataStream sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT vars, data_stream FROM system_tests WHERE case_name = 'empty'").
		Scan(&vars, &dataStream)
	if err != nil {
		t.Fatalf("querying empty system test: %v", err)
	}
	if vars.Valid {
		t.Errorf("expected NULL vars for empty config, got %q", vars.String)
	}
	if dataStream.Valid {
		t.Errorf("expected NULL data_stream for empty config, got %q", dataStream.String)
	}

	// The typical config has extra unknown fields but no data_stream.dataset
	// in vars. The vars column should be NULL (zero-value TestVars), while
	// data_stream should be non-NULL (pointer was set by YAML).
	err = db.QueryRowContext(ctx,
		"SELECT vars, data_stream FROM system_tests WHERE case_name = 'typical'").
		Scan(&vars, &dataStream)
	if err != nil {
		t.Fatalf("querying typical system test: %v", err)
	}
	if vars.Valid {
		t.Errorf("expected NULL vars for typical config (no data_stream.dataset), got %q", vars.String)
	}
	if !dataStream.Valid {
		t.Error("expected non-NULL data_stream for typical config (key was present in YAML)")
	}

	// The withvars config should have non-NULL values.
	err = db.QueryRowContext(ctx,
		"SELECT vars, data_stream FROM system_tests WHERE case_name = 'withvars'").
		Scan(&vars, &dataStream)
	if err != nil {
		t.Fatalf("querying withvars system test: %v", err)
	}
	if !vars.Valid {
		t.Error("expected non-NULL vars for withvars config")
	} else if !strings.Contains(vars.String, "custom_dataset") {
		t.Errorf("expected vars to contain custom_dataset, got %q", vars.String)
	}
	if !dataStream.Valid {
		t.Error("expected non-NULL data_stream for withvars config")
	} else if !strings.Contains(dataStream.String, "ds_override") {
		t.Errorf("expected data_stream to contain ds_override, got %q", dataStream.String)
	}
}

func TestBuildFleetPackagesDB(t *testing.T) {
	dir := os.Getenv("INTEGRATIONS_DIR")
	if dir == "" {
		t.Skip("INTEGRATIONS_DIR not set")
	}

	packagesDir := filepath.Join(dir, "packages")
	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		t.Fatalf("reading packages directory: %v", err)
	}

	dbPath := filepath.Join(".", "fleet-packages.sqlite")
	os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Enable WAL mode and other SQLite optimizations for bulk inserts.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-64000",
		"PRAGMA mmap_size=268435456",
		"PRAGMA temp_store=MEMORY",
	} {
		if _, err := db.Exec(pragma); err != nil {
			t.Fatalf("setting %s: %v", pragma, err)
		}
	}

	ctx := context.Background()

	// Create tables.
	for _, ddl := range pkgsql.TableSchemas() {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("creating tables: %v", err)
		}
	}

	codeownersPath := filepath.Join(dir, ".github", "CODEOWNERS")
	opts := []pkgreader.Option{
		pkgreader.WithKnownFields(),
		pkgreader.WithGitMetadata(),
		pkgreader.WithImageMetadata(),
		pkgreader.WithTestConfigs(),
		pkgreader.WithAgentTemplates(),
		pkgreader.WithCodeowners(codeownersPath),
	}

	// Read packages in parallel, write to DB sequentially.
	type result struct {
		pkg  *pkgreader.Package
		name string
		err  error
	}

	// Use more workers than CPUs since package reading is I/O bound
	// (git blame subprocess, file reads).
	workers := 4 * runtime.NumCPU()
	work := make(chan string, workers)
	results := make(chan result, workers)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range work {
				pkgPath := filepath.Join(packagesDir, name)
				pkgOpts := append(opts, pkgreader.WithPathPrefix(path.Join("packages", name)))
				pkg, err := pkgreader.Read(pkgPath, pkgOpts...)
				results <- result{pkg: pkg, name: name, err: err}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			work <- entry.Name()
		}
		close(work)
	}()

	var loaded int
	for r := range results {
		if r.err != nil {
			t.Fatalf("reading package %s: %v", r.name, r.err)
		}

		if err := pkgsql.WritePackage(ctx, db, r.pkg, pkgsql.WithDocContent(pkgsql.OSDocReader)); err != nil {
			t.Fatalf("writing package %s: %v", r.name, r.err)
		}
		loaded++
	}

	// Rebuild FTS indexes after all individual writes.
	if err := pkgsql.RebuildFTS(ctx, db); err != nil {
		t.Fatalf("rebuilding FTS indexes: %v", err)
	}

	t.Logf("loaded %d packages into %s", loaded, dbPath)

	// Verify commit_id is populated (WithGitMetadata was used).
	var commitID sql.NullString
	err = db.QueryRowContext(ctx, "SELECT commit_id FROM packages LIMIT 1").Scan(&commitID)
	if err != nil {
		t.Fatalf("querying commit_id: %v", err)
	}
	if !commitID.Valid {
		t.Fatal("expected non-NULL commit_id with WithGitMetadata")
	}
	if len(commitID.String) != 40 {
		t.Errorf("expected 40-char hex SHA commit_id, got %q (len=%d)", commitID.String, len(commitID.String))
	}

	// Verify github_code_owner is populated for a data stream with CODEOWNERS entry.
	var githubCodeOwner sql.NullString
	err = db.QueryRowContext(ctx, `
		SELECT ds.github_code_owner
		FROM data_streams ds
		JOIN packages p ON p.id = ds.packages_id
		WHERE p.name = 'aws' AND ds.dir_name = 'cloudtrail'`).Scan(&githubCodeOwner)
	if err != nil {
		t.Fatalf("querying github_code_owner: %v", err)
	}
	if !githubCodeOwner.Valid || githubCodeOwner.String == "" {
		t.Error("expected non-NULL github_code_owner for aws/cloudtrail with WithCodeowners")
	}
}

func TestWritePackageWithAgentTemplates(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-agent-tpl
title: Test Agent Templates
version: 1.0.0
description: Test agent template persistence.
format_version: 3.5.7
type: integration
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: test-policy
    title: Test Policy
    description: A test policy.
    inputs:
      - type: logfile
        title: Log File
        description: Collect log files.
        template_path: custom-input.yml.hbs
      - type: httpjson
        title: HTTP JSON
        description: Collect via HTTP JSON.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"data_stream/logs/manifest.yml": {Data: []byte(`
title: Log Events
type: logs
streams:
  - input: logfile
    title: Log stream with custom template
    description: Collect logs via custom template.
    template_path: custom.yml.hbs
  - input: httpjson
    title: HTTP JSON stream with default template
    description: Collect via HTTP JSON with default template.
`)},
		"data_stream/logs/fields/base-fields.yml": {Data: []byte(`
- name: message
  type: text
  description: Log message.
`)},
		"data_stream/logs/agent/stream/custom.yml.hbs": {Data: []byte("custom ds template content\n")},
		"data_stream/logs/agent/stream/stream.yml.hbs": {Data: []byte("default ds template content\n")},
		"agent/input/custom-input.yml.hbs":             {Data: []byte("custom input stream template\n")},
	}

	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys), pkgreader.WithAgentTemplates())
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	err = pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg})
	if err != nil {
		t.Fatalf("writing packages: %v", err)
	}

	// Verify agent_templates has 3 rows (2 data stream + 1 package-level).
	var atCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM agent_templates").Scan(&atCount)
	if err != nil {
		t.Fatalf("querying agent_templates count: %v", err)
	}
	if atCount != 3 {
		t.Errorf("expected 3 agent templates, got %d", atCount)
	}

	// Verify data stream templates have non-NULL data_streams_id.
	var dsTemplateCount int
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM agent_templates WHERE data_streams_id IS NOT NULL").
		Scan(&dsTemplateCount)
	if err != nil {
		t.Fatalf("querying ds templates: %v", err)
	}
	if dsTemplateCount != 2 {
		t.Errorf("expected 2 data stream templates, got %d", dsTemplateCount)
	}

	// Verify package-level template has NULL data_streams_id.
	var pkgTemplateCount int
	err = db.QueryRowContext(ctx,
		"SELECT count(*) FROM agent_templates WHERE data_streams_id IS NULL").
		Scan(&pkgTemplateCount)
	if err != nil {
		t.Fatalf("querying pkg templates: %v", err)
	}
	if pkgTemplateCount != 1 {
		t.Errorf("expected 1 package-level template, got %d", pkgTemplateCount)
	}

	// Verify template content is stored correctly.
	var content string
	err = db.QueryRowContext(ctx,
		"SELECT content FROM agent_templates WHERE file_path = 'data_stream/logs/agent/stream/custom.yml.hbs'").
		Scan(&content)
	if err != nil {
		t.Fatalf("querying template content: %v", err)
	}
	if content != "custom ds template content\n" {
		t.Errorf("unexpected content: %q", content)
	}

	// Verify streams.template_path is resolved to full path (custom template).
	var streamTP sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template_path FROM streams WHERE input = 'logfile'").
		Scan(&streamTP)
	if err != nil {
		t.Fatalf("querying stream template_path: %v", err)
	}
	if !streamTP.Valid || streamTP.String != "data_stream/logs/agent/stream/custom.yml.hbs" {
		t.Errorf("expected resolved template_path=data_stream/logs/agent/stream/custom.yml.hbs, got %v", streamTP)
	}

	// Verify stream default: second stream has no template_path in manifest,
	// should default to stream.yml.hbs resolved path.
	var defaultTP sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template_path FROM streams WHERE input = 'httpjson'").
		Scan(&defaultTP)
	if err != nil {
		t.Fatalf("querying default stream template_path: %v", err)
	}
	if !defaultTP.Valid || defaultTP.String != "data_stream/logs/agent/stream/stream.yml.hbs" {
		t.Errorf("expected resolved template_path=data_stream/logs/agent/stream/stream.yml.hbs, got %v", defaultTP)
	}

	// Verify join from streams to agent_templates works.
	var joinContent string
	err = db.QueryRowContext(ctx, `
		SELECT at.content FROM streams s
		JOIN agent_templates at ON at.file_path = s.template_path
		WHERE s.input = 'logfile'`).
		Scan(&joinContent)
	if err != nil {
		t.Fatalf("querying streams->agent_templates join: %v", err)
	}
	if joinContent != "custom ds template content\n" {
		t.Errorf("unexpected join content: %q", joinContent)
	}

	// Verify policy_template_inputs.template_path is resolved.
	var inputTP sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template_path FROM policy_template_inputs WHERE type = 'logfile'").
		Scan(&inputTP)
	if err != nil {
		t.Fatalf("querying input template_path: %v", err)
	}
	if !inputTP.Valid || inputTP.String != "agent/input/custom-input.yml.hbs" {
		t.Errorf("expected resolved template_path=agent/input/custom-input.yml.hbs, got %v", inputTP)
	}

	// Verify policy_template_inputs with no template_path is NULL.
	var noTP sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template_path FROM policy_template_inputs WHERE type = 'httpjson'").
		Scan(&noTP)
	if err != nil {
		t.Fatalf("querying no-template input: %v", err)
	}
	if noTP.Valid {
		t.Errorf("expected NULL template_path for httpjson input, got %v", noTP)
	}

	// Verify join from policy_template_inputs to agent_templates works.
	var inputJoinContent string
	err = db.QueryRowContext(ctx, `
		SELECT at.content FROM policy_template_inputs pti
		JOIN agent_templates at ON at.file_path = pti.template_path
		WHERE pti.type = 'logfile'`).
		Scan(&inputJoinContent)
	if err != nil {
		t.Fatalf("querying policy_template_inputs->agent_templates join: %v", err)
	}
	if inputJoinContent != "custom input stream template\n" {
		t.Errorf("unexpected join content: %q", inputJoinContent)
	}
}

func TestWriteInputPackageAgentTemplates(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: test-input-tpl
title: Test Input Templates
version: 1.0.0
description: Test input package agent templates.
format_version: 3.5.7
type: input
owner:
  github: elastic/integrations
  type: elastic
policy_templates:
  - name: test-input-pt
    type: logs
    title: Test Input Policy
    description: Collect data.
    input: httpjson
    template_path: input.yml.hbs
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"agent/input/input.yml.hbs": {Data: []byte("input template content\n")},
	}

	pkg, err := pkgreader.Read(".", pkgreader.WithFS(fsys), pkgreader.WithAgentTemplates())
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	db := newTestDB(t)
	ctx := context.Background()

	err = pkgsql.WritePackages(ctx, db, []*pkgreader.Package{pkg})
	if err != nil {
		t.Fatalf("writing packages: %v", err)
	}

	// Verify agent_templates has 1 row.
	var atCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM agent_templates").Scan(&atCount)
	if err != nil {
		t.Fatalf("querying agent_templates count: %v", err)
	}
	if atCount != 1 {
		t.Errorf("expected 1 agent template, got %d", atCount)
	}

	// Verify template is package-level (no data stream).
	var dsID sql.NullInt64
	err = db.QueryRowContext(ctx,
		"SELECT data_streams_id FROM agent_templates").Scan(&dsID)
	if err != nil {
		t.Fatalf("querying ds id: %v", err)
	}
	if dsID.Valid {
		t.Errorf("expected NULL data_streams_id, got %v", dsID)
	}

	// Verify policy_templates.template_path is resolved.
	var ptTP sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT template_path FROM policy_templates").Scan(&ptTP)
	if err != nil {
		t.Fatalf("querying policy template path: %v", err)
	}
	if !ptTP.Valid || ptTP.String != "agent/input/input.yml.hbs" {
		t.Errorf("expected template_path=agent/input/input.yml.hbs, got %v", ptTP)
	}

	// Verify join from policy_templates to agent_templates.
	var joinContent string
	err = db.QueryRowContext(ctx, `
		SELECT at.content FROM policy_templates pt
		JOIN agent_templates at ON at.file_path = pt.template_path
		WHERE pt.name = 'test-input-pt'`).
		Scan(&joinContent)
	if err != nil {
		t.Fatalf("querying policy_templates->agent_templates join: %v", err)
	}
	if joinContent != "input template content\n" {
		t.Errorf("unexpected join content: %q", joinContent)
	}
}

func TestWritePackageWithSecurityRules(t *testing.T) {
	ruleJSON := `{
  "id": "test-rule-id-1",
  "type": "security-rule",
  "attributes": {
    "name": "Okta Suspicious Login Attempt",
    "description": "Detects suspicious login attempts via Okta SSO.",
    "rule_id": "okta-suspicious-login-001",
    "type": "eql",
    "severity": "high",
    "risk_score": 73,
    "language": "eql",
    "query": "authentication where event.dataset == \"okta.system\" and event.action == \"user.session.start\" and event.outcome == \"failure\"",
    "enabled": true,
    "version": 5,
    "license": "Elastic License v2",
    "interval": "5m",
    "from": "now-9m",
    "max_signals": 100,
    "timestamp_override": "event.ingested",
    "setup": "## Setup\nRequires Okta integration.",
    "note": "## Triage\nCheck the source IP address.",
    "author": ["Elastic"],
    "false_positives": ["Legitimate failed logins"],
    "references": ["https://developer.okta.com/docs/reference/api/system-log/"],
    "index": ["logs-okta.system-*", "filebeat-*"],
    "tags": ["Domain: Cloud", "Data Source: Okta", "Tactic: Initial Access"],
    "threat": [
      {
        "framework": "MITRE ATT&CK",
        "tactic": {
          "id": "TA0001",
          "name": "Initial Access",
          "reference": "https://attack.mitre.org/tactics/TA0001/"
        },
        "technique": [
          {
            "id": "T1078",
            "name": "Valid Accounts",
            "reference": "https://attack.mitre.org/techniques/T1078/",
            "subtechnique": [
              {
                "id": "T1078.004",
                "name": "Cloud Accounts",
                "reference": "https://attack.mitre.org/techniques/T1078/004/"
              }
            ]
          }
        ]
      },
      {
        "framework": "MITRE ATT&CK",
        "tactic": {
          "id": "TA0005",
          "name": "Defense Evasion",
          "reference": "https://attack.mitre.org/tactics/TA0005/"
        },
        "technique": []
      }
    ],
    "related_integrations": [
      {"package": "okta", "integration": "system", "version": "^2.0.0"}
    ],
    "required_fields": [
      {"name": "event.action", "type": "keyword", "ecs": true},
      {"name": "event.dataset", "type": "keyword", "ecs": true},
      {"name": "event.outcome", "type": "keyword", "ecs": true}
    ],
    "risk_score_mapping": [],
    "severity_mapping": []
  },
  "references": []
}`

	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: security-rule-test
title: Security Rule Test
version: 1.0.0
description: A package with security rules.
format_version: 3.5.7
type: integration
owner:
  github: elastic/security-rules
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"kibana/security_rule/rule.json": {Data: []byte(ruleJSON)},
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

	// Verify security_rules has 1 row with correct fields.
	var ruleID, ruleType, severity, language, query string
	var riskScore float64
	err = db.QueryRowContext(ctx,
		"SELECT rule_id, type, severity, language, query, risk_score FROM security_rules").
		Scan(&ruleID, &ruleType, &severity, &language, &query, &riskScore)
	if err != nil {
		t.Fatalf("querying security_rules: %v", err)
	}
	if ruleID != "okta-suspicious-login-001" {
		t.Errorf("expected rule_id=okta-suspicious-login-001, got %s", ruleID)
	}
	if ruleType != "eql" {
		t.Errorf("expected type=eql, got %s", ruleType)
	}
	if severity != "high" {
		t.Errorf("expected severity=high, got %s", severity)
	}
	if riskScore != 73 {
		t.Errorf("expected risk_score=73, got %f", riskScore)
	}

	// Verify enabled, version, interval, from_time, max_signals.
	var enabled bool
	var version, maxSignals int
	var interval, fromTime string
	err = db.QueryRowContext(ctx,
		"SELECT enabled, version, interval, from_time, max_signals FROM security_rules").
		Scan(&enabled, &version, &interval, &fromTime, &maxSignals)
	if err != nil {
		t.Fatalf("querying security_rules scalars: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true")
	}
	if version != 5 {
		t.Errorf("expected version=5, got %d", version)
	}
	if interval != "5m" {
		t.Errorf("expected interval=5m, got %s", interval)
	}
	if fromTime != "now-9m" {
		t.Errorf("expected from_time=now-9m, got %s", fromTime)
	}
	if maxSignals != 100 {
		t.Errorf("expected max_signals=100, got %d", maxSignals)
	}

	// Verify setup and note.
	var setup, note string
	err = db.QueryRowContext(ctx, "SELECT setup, note FROM security_rules").
		Scan(&setup, &note)
	if err != nil {
		t.Fatalf("querying setup/note: %v", err)
	}
	if !strings.Contains(setup, "Requires Okta") {
		t.Errorf("expected setup to contain 'Requires Okta', got %s", setup)
	}
	if !strings.Contains(note, "source IP") {
		t.Errorf("expected note to contain 'source IP', got %s", note)
	}

	// Verify security_rule_index_patterns has 2 rows.
	var patternCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM security_rule_index_patterns").Scan(&patternCount)
	if err != nil {
		t.Fatalf("querying index patterns: %v", err)
	}
	if patternCount != 2 {
		t.Errorf("expected 2 index patterns, got %d", patternCount)
	}

	// Verify security_rule_tags has 3 rows.
	var tagCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM security_rule_tags").Scan(&tagCount)
	if err != nil {
		t.Fatalf("querying tags: %v", err)
	}
	if tagCount != 3 {
		t.Errorf("expected 3 tags, got %d", tagCount)
	}

	// Verify security_rule_threats: 2 rows (1 technique + 1 tactic-only).
	var threatCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM security_rule_threats").Scan(&threatCount)
	if err != nil {
		t.Fatalf("querying threats: %v", err)
	}
	if threatCount != 2 {
		t.Errorf("expected 2 threat rows, got %d", threatCount)
	}

	// Verify the technique row has correct values.
	var tacticID, tacticName string
	var techID, techName sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT tactic_id, tactic_name, technique_id, technique_name FROM security_rule_threats WHERE technique_id IS NOT NULL").
		Scan(&tacticID, &tacticName, &techID, &techName)
	if err != nil {
		t.Fatalf("querying technique row: %v", err)
	}
	if tacticID != "TA0001" {
		t.Errorf("expected tactic_id=TA0001, got %s", tacticID)
	}
	if tacticName != "Initial Access" {
		t.Errorf("expected tactic_name=Initial Access, got %s", tacticName)
	}
	if !techID.Valid || techID.String != "T1078" {
		t.Errorf("expected technique_id=T1078, got %v", techID)
	}

	// Verify the tactic-only row (Defense Evasion with empty technique list).
	var tactOnlyID string
	var tactOnlyTechID sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT tactic_id, technique_id FROM security_rule_threats WHERE tactic_id = 'TA0005'").
		Scan(&tactOnlyID, &tactOnlyTechID)
	if err != nil {
		t.Fatalf("querying tactic-only row: %v", err)
	}
	if tactOnlyTechID.Valid {
		t.Errorf("expected NULL technique_id for tactic-only row, got %s", tactOnlyTechID.String)
	}

	// Verify subtechniques JSON on the T1078 row.
	var subtechniques sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT subtechniques FROM security_rule_threats WHERE technique_id = 'T1078'").
		Scan(&subtechniques)
	if err != nil {
		t.Fatalf("querying subtechniques: %v", err)
	}
	if !subtechniques.Valid {
		t.Fatal("expected non-NULL subtechniques")
	}
	if !strings.Contains(subtechniques.String, "T1078.004") {
		t.Errorf("expected subtechniques to contain T1078.004, got %s", subtechniques.String)
	}

	// Verify security_rule_related_integrations has 1 row.
	var riPkg, riVersion string
	var riIntegration sql.NullString
	err = db.QueryRowContext(ctx,
		"SELECT package, integration, version FROM security_rule_related_integrations").
		Scan(&riPkg, &riIntegration, &riVersion)
	if err != nil {
		t.Fatalf("querying related integrations: %v", err)
	}
	if riPkg != "okta" {
		t.Errorf("expected package=okta, got %s", riPkg)
	}
	if !riIntegration.Valid || riIntegration.String != "system" {
		t.Errorf("expected integration=system, got %v", riIntegration)
	}
	if riVersion != "^2.0.0" {
		t.Errorf("expected version=^2.0.0, got %s", riVersion)
	}

	// Verify security_rule_required_fields has 3 rows.
	var rfCount int
	err = db.QueryRowContext(ctx, "SELECT count(*) FROM security_rule_required_fields").Scan(&rfCount)
	if err != nil {
		t.Fatalf("querying required fields: %v", err)
	}
	if rfCount != 3 {
		t.Errorf("expected 3 required fields, got %d", rfCount)
	}

	// Verify a specific required field.
	var rfName, rfType string
	var rfECS bool
	err = db.QueryRowContext(ctx,
		"SELECT name, type, ecs FROM security_rule_required_fields WHERE name = 'event.action'").
		Scan(&rfName, &rfType, &rfECS)
	if err != nil {
		t.Fatalf("querying required field event.action: %v", err)
	}
	if rfType != "keyword" {
		t.Errorf("expected type=keyword, got %s", rfType)
	}
	if !rfECS {
		t.Error("expected ecs=true for event.action")
	}

	// Verify join from security_rules to kibana_saved_objects.
	var ksoTitle string
	err = db.QueryRowContext(ctx, `
		SELECT kso.title
		FROM security_rules sr
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id`).
		Scan(&ksoTitle)
	if err != nil {
		t.Fatalf("querying security_rules->kibana_saved_objects join: %v", err)
	}
	if ksoTitle != "Okta Suspicious Login Attempt" {
		t.Errorf("expected title=Okta Suspicious Login Attempt, got %s", ksoTitle)
	}
}

func TestSecurityRulesFTS(t *testing.T) {
	ruleJSON := `{
  "id": "fts-test-rule-1",
  "type": "security-rule",
  "attributes": {
    "title": "Log4Shell Remote Code Execution",
    "description": "Detects exploitation of the Log4Shell vulnerability CVE-2021-44228.",
    "rule_id": "log4shell-rce-001",
    "type": "query",
    "severity": "critical",
    "risk_score": 99,
    "language": "kuery",
    "query": "process.command_line : *jndi:ldap* or process.command_line : *jndi:rmi*",
    "enabled": true,
    "version": 1,
    "setup": "## Setup\nDeploy Elastic Defend to collect process events.",
    "note": "## Investigation Guide\nCheck for JNDI lookup patterns in process arguments."
  },
  "references": []
}`

	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte(`
name: fts-security-test
title: FTS Security Test
version: 1.0.0
description: Package for FTS test.
format_version: 3.5.7
type: integration
owner:
  github: elastic/security-rules
  type: elastic
policy_templates:
  - name: default
    title: Default
    description: Default policy.
    inputs:
      - type: logfile
        title: Log
        description: Collect logs.
`)},
		"changelog.yml": {Data: []byte(`
- version: 1.0.0
  changes:
    - description: Initial release
      type: enhancement
      link: https://github.com/test/1
`)},
		"kibana/security_rule/rule.json": {Data: []byte(ruleJSON)},
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

	// Search for Log4Shell in title.
	var ftsTitle string
	err = db.QueryRowContext(ctx, `
		SELECT kso.title
		FROM security_rules_fts
		JOIN security_rules sr ON sr.id = security_rules_fts.rowid
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id
		WHERE security_rules_fts MATCH 'Log4Shell'`).
		Scan(&ftsTitle)
	if err != nil {
		t.Fatalf("FTS search for Log4Shell: %v", err)
	}
	if ftsTitle != "Log4Shell Remote Code Execution" {
		t.Errorf("expected Log4Shell title, got %s", ftsTitle)
	}

	// Search for term in query column.
	err = db.QueryRowContext(ctx, `
		SELECT kso.title
		FROM security_rules_fts
		JOIN security_rules sr ON sr.id = security_rules_fts.rowid
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id
		WHERE security_rules_fts MATCH 'jndi'`).
		Scan(&ftsTitle)
	if err != nil {
		t.Fatalf("FTS search for jndi: %v", err)
	}
	if ftsTitle != "Log4Shell Remote Code Execution" {
		t.Errorf("expected Log4Shell title from query match, got %s", ftsTitle)
	}

	// Search for term in setup column.
	err = db.QueryRowContext(ctx, `
		SELECT kso.title
		FROM security_rules_fts
		JOIN security_rules sr ON sr.id = security_rules_fts.rowid
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id
		WHERE security_rules_fts MATCH 'setup:Defend'`).
		Scan(&ftsTitle)
	if err != nil {
		t.Fatalf("FTS search for Defend in setup: %v", err)
	}
	if ftsTitle != "Log4Shell Remote Code Execution" {
		t.Errorf("expected Log4Shell title from setup match, got %s", ftsTitle)
	}

	// Search for term in note column.
	err = db.QueryRowContext(ctx, `
		SELECT kso.title
		FROM security_rules_fts
		JOIN security_rules sr ON sr.id = security_rules_fts.rowid
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id
		WHERE security_rules_fts MATCH 'note:JNDI'`).
		Scan(&ftsTitle)
	if err != nil {
		t.Fatalf("FTS search for JNDI in note: %v", err)
	}
	if ftsTitle != "Log4Shell Remote Code Execution" {
		t.Errorf("expected Log4Shell title from note match, got %s", ftsTitle)
	}

	// Verify join from FTS to packages.
	var pkgName string
	err = db.QueryRowContext(ctx, `
		SELECT p.name
		FROM security_rules_fts
		JOIN security_rules sr ON sr.id = security_rules_fts.rowid
		JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id
		JOIN packages p ON p.id = kso.packages_id
		WHERE security_rules_fts MATCH 'Log4Shell'`).
		Scan(&pkgName)
	if err != nil {
		t.Fatalf("FTS to packages join: %v", err)
	}
	if pkgName != "fts-security-test" {
		t.Errorf("expected fts-security-test, got %s", pkgName)
	}
}
