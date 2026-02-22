# go-package-spec

Go library for
reading [Elastic integration packages](https://github.com/elastic/integrations)
into typed Go structs.

[![Go Documentation](https://pkg.go.dev/badge/github.com/andrewkroh/go-package-spec.svg)](https://pkg.go.dev/github.com/andrewkroh/go-package-spec)

## Overview

`go-package-spec` provides a typed data model and package reader for Elastic
packages (integration, input, and content). The data model is generated
from [package-spec JSON schemas](https://github.com/andrewkroh/package-spec-schema),
ensuring it stays in sync with the specification.

The upstream [elastic/package-spec](https://github.com/elastic/package-spec)
defines its schemas in a non-standard YAML format (`.spec.yml` files) where the
schema is nested under a `spec` key, `$id` and `$ref` usage doesn't conform to
JSON Schema conventions.
The [package-spec-schema](https://github.com/andrewkroh/package-spec-schema)
project converts these into
standard [JSON Schema (draft 2020-12)](https://json-schema.org/draft/2020-12)
with proper `$id` URIs and `$ref` resolution, which this library's code
generator consumes.

## Features

- Typed data model generated from package-spec JSON schemas (manifests, fields,
  pipelines, changelogs, etc.)
- Package reader that loads full packages from disk via `io/fs.FS`
- File metadata annotations — every decoded type tracks its source file, line,
  and column
- Ingest pipeline processor parsing with type detection
- Kibana saved object loading with partial attribute decoding
- Git metadata enrichment (commit ID, changelog dates via blame)
- Image metadata extraction (dimensions, byte sizes)
- Agent Handlebars template loading
- Field flattening — expands nested group hierarchies into dot-joined names
  with optional [ECS](https://github.com/andrewkroh/go-ecs) enrichment via
  callback

## Install

```sh
go get github.com/andrewkroh/go-package-spec
```

## Quick start

```go
package main

import (
	"fmt"
	"log"

	"github.com/andrewkroh/go-package-spec/pkgreader"
)

func main() {
	pkg, err := pkgreader.Read("path/to/package")
	if err != nil {
		log.Fatal(err)
	}

	m := pkg.Manifest()
	fmt.Printf("%s %s (%s)\n", m.Name, m.Version, m.Description)

	for name, ds := range pkg.DataStreams {
		fmt.Printf("  data_stream: %s (%d fields, %d pipelines)\n",
			name, len(ds.AllFields()), len(ds.Pipelines))
	}
}
```

## Examples

See the [`example/`](example/) directory:

- **[processor_count](example/processor_count/)** — Counts ingest pipeline
  processors by type across all pipelines in a package.
- **[field_locations](example/field_locations/)** — Prints each field's name
  with its source file:line:column location, demonstrating the FileMetadata
  annotation feature.
- **[flatten_fields](example/flatten_fields/)** — Flattens nested group fields
  into dot-joined names and prints sorted results with types and locations.

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
