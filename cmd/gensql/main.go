// Command gensql generates SQL schema, queries, and Go mapping code from
// pkgspec types and a declarative table configuration file.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/andrewkroh/go-package-spec/internal/sqlgen"
)

func main() {
	cfg := sqlgen.Config{}

	flag.StringVar(&cfg.TablesFile, "tables", "", "Path to tables.yml configuration file (required)")
	flag.StringVar(&cfg.OutputDir, "output", "pkgsql", "Output directory for generated files")
	flag.StringVar(&cfg.SQLOutputDir, "sql-output", "", "Output directory for schema.sql and query.sql (default: <output>/internal/db)")
	flag.StringVar(&cfg.PackageName, "package", "pkgsql", "Go package name for generated files")
	flag.Parse()

	if cfg.TablesFile == "" {
		fmt.Fprintln(os.Stderr, "error: -tables flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if err := sqlgen.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
