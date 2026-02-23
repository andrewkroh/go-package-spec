package pkgsql

import (
	"context"
	"database/sql"
)

// stmtCache wraps a *sql.Tx and caches prepared statements so that
// repeated INSERT calls (e.g. thousands of InsertFields) avoid the
// overhead of re-parsing the SQL on every invocation.
type stmtCache struct {
	tx    *sql.Tx
	cache map[string]*sql.Stmt
}

func newStmtCache(tx *sql.Tx) *stmtCache {
	return &stmtCache{tx: tx, cache: make(map[string]*sql.Stmt)}
}

func (c *stmtCache) stmt(ctx context.Context, query string) (*sql.Stmt, error) {
	if s, ok := c.cache[query]; ok {
		return s, nil
	}
	s, err := c.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	c.cache[query] = s
	return s, nil
}

func (c *stmtCache) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	s, err := c.stmt(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.ExecContext(ctx, args...)
}

func (c *stmtCache) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return c.stmt(ctx, query)
}

func (c *stmtCache) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	s, err := c.stmt(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.QueryContext(ctx, args...)
}

func (c *stmtCache) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	s, err := c.stmt(ctx, query)
	if err != nil {
		// Fall back to uncached path to propagate error through Row.Scan.
		return c.tx.QueryRowContext(ctx, query, args...)
	}
	return s.QueryRowContext(ctx, args...)
}

func (c *stmtCache) close() {
	for _, s := range c.cache {
		s.Close()
	}
}
