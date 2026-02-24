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

var ftsSchemas = []string{docsFTS}

// RebuildDocsFTS rebuilds the FTS5 full-text search index for the docs
// table. WritePackages calls this automatically after all packages are
// inserted. Callers using WritePackage directly must call this after all
// inserts are complete.
func RebuildDocsFTS(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "INSERT INTO docs_fts(docs_fts) VALUES('rebuild')")
	return err
}
