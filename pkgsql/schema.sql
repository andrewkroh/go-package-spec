CREATE TABLE IF NOT EXISTS fields (
  -- Elasticsearch field definitions, flattened from nested YAML into dotted-path names.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  analyzer TEXT, -- Analyzer
  copy_to TEXT, -- CopyTo
  date_format TEXT, -- DateFormat
  default_metric TEXT, -- JSON-encoded DefaultMetric
  description TEXT, -- Description
  dimension BOOLEAN, -- Dimension
  doc_values BOOLEAN, -- DocValues
  dynamic TEXT, -- JSON-encoded Dynamic
  enabled BOOLEAN, -- Enabled
  example TEXT, -- JSON-encoded Example
  expected_values TEXT, -- JSON-encoded ExpectedValues
  external TEXT, -- External
  ignore_above INTEGER, -- IgnoreAbove
  ignore_malformed BOOLEAN, -- IgnoreMalformed
  include_in_parent BOOLEAN, -- IncludeInParent
  include_in_root BOOLEAN, -- IncludeInRoot
  "index" BOOLEAN, -- Index
  inference_id TEXT, -- InferenceID
  metric_type TEXT, -- MetricType
  metrics TEXT, -- JSON-encoded Metrics
  name TEXT NOT NULL, -- Name
  normalize TEXT, -- JSON-encoded Normalize
  normalizer TEXT, -- Normalizer
  null_value TEXT, -- JSON-encoded NullValue
  object_type TEXT, -- ObjectType
  object_type_mapping_type TEXT, -- ObjectTypeMappingType
  path TEXT, -- Path
  pattern TEXT, -- Pattern
  runtime TEXT, -- JSON-encoded Runtime
  scaling_factor INTEGER, -- ScalingFactor
  search_analyzer TEXT, -- SearchAnalyzer
  store BOOLEAN, -- Store
  subobjects BOOLEAN, -- Subobjects
  type TEXT, -- Type
  unit TEXT, -- Unit
  value TEXT -- Value
);

CREATE TABLE IF NOT EXISTS packages (
  -- Fleet packages (integration, input, or content). Each row is one package version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  dir_name TEXT NOT NULL UNIQUE, -- directory name of the package
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description
  format_version TEXT NOT NULL, -- FormatVersion
  name TEXT NOT NULL, -- Name
  owner_github TEXT NOT NULL, -- Owner.Github
  owner_type TEXT NOT NULL, -- Owner.Type
  source_license TEXT, -- Source.License
  title TEXT NOT NULL, -- Title
  type TEXT NOT NULL, -- Type
  version TEXT NOT NULL -- Version
);

CREATE TABLE IF NOT EXISTS build_manifests (
  -- Build configuration for integration packages (_dev/build/build.yml).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dependencies_ecs_import_mappings BOOLEAN, -- Dependencies.ECS.ImportMappings
  dependencies_ecs_reference TEXT NOT NULL -- Dependencies.ECS.Reference
);

CREATE TABLE IF NOT EXISTS changelogs (
  -- Changelog versions for a package. Each row is one version entry with its release date.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  version TEXT NOT NULL, -- Version
  date TEXT -- Date
);

CREATE TABLE IF NOT EXISTS changelog_entries (
  -- Individual changelog entries within a changelog version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  changelogs_id INTEGER NOT NULL REFERENCES changelogs(id), -- foreign key to changelogs
  description TEXT NOT NULL, -- Description
  link TEXT NOT NULL, -- Link
  type TEXT NOT NULL -- Type
);

CREATE TABLE IF NOT EXISTS data_streams (
  -- Data streams within integration packages. Each row is one data stream with its Elasticsearch and agent config.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the data stream
  dataset TEXT, -- Dataset
  dataset_is_prefix BOOLEAN, -- DatasetIsPrefix
  deprecated TEXT, -- JSON-encoded Deprecated
  elasticsearch_dynamic_dataset BOOLEAN, -- Elasticsearch.DynamicDataset
  elasticsearch_dynamic_namespace BOOLEAN, -- Elasticsearch.DynamicNamespace
  elasticsearch_index_mode TEXT, -- Elasticsearch.IndexMode
  elasticsearch_index_template TEXT, -- JSON-encoded IndexTemplate
  elasticsearch_privileges TEXT, -- JSON-encoded Privileges
  elasticsearch_source_mode TEXT, -- Elasticsearch.SourceMode
  hidden BOOLEAN, -- Hidden
  ilm_policy TEXT, -- ILMPolicy
  "release" TEXT, -- Release
  title TEXT NOT NULL, -- Title
  type TEXT -- Type
);

CREATE TABLE IF NOT EXISTS data_stream_fields (
  -- Join table linking fields to data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_stream_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  field_id INTEGER NOT NULL REFERENCES fields(id) -- foreign key to fields
);

CREATE TABLE IF NOT EXISTS ingest_pipelines (
  -- Elasticsearch ingest pipeline definitions within data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  file_name TEXT NOT NULL, -- file name of the pipeline (e.g. default.yml)
  description TEXT -- Description
);

CREATE TABLE IF NOT EXISTS ingest_processors (
  -- Individual ingest processors flattened from pipelines. Nested on_failure handlers are included as separate rows.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  ingest_pipelines_id INTEGER NOT NULL REFERENCES ingest_pipelines(id), -- foreign key to ingest_pipelines
  json_pointer TEXT NOT NULL, -- RFC 6901 JSON Pointer location within the pipeline
  ordinal INTEGER NOT NULL, -- order of processor within the pipeline
  type TEXT NOT NULL, -- processor type (e.g. set, grok, rename)
  attributes TEXT -- JSON-encoded processor attributes
);

CREATE TABLE IF NOT EXISTS package_categories (
  -- Categories assigned to a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  package_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  category TEXT NOT NULL -- category value
);

CREATE TABLE IF NOT EXISTS package_fields (
  -- Join table linking fields to packages (for input packages).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  package_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  field_id INTEGER NOT NULL REFERENCES fields(id) -- foreign key to fields
);

CREATE TABLE IF NOT EXISTS package_icons (
  -- Icon definitions for a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dark_mode BOOLEAN, -- DarkMode
  size TEXT, -- Size
  src TEXT NOT NULL, -- Src
  title TEXT, -- Title
  type TEXT -- Type
);

CREATE TABLE IF NOT EXISTS package_screenshots (
  -- Screenshot definitions for a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  size TEXT, -- Size
  src TEXT NOT NULL, -- Src
  title TEXT NOT NULL, -- Title
  type TEXT -- Type
);

CREATE TABLE IF NOT EXISTS policy_templates (
  -- Policy templates offered by integration packages. Defines how an integration is configured in Fleet.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  configuration_links TEXT, -- JSON-encoded ConfigurationLinks
  data_streams TEXT, -- DataStreams
  deployment_modes TEXT, -- JSON-encoded DeploymentModes
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description
  fips_compatible BOOLEAN, -- FipsCompatible
  multiple BOOLEAN, -- Multiple
  name TEXT NOT NULL, -- Name
  title TEXT NOT NULL -- Title
);

CREATE TABLE IF NOT EXISTS policy_template_categories (
  -- Categories assigned to a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_template_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  category TEXT NOT NULL -- category value
);

CREATE TABLE IF NOT EXISTS policy_template_inputs (
  -- Inputs defined within a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_templates_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  deployment_modes TEXT, -- JSON-encoded DeploymentModes
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description
  hide_in_var_group_options TEXT, -- JSON-encoded HideInVarGroupOptions
  input_group TEXT, -- InputGroup
  multi BOOLEAN, -- Multi
  template_path TEXT, -- TemplatePath
  title TEXT NOT NULL, -- Title
  type TEXT NOT NULL -- Type
);

CREATE TABLE IF NOT EXISTS routing_rules (
  -- Routing rules for rerouting documents from a source dataset (technical preview).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  "if" TEXT NOT NULL, -- If
  namespace TEXT, -- JSON-encoded Namespace
  target_dataset TEXT -- JSON-encoded TargetDataset
);

CREATE TABLE IF NOT EXISTS streams (
  -- Streams offered by a data stream.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  description TEXT NOT NULL, -- Description
  enabled BOOLEAN, -- Enabled
  input TEXT NOT NULL, -- Input
  template_path TEXT, -- TemplatePath
  title TEXT NOT NULL -- Title
);

CREATE TABLE IF NOT EXISTS tags (
  -- Kibana tags associated with integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  asset_ids TEXT, -- JSON-encoded AssetIDs
  asset_types TEXT, -- JSON-encoded AssetTypes
  text TEXT -- Text
);

CREATE TABLE IF NOT EXISTS transforms (
  -- Elasticsearch transform configurations within integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the transform
  meta TEXT, -- JSON-encoded Meta
  description TEXT, -- Description
  dest TEXT, -- JSON-encoded Dest
  frequency TEXT, -- Frequency
  latest TEXT, -- JSON-encoded Latest
  pivot TEXT, -- JSON-encoded Pivot
  retention_policy TEXT, -- JSON-encoded RetentionPolicy
  settings TEXT, -- JSON-encoded Settings
  source TEXT, -- JSON-encoded Source
  sync TEXT -- JSON-encoded Sync
);

CREATE TABLE IF NOT EXISTS vars (
  -- Input variable definitions. Linked to packages, policy templates, streams, or inputs via join tables.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  "default" TEXT, -- JSON-encoded Default
  description TEXT, -- Description
  hide_in_deployment_modes TEXT, -- JSON-encoded HideInDeploymentModes
  max_duration TEXT, -- MaxDuration
  min_duration TEXT, -- MinDuration
  multi BOOLEAN, -- Multi
  name TEXT NOT NULL, -- Name
  options TEXT, -- JSON-encoded Options
  required BOOLEAN, -- Required
  secret BOOLEAN, -- Secret
  show_user BOOLEAN, -- ShowUser
  title TEXT, -- Title
  type TEXT NOT NULL, -- Type
  url_allowed_schemes TEXT -- JSON-encoded URLAllowedSchemes
);

CREATE TABLE IF NOT EXISTS package_vars (
  -- Join table linking vars to packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  package_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  var_id INTEGER NOT NULL REFERENCES vars(id) -- foreign key to vars
);

CREATE TABLE IF NOT EXISTS policy_template_input_vars (
  -- Join table linking vars to policy template inputs.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_template_input_id INTEGER NOT NULL REFERENCES policy_template_inputs(id), -- foreign key to policy_template_inputs
  var_id INTEGER NOT NULL REFERENCES vars(id) -- foreign key to vars
);

CREATE TABLE IF NOT EXISTS policy_template_vars (
  -- Join table linking vars to policy templates.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_template_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  var_id INTEGER NOT NULL REFERENCES vars(id) -- foreign key to vars
);

CREATE TABLE IF NOT EXISTS stream_vars (
  -- Join table linking vars to streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  stream_id INTEGER NOT NULL REFERENCES streams(id), -- foreign key to streams
  var_id INTEGER NOT NULL REFERENCES vars(id) -- foreign key to vars
);
