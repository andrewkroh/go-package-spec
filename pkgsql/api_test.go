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
