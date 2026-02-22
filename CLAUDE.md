# go-package-spec

Go data model and reader for Elastic [package-spec](https://github.com/andrewkroh/package-spec-schema) packages. See [REQS.md](REQS.md) for original requirements.

## Project layout

```
cmd/generate/main.go          CLI entry point for code generator
internal/generator/            Code generation pipeline
  augment.go                   Type/field overrides + extra_fields + base_types
  emitter.go                   Go source emission via dave/jennifer/jen
  filemap.go                   Maps types to output files
  generator.go                 Orchestrator (Run pipeline)
  loader.go                    JSON Schema parser + $ref resolution
  naming.go                    Go identifier conventions
  typemap.go                   JSON Schema -> GoType conversion
packagespec/                   Generated data model (DO NOT EDIT except annotation.go)
  annotation.go                Hand-written: exports AnnotateFileMetadata
  metadata.go                  Generated: FileMetadata type + reflection walker
  manifest.go                  Manifest base type + Integration/Input/Content manifests
  *.go                         Other generated types (changelog, field, transform, etc.)
reader/                        Package reader (loads from disk into packagespec types)
  reader.go                    Read() entry point, Package type, options
  decode.go                    YAML decoding helpers
  datastream.go                DataStream + FieldsFile types
  transform.go                 TransformData type
  git.go                       Git commit + git blame for changelog dates
augment.yml                    Type/field augmentation config
filemap.yml                    Type -> output file mapping
```

## Code generation

### Running the generator

```sh
go run ./cmd/generate/ \
  -schema-dir ../package-spec-schema/3.5.7/jsonschema \
  -augment augment.yml \
  -filemap filemap.yml \
  -output packagespec \
  -package packagespec
```

All files in `packagespec/` except `annotation.go` are generated. Never hand-edit them.

### Generator pipeline

1. Load JSON schemas from `-schema-dir` via `SchemaRegistry`
2. Convert to Go types via `TypeMapper` (handles $ref, allOf, if/then/else, enums)
3. `ApplyAugmentations` — renames, doc overrides, field type overrides, extra fields
4. `ApplyBaseTypes` — extracts common fields into shared base types (e.g. `Manifest`)
5. Assign output files via `FileMap`
6. Validate (enum collision check)
7. Emit Go files via `Emitter` (jennifer/jen)

### augment.yml features

- **Type overrides**: rename types, override docs, override field types/names/docs
- **`extra_fields`**: inject fields not in the schema (e.g. `Changelog.Date *time.Time`)
- **`base_types`**: extract common fields from multiple types into a shared base type with embedding (e.g. `Manifest` from Integration/Input/ContentManifest)

### Key design decisions

- **FileMetadata embedding**: Entry-point types embed `FileMetadata` to track source file/line/column. The `UnmarshalYAML` method uses a type-alias trick to capture YAML node position without infinite recursion.
- **Base type YAML workaround**: go-yaml's type-alias trick doesn't populate promoted fields from Go embeds. The emitter generates a second `node.Decode(&v.BaseType)` call for types embedding a base type.
- **Base types don't get UnmarshalYAML**: Only concrete types (IntegrationManifest, etc.) have UnmarshalYAML. The base type (Manifest) embeds FileMetadata but has no UnmarshalYAML to avoid conflicts.
- **Required vs optional**: Required fields use bare struct tags (`json:"name"`), optional fields use `omitempty` (`json:"title,omitempty"`). Only optional booleans use pointer types (`*bool`).
- **Qualified types**: `parseTypeRef` handles `pkg.Type` patterns (e.g. `time.Time`) via `GoTypeRef.Package`/`QualName`, emitted as `jen.Qual()`.

## Reader package

- Uses `io/fs.FS` for filesystem abstraction (testable with `fstest.MapFS`)
- Detects package type from `manifest.yml` `type` field
- Options: `WithFS()`, `WithKnownFields()`, `WithGitMetadata()`
- `Package.Manifest()` returns the common `*packagespec.Manifest` for any package type
- Transform files always decoded with `knownFields=false` (contain arbitrary ES DSL)
- Git operations require real filesystem path (shell out to `git`)

## Testing

```sh
go test ./...                                                    # unit tests
INTEGRATIONS_DIR=/path/to/integrations go test ./reader/ -run TestReadAllPackages  # all real packages
```

- Generator tests: schema loading, type mapping, augmentation, naming
- Packagespec tests: YAML unmarshaling with real 1password package (skipped if unavailable)
- Reader tests: synthetic testdata packages + optional integration test against all real packages

## Go practices

- Never use `github.com/stretchr/testify`
- Add conventional Go doc comments to all exported types, functions, and methods
- Format with `gofumpt -extra`
- Use `path.Join` (not `filepath.Join`) inside reader for `fs.FS` compatibility (forward slashes)
- Avoid import aliases unless necessary to avoid conflicts.
