// Command field_locations reads an Elastic package and prints each data stream
// field with its source file:line:column location. This demonstrates the
// FileMetadata annotation feature, which is useful for IDE plugins, MCP
// servers, or diagnostic tools that need to map a field back to its definition.
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

	for name, ds := range pkg.DataStreams {
		fmt.Printf("data_stream: %s\n", name)
		for _, field := range ds.AllFields() {
			printField(field, "")
		}
		fmt.Println()
	}
}

func printField(field pkgspec.Field, indent string) {
	loc := fmt.Sprintf("%s:%d:%d", field.FilePath(), field.Line(), field.Column())
	fmt.Printf("%s%-40s %-12s %s\n", indent, field.Name, field.Type, loc)

	for _, sub := range field.Fields {
		printField(sub, indent+"  ")
	}
}
