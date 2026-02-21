// Command generate reads upstream JSON Schema definitions and produces
// Go data model types for the packagespec package.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/andrewkroh/go-package-spec/internal/generator"
)

func main() {
	cfg := generator.Config{}

	flag.StringVar(&cfg.SchemaDir, "schema-dir", "../package-spec-schema/3.5.7/jsonschema", "Path to jsonschema/ directory")
	flag.StringVar(&cfg.AugmentFile, "augment", "", "Path to augment.yml (optional)")
	flag.StringVar(&cfg.FileMapFile, "filemap", "", "Path to filemap.yml (optional)")
	flag.StringVar(&cfg.OutputDir, "output", "packagespec", "Output directory for generated Go files")
	flag.StringVar(&cfg.PackageName, "package", "packagespec", "Go package name for generated files")
	flag.Parse()

	if err := generator.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
