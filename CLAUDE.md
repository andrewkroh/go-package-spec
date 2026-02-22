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
pkgspec/                   Generated data model (DO NOT EDIT except hand-written files below)
  annotation.go                Hand-written: exports AnnotateFileMetadata
  processor.go                 Hand-written: Processor type with custom marshal/unmarshal
  stringorstrings.go           Hand-written: StringOrStrings type for anyOf [string, []string]
  metadata.go                  Generated: FileMetadata type + reflection walker
  manifest.go                  Manifest base type + Integration/Input/Content manifests
  ingest_pipeline.go           Generated: IngestPipeline type
  routing_rules.go             Generated: RoutingRuleSet, RoutingRule types
  *.go                         Other generated types (changelog, field, transform, etc.)
pkgreader/                        Package reader (loads from disk into pkgspec types)
  reader.go                    Read() entry point, Package type, options
  decode.go                    YAML decoding helpers
  datastream.go                DataStream + FieldsFile + PipelineFile types
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
  -output pkgspec \
  -package pkgspec
```

All files in `pkgspec/` except `annotation.go` and `processor.go` are generated. Never hand-edit them.

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

## Package structure (from package-spec, excluding _dev/)

### Integration package

```
manifest.yml                                      required  schema:integration/manifest.spec.yml
changelog.yml                                     required  schema:integration/changelog.spec.yml
validation.yml                                    optional  schema:integration/validation.spec.yml
NOTICE.txt                                        optional  plain text
LICENSE.txt                                       optional  plain text
docs/                                             required
  README.md                                       required  markdown (usually generated)
  *.md                                            optional  markdown
  knowledge_base/*.md                             optional  markdown (AI assistant context)
img/                                              optional  images (additionalContents: true)
agent/                                            optional
  input/stream/*.yml.hbs                          required  Handlebars templates
kibana/                                           optional
  tags.yml                                        optional  schema:integration/kibana/tags.spec.yml
  dashboard/*.json                                optional  opaque JSON saved objects
  visualization/*.json                            optional  "
  search/*.json                                   optional  "
  map/*.json                                      optional  "
  lens/*.json                                     optional  "
  index_pattern/*.json                            optional  "
  security_rule/*.json                            optional  "
  csp_rule_template/*.json                        optional  "
  ml_module/*.json                                optional  "
  tag/*.json                                      optional  "
  osquery_pack_asset/*.json                       optional  "
  osquery_saved_query/*.json                      optional  "
  alerting_rule_template/*.json                   optional  "
  slo_template/*.json                             optional  "
elasticsearch/                                    optional
  ingest_pipeline/*.yml|*.json                    optional  schema:integration/elasticsearch/pipeline.spec.yml
  ml_model/{PKG_NAME}_*.json                      optional  opaque JSON
  transform/<name>/                               optional
    transform.yml                                 required  schema:elasticsearch/transform/transform.spec.yml
    manifest.yml                                  optional  schema:elasticsearch/transform/manifest.spec.yml
    fields/*.yml                                  optional  schema:data_stream/fields/fields.spec.yml
  esql_view/*.yml                                 optional  schema:elasticsearch/view.spec.yml (3.6.0+)
data_stream/<name>/                               optional
  manifest.yml                                    required  schema:data_stream/manifest.spec.yml
  fields/*.yml                                    required  schema:data_stream/fields/fields.spec.yml
  sample_event.json                               optional  opaque JSON
  routing_rules.yml                               optional  schema:data_stream/routing_rules.spec.yml (tech preview)
  lifecycle.yml                                   optional  schema:data_stream/lifecycle.spec.yml (tech preview)
  agent/stream/*.yml.hbs                          optional  Handlebars templates
  elasticsearch/
    ingest_pipeline/*.yml|*.json                   optional  schema:elasticsearch/pipeline.spec.yml
    ilm/*.yml|*.json                               optional  opaque YAML/JSON
```

### Input package

```
manifest.yml                                      required  schema:input/manifest.spec.yml
changelog.yml                                     required  schema:integration/changelog.spec.yml
LICENSE.txt                                       optional  plain text
validation.yml                                    optional  schema:integration/validation.spec.yml
lifecycle.yml                                     optional  schema:data_stream/lifecycle.spec.yml (tech preview)
sample_event.json                                 optional  opaque JSON
agent/                                            required
  input/*.yml.hbs                                 required  Handlebars templates
docs/                                             required
  README.md                                       required  markdown
  *.md                                            optional  markdown
  knowledge_base/*.md                             optional  markdown
fields/*.yml                                      optional  schema:data_stream/fields/fields.spec.yml
img/                                              optional  images
```

### Content package

```
manifest.yml                                      required  schema:content/manifest.spec.yml
changelog.yml                                     required  schema:integration/changelog.spec.yml
LICENSE.txt                                       optional  plain text
validation.yml                                    optional  schema:integration/validation.spec.yml
docs/                                             required
  README.md                                       required  markdown
  *.md                                            optional  markdown
  knowledge_base/*.md                             optional  markdown
img/                                              optional  images
kibana/                                           optional
  tags.yml                                        optional  schema:integration/kibana/tags.spec.yml
  dashboard/*.json                                optional  opaque JSON saved objects
  security_ai_prompt/*.json                       optional  "
  security_rule/*.json                            optional  "
  alerting_rule_template/*.json                   optional  "
  slo_template/*.json                             optional  "
elasticsearch/                                    optional
  esql_view/*.yml                                 optional  schema:elasticsearch/view.spec.yml (3.6.0+)
```

## Package reader (pkgreader)

- Uses `io/fs.FS` for filesystem abstraction (testable with `fstest.MapFS`)
- Detects package type from `manifest.yml` `type` field
- Options: `WithFS()`, `WithKnownFields()`, `WithGitMetadata()`
- `Package.Manifest()` returns the common `*pkgspec.Manifest` for any package type
- Transform and pipeline files always decoded with `knownFields=false` (contain arbitrary ES DSL)
- Ingest pipelines loaded from `data_stream/<name>/elasticsearch/ingest_pipeline/*.yml`
- Git operations require real filesystem path (shell out to `git`)

## Testing

```sh
go test ./...                                                    # unit tests
INTEGRATIONS_DIR=/path/to/integrations go test ./pkgreader/ -run TestReadAllPackages  # all real packages
```

- Generator tests: schema loading, type mapping, augmentation, naming
- pkgspec tests: YAML unmarshaling with real 1password package (skipped if unavailable)
- pkgreader tests: synthetic testdata packages + optional integration test against all real packages

## Go practices

- Never use `github.com/stretchr/testify`
- Add conventional Go doc comments to all exported types, functions, and methods
- Format with `gofumpt -extra`
- Use `path.Join` (not `filepath.Join`) inside pkgreader for `fs.FS` compatibility (forward slashes)
- Avoid import aliases unless necessary to avoid conflicts.
