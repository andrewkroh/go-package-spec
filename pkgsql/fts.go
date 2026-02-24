package pkgsql

import (
	"context"
	"database/sql"
)

// docsFTS is the FTS5 virtual table for full-text search over doc content.
// Uses external content mode: reads content from the docs table, stores
// only the inverted index. After bulk inserts, rebuild with:
//
//	INSERT INTO docs_fts(docs_fts) VALUES('rebuild')
const docsFTS = `CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
  content,
  content=docs,
  content_rowid=id,
  tokenize='porter unicode61'
)`

// changelogEntriesFTS is the FTS5 virtual table for full-text search over
// changelog entry descriptions. Uses external content mode against the
// changelog_entries table. This enables searching changelog prose like
// "Fixed SSL handshake timeout when proxy is configured".
const changelogEntriesFTS = `CREATE VIRTUAL TABLE IF NOT EXISTS changelog_entries_fts USING fts5(
  description,
  content=changelog_entries,
  content_rowid=id,
  tokenize='porter unicode61'
)`

// securityRulesFTSView is a view joining security_rules with
// kibana_saved_objects to provide the content source for FTS5 indexing.
// Title and description live on kibana_saved_objects while query, setup,
// and note live on security_rules.
const securityRulesFTSView = `CREATE VIEW IF NOT EXISTS security_rules_fts_content AS
SELECT
  sr.id AS id,
  COALESCE(kso.title, '') AS title,
  COALESCE(kso.description, '') AS description,
  COALESCE(sr.query, '') AS query,
  COALESCE(sr.setup, '') AS setup,
  COALESCE(sr.note, '') AS note
FROM security_rules sr
JOIN kibana_saved_objects kso ON kso.id = sr.kibana_saved_objects_id`

// securityRulesFTS is the FTS5 virtual table for full-text search over
// security detection rules. Uses external content mode backed by the
// security_rules_fts_content view. Indexes title, description, query,
// setup guide, and investigation note.
const securityRulesFTS = `CREATE VIRTUAL TABLE IF NOT EXISTS security_rules_fts USING fts5(
  title,
  description,
  query,
  setup,
  note,
  content=security_rules_fts_content,
  content_rowid=id,
  tokenize='porter unicode61'
)`

var ftsSchemas = []string{docsFTS, changelogEntriesFTS, securityRulesFTSView, securityRulesFTS}

// RebuildFTS rebuilds all FTS5 full-text search indexes (docs, changelog
// entries, and security rules). WritePackages calls this automatically after
// all packages are inserted. Callers using WritePackage directly must call
// this after all inserts are complete.
func RebuildFTS(ctx context.Context, db *sql.DB) error {
	for _, stmt := range []string{
		"INSERT INTO docs_fts(docs_fts) VALUES('rebuild')",
		"INSERT INTO changelog_entries_fts(changelog_entries_fts) VALUES('rebuild')",
		"INSERT INTO security_rules_fts(security_rules_fts) VALUES('rebuild')",
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// RebuildDocsFTS rebuilds the FTS5 full-text search index for the docs
// table.
//
// Deprecated: Use [RebuildFTS] instead, which rebuilds all FTS indexes.
func RebuildDocsFTS(ctx context.Context, db *sql.DB) error {
	return RebuildFTS(ctx, db)
}
