// Command processor_count reads an Elastic package and prints a summary of
// ingest pipeline processor counts by type. It walks all data stream pipelines
// and package-level pipelines, including nested on_failure processors.
package main

import (
	"cmp"
	"fmt"
	"log"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/andrewkroh/go-package-spec/pkgspec"
	"github.com/andrewkroh/go-package-spec/pkgreader"
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

	counts := map[string]int{}

	// Count processors in data stream pipelines.
	for _, ds := range pkg.DataStreams {
		for _, pf := range ds.Pipelines {
			countProcessors(pf.Pipeline.Processors, counts)
			countProcessors(pf.Pipeline.OnFailure, counts)
		}
	}

	// Count processors in package-level pipelines.
	for _, pf := range pkg.Pipelines {
		countProcessors(pf.Pipeline.Processors, counts)
		countProcessors(pf.Pipeline.OnFailure, counts)
	}

	if len(counts) == 0 {
		fmt.Println("No processors found.")
		return
	}

	// Sort by count descending, then by name.
	type entry struct {
		name  string
		count int
	}
	entries := make([]entry, 0, len(counts))
	for name, count := range counts {
		entries = append(entries, entry{name, count})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if c := cmp.Compare(b.count, a.count); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})

	total := 0
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "PROCESSOR\tCOUNT\n")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%d\n", e.name, e.count)
		total += e.count
	}
	fmt.Fprintf(tw, "\t\n")
	fmt.Fprintf(tw, "TOTAL\t%d\n", total)
	tw.Flush()
}

// countProcessors recursively counts processors by type, including nested
// on_failure processors.
func countProcessors(processors []*pkgspec.Processor, counts map[string]int) {
	for _, proc := range processors {
		counts[proc.Type]++
		countProcessors(proc.OnFailure, counts)
	}
}
