// Command flatten_fields reads an Elastic package and prints a flat,
// sorted list of all data stream fields with their types and source
// locations. Group fields are expanded into dot-joined names.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/andrewkroh/go-package-spec/pkgreader"
	"github.com/andrewkroh/go-package-spec/pkgspec"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <package-path>\n", os.Args[0])
		os.Exit(1)
	}

	pkg, err := pkgreader.Read(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	if pkg.Build != nil {
		fmt.Printf("build ecs reference: %s\n\n", pkg.Build.Dependencies.ECS.Reference)
	}

	for name, ds := range pkg.DataStreams {
		fmt.Printf("data_stream: %s\n", name)

		flat := pkgspec.FlattenFields(ds.AllFields(), nil)
		for _, f := range flat {
			loc := fmt.Sprintf("%s:%d:%d", f.FilePath(), f.Line(), f.Column())
			ext := ""
			if f.External == pkgspec.FieldExternalECS {
				ext = " [ecs]"
			}
			fmt.Printf("  %-50s %-20s %s%s\n", f.Name, f.Type, loc, ext)
		}
		fmt.Println()
	}
}
