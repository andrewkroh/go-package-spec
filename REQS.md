# go-package-spec

This project provides a Go data model based on the [package-spec schema][package-spec].

## Code generator

- Utilizes [google/jsonschema-go][google-jsonschema-go] (or a cloned and owned fork) for reading the upstream package-spec-schema contents.
- Automated code generation based on the upstream JSONSchema specification.
- Type names and code style that follow Go conventions.
- Augments type and field names, data types, and documention when upstream information is unclear or doesn't align with Go conventions. Augmentation and override information (type names, type docs, field docs) are stored separately from the code generator Go code in a YAML file.
- Data model types include file metadata (file path, line number, column number) to track the location where the definition was loaded from.
- Go code generation is implemented using github.com/dave/jennifer/jen, not templates.
- Types are organized into files on a semantic basis (e.g. datastream.go, integration.go, field.go, systemtest.go, pipeline.go, transform.go). This should be decided once via LLM analysis, then codified such that it is deterministic on future code generator runs.

## Reader Go package

- The data model package can be used independently of the package that provides the functions for reading package contents from the filesystem.
- The reader package (name TBD) has a function that accepts the directory containing the package (e.g. packages/1password) and a set of options (using Go options design pattern), and then it reads all package contents into the associated data model types and adds the file metadata annotations.
- The goal is to be able to unmarshal files in their native format.
- Round-trip marshaling and preserving the exact file format is not a goal.

## Reader options

- strict mode: By default the reader does not enforce "known fields" mode on unmarshalers to allow flexible forward compatibility. Through the WithKnownFields() option, users can turn on strict validation where only fields defined in the model types are allowed (unless the underlying field itselfis declared as a generic object). This delegates to the underlying YAML or JSON decoder library.
- git metadata: When enabled (off by default) the library may use git metadata from the directory containing the package to enrich model types. The primary uses for this are to determine git commit ID associated with the package, and to use git blame to determine when release were created based on the changelog file.

Prior art: see gitblame.go in github.com/andrewkroh/go-fleetpkg

## ECS field enrichment

Fields are included in the data model matching how they are represented in the package.
A helper is offerered to transform fields into a more user friendly representation. The helper flattens key names and joins/enriches them with their Elastic Common Schema defintions.
It uses the github.com/andrewkroh/go-ecs library to obtain the ECS defintion for fields that are
defined using 'external: ecs'.

Prior art: see FlattenFields in github.com/andrewkroh/go-fleetpkg 

[package-spec]: https://github.com/andrewkroh/package-spec-schema
[google-jsonschema-go]: https://pkg.go.dev/github.com/google/jsonschema-go@v0.4.2/jsonschema

# Package structure

## `type: integration` packages

These are the most common. Each contains one or more **data streams**, along with Kibana assets and Elasticsearch transforms.

```
<package>/
├── manifest.yml                          # Package metadata (name, version, type, inputs, policy templates, etc.)
├── changelog.yml                         # Version history
├── validation.yml                        # Validation overrides / exclusions
├── docs/
│   └── README.md                         # User-facing documentation (rendered in Kibana)
├── img/                                  # Logo SVG + dashboard screenshot PNGs
│   ├── logo.svg
│   └── *.png
│
├── data_stream/<name>/                   # One directory per data stream (e.g. "alert", "falcon", "url")
│   ├── manifest.yml                      # Data stream config (title, type, input types)
│   ├── sample_event.json                 # Example document
│   ├── lifecycle.yml                     # ILM / data stream lifecycle config (optional)
│   ├── agent/stream/                     # Agent input templates (one per input type)
│   │   ├── cel.yml.hbs                   #   Handlebars templates rendered by Fleet
│   │   ├── log.yml.hbs
│   │   └── streaming.yml.hbs
│   ├── elasticsearch/
│   │   ├── ingest_pipeline/
│   │   │   ├── default.yml              # Main ingest pipeline
│   │   │   └── *.yml                    # Sub-pipelines (called from default)
│   │   └── ilm/
│   │       └── default_policy.json      # ILM policy (optional)
│   ├── fields/                           # Field definitions (merged to produce mappings)
│   │   ├── base-fields.yml              # data_stream.*, @timestamp, event.module, etc.
│   │   ├── ecs.yml                      # ECS field imports
│   │   ├── fields.yml                   # Package-specific fields
│   │   ├── agent.yml                    # Agent-related fields (optional)
│   │   └── beats.yml                    # Beats compat fields (optional)
│   └── _dev/                             # Development & test assets (not shipped)
│       ├── test/
│       │   ├── pipeline/                # Ingest pipeline unit tests (input log + expected JSON)
│       │   ├── policy/                  # Policy rendering tests (.yml input + .expected output)
│       │   └── system/                  # System (integration) test configs
│       └── benchmark/
│           └── pipeline/                # Pipeline benchmark configs
│
├── elasticsearch/                        # Package-level ES assets (optional)
│   └── transform/<name>/                # Elasticsearch transforms
│       ├── manifest.yml
│       ├── transform.yml
│       └── fields/                      # Transform destination index fields
│
├── kibana/                               # Kibana saved objects
│   ├── dashboard/*.json                  # Dashboards
│   ├── search/*.json                     # Saved searches (optional)
│   ├── tag/*.json                        # Kibana tags (optional)
│   └── tags.yml                          # Tag definitions
│
└── _dev/                                 # Package-level dev/test infrastructure (not shipped)
    ├── build/
    │   ├── build.yml                     # Build-time config (e.g. field dependency overrides)
    │   └── docs/README.md                # Build-time doc overrides
    ├── deploy/
    │   ├── docker/                       # Docker-compose for local test environment
    │   │   ├── docker-compose.yml
    │   │   └── files/                    # Mock server configs / sample data
    │   └── tf/                           # Terraform configs for cloud tests (optional, e.g. S3)
    └── benchmark/
        ├── rally/                        # Rally (ES) benchmark definitions
        └── system/                       # System benchmark definitions
```

## `type: input` packages

These are simpler — they provide a reusable **input** (no data streams, no ingest pipelines, no dashboards). The agent template lives directly under `agent/input/` instead of inside a data stream.

```
<package>/
├── manifest.yml                          # Package metadata (type: input, policy templates with input vars)
├── changelog.yml
├── docs/
│   └── README.md
├── sample_event.json                     # Example document
├── agent/
│   └── input/
│       └── input.yml.hbs                 # Agent input template (Handlebars)
├── fields/
│   └── input.yml                         # Field definitions for the input
├── kibana/
│   └── tags.yml                          # Tag definitions (usually minimal)
└── _dev/
    ├── build/
    │   └── build.yml
    ├── deploy/docker/                    # Docker-compose for local testing
    └── test/
        ├── policy/                       # Policy rendering tests
        └── system/                       # System integration tests
```

**Key differences:**
- **Integration** packages have `data_stream/*/` directories, each with their own manifest, fields, pipelines, and agent stream templates.
- **Input** packages have no data streams — the agent template is at `agent/input/input.yml.hbs` and fields live at the package root in `fields/`.
- **Integration** packages commonly include Kibana dashboards/searches and may include Elasticsearch transforms. Input packages typically have minimal or no Kibana assets.
- The `_dev/` tree is never shipped — it holds Docker/Terraform test infrastructure, pipeline/policy/system tests, and benchmarks.

# General Go practices

- Never use github.com/stretchr/testify.
- Add conventional Go doc comments to all exported types, functions, and methods.

