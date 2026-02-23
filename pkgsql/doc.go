// Package pkgsql provides a SQL interface for loading Elastic packages into
// a SQLite database. It generates the schema, insert queries, and mapping
// code from pkgspec types, then uses sqlc for type-safe database operations.
//
// The primary entry points are [WritePackages] and [WritePackage], which
// accept packages loaded by pkgreader and insert them into the database.
// The [TableSchemas] function returns the CREATE TABLE statements, which
// include inline comments describing each table and column.
//
// The SQL schema is designed to be self-documenting: all table and column
// descriptions are embedded inside the CREATE TABLE body, so they are
// preserved in sqlite_master and accessible to LLMs and other consumers
// without access to the Go source code.
//
// This package imports only [database/sql] and does not depend on any
// SQLite driver. The consumer must import a driver (e.g. modernc.org/sqlite)
// and pass a *sql.DB.
//
//go:generate go run ../cmd/gensql -tables ../cmd/gensql/tables.yml -output . -package pkgsql
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 generate -f internal/db/sqlc.yaml
package pkgsql
