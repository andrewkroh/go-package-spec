CREATE TABLE IF NOT EXISTS fields (
  -- Elasticsearch field definitions, flattened from nested YAML into dotted-path names.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  analyzer TEXT, -- Name of the analyzer to use for indexing. Unless search_analyzer is specified this analyzer is used for both indexing and searching. Only valid for 'type: text'.
  copy_to TEXT, -- The copy_to parameter allows you to copy the values of multiple fields into a group field, which can then be queried as a single field.
  date_format TEXT, -- The date format(s) that can be parsed. Type date format default to `strict_date_optional_time||epoch_millis`, see the [doc]. In JSON documents, dates are represented as strings. Elasticsearch uses ...
  default_metric JSON, -- JSON-encoded DefaultMetric
  description TEXT, -- Short description of field
  dimension BOOLEAN, -- Declare a field as dimension of time series. This is attached to the field as a `time_series_dimension` mapping parameter.
  doc_values BOOLEAN, -- Controls whether doc values are enabled for a field. All fields which support doc values have them enabled by default. If you are sure that you don’t need to sort or aggregate on a field, or acce...
  dynamic JSON, -- Dynamic controls whether new fields are added dynamically. Accepts true, false, "strict", or "runtime".
  enabled BOOLEAN, -- The enabled setting, which can be applied only to the top-level mapping definition and to object fields, causes Elasticsearch to skip parsing of the contents of the field entirely. The JSON can sti...
  example JSON, -- Example values for this field.
  expected_values JSON, -- An array of expected values for the field. When defined, these are the only expected values.
  external TEXT, -- External source reference
  ignore_above INTEGER, -- Strings longer than the ignore_above setting will not be indexed or stored. For arrays of strings, ignore_above will be applied for each array element separately and string elements longer than ign...
  ignore_malformed BOOLEAN, -- Trying to index the wrong data type into a field throws an exception by default, and rejects the whole document. The ignore_malformed parameter, if set to true, allows the exception to be ignored. ...
  include_in_parent BOOLEAN, -- For nested field types, this specifies if all fields in the nested object are also added to the parent document as standard (flat) fields.
  include_in_root BOOLEAN, -- For nested field types, this specifies if all fields in the nested object are also added to the root document as standard (flat) fields.
  "index" BOOLEAN, -- The index option controls whether field values are indexed. Fields that are not indexed are typically not queryable.
  inference_id TEXT, -- For semantic_text fields, this specifies the id of the inference endpoint associated with the field
  metric_type TEXT, -- The metric type of a numeric field. This is attached to the field as a `time_series_metric` mapping parameter. A gauge is a single-value measurement that can go up or down over time, such as a temp...
  metrics JSON, -- JSON-encoded Metrics
  multi_fields JSON, -- It is often useful to index the same field in different ways for different purposes. This is the purpose of multi-fields. For instance, a string field could be mapped as a text field for full-text ...
  name TEXT NOT NULL, -- Name of field. Names containing dots are automatically split into sub-fields. Names with wildcards generate dynamic mappings.
  normalize JSON, -- Specifies the expected normalizations for a field. `array` normalization implies that the values in the field should always be an array, even if they are single values.
  normalizer TEXT, -- Specifies the name of a normalizer to apply to keyword fields. A simple normalizer called lowercase ships with elasticsearch and can be used. Custom normalizers can be defined as part of analysis i...
  null_value JSON, -- The null_value parameter allows you to replace explicit null values with the specified value so that it can be indexed and searched. A null value cannot be indexed or searched. When a field is set ...
  object_type TEXT, -- Type of the members of the object when `type: object` is used. In these cases a dynamic template is created so direct subobjects of this field have the type indicated. When `object_type_mapping_typ...
  object_type_mapping_type TEXT, -- Type that members of a field of with `type: object` must have in the source document. This type corresponds to the data type detected by the JSON parser, and is translated to the `match_mapping_typ...
  path TEXT, -- For alias type fields this is the path to the target field. Note that this must be the full path, including any parent objects (e.g. object1.object2.field).
  pattern TEXT, -- Regular expression pattern matching the allowed values for the field. This is used for development-time data validation.
  runtime JSON, -- Runtime specifies if this field is evaluated at query time. Can be a boolean or a script string.
  scaling_factor INTEGER, -- The scaling factor to use when encoding values. Values will be multiplied by this factor at index time and rounded to the closest long value. For instance, a scaled_float with a scaling_factor of 1...
  search_analyzer TEXT, -- Name of the analyzer to use for searching. Only valid for 'type: text'.
  store BOOLEAN, -- By default, field values are indexed, but not stored. This means that the field can be queried, but the original field cannot be retrieved. Setting this value to true ensures that the field is also...
  subobjects BOOLEAN, -- Specifies if field names containing dots should be expanded into subobjects. For example, if this is set to `true`, a field named `foo.bar` will be expanded into an object with a field named `bar` ...
  type TEXT, -- Datatype of field. If the type is set to object, a dynamic mapping is created. In this case, if the name doesn't contain any wildcard, the wildcard is added as the last segment of the path.
  unit TEXT, -- Unit type to associate with a numeric field. This is attached to the field as metadata (via `meta`). By default, a field does not have a unit. The convention for percents is to use value 1 to mean ...
  value TEXT, -- The value to associate with a constant_keyword field.
  json_pointer TEXT -- JsonPointer is the RFC 6901 JSON Pointer to this field's location in the original fields file (e.g. /0/fields/1). Set by pkgreader after parsing.
);

CREATE TABLE IF NOT EXISTS packages (
  -- Fleet packages (integration, input, or content). Each row is one package version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  agent_privileges_root BOOLEAN, -- whether collection requires root privileges in the agent
  commit_id TEXT, -- git HEAD commit ID (populated when WithGitMetadata is used)
  conditions_elastic_subscription TEXT, -- required Elastic subscription level
  conditions_kibana_version TEXT, -- required Kibana version range
  dir_name TEXT NOT NULL UNIQUE, -- directory name of the package
  elasticsearch_privileges_cluster JSON, -- Elasticsearch cluster privilege requirements (JSON array)
  policy_templates_behavior TEXT, -- behavior when multiple policy templates are defined (all, combined_policy, individual_policies)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  description TEXT NOT NULL, -- Description
  format_version TEXT NOT NULL, -- The version of the package specification format used by this package.
  name TEXT NOT NULL, -- The name of the package.
  owner_github TEXT NOT NULL, -- Github team name of the package maintainer.
  owner_type TEXT NOT NULL, -- Describes who owns the package and the level of support that is provided. The 'elastic' value indicates that the package is built and maintained by Elastic. The 'partner' value indicates that the p...
  source_license TEXT, -- Identifier of the license of the package, as specified in https://spdx.org/licenses/.
  title TEXT NOT NULL, -- Title
  type TEXT NOT NULL, -- The type of package.
  version TEXT NOT NULL -- The version of the package.
);

CREATE TABLE IF NOT EXISTS build_manifests (
  -- Build configuration for integration packages (_dev/build/build.yml).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  dependencies_ecs_import_mappings BOOLEAN, -- Whether or not import common used dynamic templates and properties into the package
  dependencies_ecs_reference TEXT NOT NULL -- Reference is the ECS version source reference. Values begin with "git@" (e.g. "git@v8.11.0").
);

CREATE TABLE IF NOT EXISTS changelogs (
  -- Changelog versions for a package. Each row is one version entry with its release date.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  version TEXT NOT NULL, -- Package version.
  date TEXT -- Date is the approximate release date, populated via git blame when WithGitMetadata is used.
);

CREATE TABLE IF NOT EXISTS changelog_entries (
  -- Individual changelog entries within a changelog version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  changelogs_id INTEGER NOT NULL REFERENCES changelogs(id), -- foreign key to changelogs
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  description TEXT NOT NULL, -- Description of change.
  link TEXT NOT NULL, -- Link to issue or PR describing change in detail.
  type TEXT NOT NULL -- Type of change.
);

CREATE TABLE IF NOT EXISTS data_streams (
  -- Data streams within integration packages. Each row is one data stream with its Elasticsearch and agent config.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the data stream
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  dataset TEXT, -- Name of data set.
  dataset_is_prefix BOOLEAN, -- If true, the index pattern in the ES template will contain the dataset as a prefix only
  elasticsearch_dynamic_dataset BOOLEAN, -- When set to true, agents running this integration are granted data stream privileges for all datasets of its type
  elasticsearch_dynamic_namespace BOOLEAN, -- When set to true, agents running this integration are granted data stream privileges for all namespaces of its type
  elasticsearch_index_mode TEXT, -- Elasticsearch.IndexMode
  elasticsearch_index_template JSON, -- JSON-encoded IndexTemplate
  elasticsearch_privileges JSON, -- Elasticsearch privilege requirements
  elasticsearch_source_mode TEXT, -- Source mode to use. This configures how the document source (`_source`) is stored for this data stream. If configured as `default`, this mode is not configured and it uses Elasticsearch defaults. I...
  hidden BOOLEAN, -- Specifies if a data stream is hidden, resulting in dot prefixed system indices. To set the data stream hidden without those dot prefixed indices, check `elasticsearch.index_template.data_stream.hid...
  ilm_policy TEXT, -- The name of an existing ILM (Index Lifecycle Management) policy
  "release" TEXT, -- Stability of data stream.
  title TEXT NOT NULL, -- Title of data stream. It should include the source of the data that is being collected, and the kind of data collected such as logs or metrics. Words should be uppercased.
  type TEXT, -- Type of data stream
  github_owner TEXT -- GithubOwner is the GitHub team owner from CODEOWNERS, populated when WithCodeowners is used.
);

CREATE TABLE IF NOT EXISTS data_stream_fields (
  -- Join table linking fields to data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_stream_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  field_id INTEGER NOT NULL REFERENCES fields(id) -- foreign key to fields
);

CREATE TABLE IF NOT EXISTS discovery_fields (
  -- Fields associated with package discovery capabilities.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  name TEXT NOT NULL, -- name of the field
  packages_id INTEGER NOT NULL REFERENCES packages(id) -- foreign key to packages
);

CREATE TABLE IF NOT EXISTS docs (
  -- Documentation files within packages. Content is optionally populated when WithDocContent is used.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  content TEXT, -- markdown content (NULL unless WithDocContent was used)
  content_type TEXT NOT NULL, -- classification: readme, doc, or knowledge_base
  file_path TEXT NOT NULL, -- file path relative to the package root (e.g. docs/README.md)
  packages_id INTEGER NOT NULL REFERENCES packages(id) -- foreign key to packages
);

CREATE TABLE IF NOT EXISTS images (
  -- Image files within packages (img/ directory). Join with icon/screenshot tables on src to correlate declared metadata with actual image properties.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  byte_size INTEGER NOT NULL, -- file size in bytes
  height INTEGER, -- image height in pixels (NULL for SVG)
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  sha256 TEXT NOT NULL, -- hex-encoded SHA-256 hash of file contents
  src TEXT NOT NULL, -- image path with leading slash to match icon/screenshot src (e.g. /img/icon.png)
  width INTEGER -- image width in pixels (NULL for SVG)
);

CREATE TABLE IF NOT EXISTS ingest_pipelines (
  -- Elasticsearch ingest pipeline definitions within data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  file_name TEXT NOT NULL, -- file name of the pipeline (e.g. default.yml)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  description TEXT -- Description of the pipeline.
);

CREATE TABLE IF NOT EXISTS ingest_processors (
  -- Individual ingest processors flattened from pipelines. Nested on_failure handlers are included as separate rows.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  ingest_pipelines_id INTEGER NOT NULL REFERENCES ingest_pipelines(id), -- foreign key to ingest_pipelines
  attributes JSON, -- JSON-encoded processor attributes
  json_pointer TEXT NOT NULL, -- RFC 6901 JSON Pointer location within the pipeline
  ordinal INTEGER NOT NULL, -- order of processor within the pipeline
  type TEXT NOT NULL, -- processor type (e.g. set, grok, rename)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER -- source file column number
);

CREATE TABLE IF NOT EXISTS kibana_saved_objects (
  -- Kibana saved objects (dashboards, visualizations, security rules, etc.) from the kibana/ directory. Each row is one JSON file.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  asset_type TEXT NOT NULL, -- asset type directory name (e.g. dashboard, visualization, security_rule)
  core_migration_version TEXT, -- core Kibana migration version
  description TEXT, -- description from attributes
  file_path TEXT NOT NULL, -- file path relative to the package root
  managed BOOLEAN, -- whether the object is managed by Kibana
  object_id TEXT NOT NULL, -- unique identifier of the saved object
  object_type TEXT, -- object type from JSON (e.g. dashboard, visualization, search)
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  reference_count INTEGER NOT NULL, -- number of references to other saved objects
  title TEXT, -- human-readable title from attributes
  type_migration_version TEXT -- type-specific migration version
);

CREATE TABLE IF NOT EXISTS kibana_references (
  -- References between Kibana saved objects. Each row is one reference from a saved object to another, enabling dependency graph queries.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  kibana_saved_objects_id INTEGER NOT NULL REFERENCES kibana_saved_objects(id), -- foreign key to kibana_saved_objects
  ref_id TEXT NOT NULL, -- referenced object identifier
  ref_name TEXT NOT NULL, -- reference name (e.g. panel_0, kibanaSavedObjectMeta.searchSourceJSON)
  ref_type TEXT NOT NULL -- referenced object type (e.g. visualization, search, index-pattern)
);

CREATE TABLE IF NOT EXISTS package_categories (
  -- Categories assigned to a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  category TEXT NOT NULL, -- category value
  package_id INTEGER NOT NULL REFERENCES packages(id) -- foreign key to packages
);

CREATE TABLE IF NOT EXISTS package_fields (
  -- Join table linking fields to packages (for input packages).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  field_id INTEGER NOT NULL REFERENCES fields(id), -- foreign key to fields
  package_id INTEGER NOT NULL REFERENCES packages(id) -- foreign key to packages
);

CREATE TABLE IF NOT EXISTS package_icons (
  -- Icon definitions for a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dark_mode BOOLEAN, -- Is this icon to be shown in dark mode?
  size TEXT, -- Size of the icon.
  src TEXT NOT NULL, -- Relative path to the icon's image file.
  title TEXT, -- Title of icon.
  type TEXT -- MIME type of the icon image file.
);

CREATE TABLE IF NOT EXISTS package_screenshots (
  -- Screenshot definitions for a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  size TEXT, -- Size of the screenshot.
  src TEXT NOT NULL, -- Relative path to the screenshot's image file.
  title TEXT NOT NULL, -- Title of screenshot.
  type TEXT -- MIME type of the screenshot image file.
);

CREATE TABLE IF NOT EXISTS pipeline_tests (
  -- Pipeline test cases for data streams. Each row is one test event file with optional per-case config.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  config_path TEXT, -- path to per-case config file
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  dynamic_fields JSON, -- dynamic fields with regex patterns (from per-case config)
  event_path TEXT NOT NULL, -- path to event file
  expected_path TEXT, -- path to expected output file
  fields JSON, -- field definitions (from per-case config)
  format TEXT NOT NULL, -- event file format (json or raw)
  multiline JSON, -- multi-line configuration (from per-case raw config)
  name TEXT NOT NULL, -- test case stem name (e.g. test-example)
  numeric_keyword_fields JSON, -- keyword fields allowed numeric values (from per-case config)
  skip_link TEXT, -- link to issue for skipped test (from per-case config)
  skip_reason TEXT, -- reason test is skipped (from per-case config)
  string_number_fields JSON -- numeric fields allowed string values (from per-case config)
);

CREATE TABLE IF NOT EXISTS policy_templates (
  -- Policy templates offered by integration and input packages. Defines how a package is configured in Fleet.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dynamic_signal_types BOOLEAN, -- whether transforms and index templates are created based on pipeline config (input packages only)
  input TEXT, -- input type for input packages (e.g. cel, httpjson)
  policy_template_type TEXT, -- data stream type for input packages (logs, metrics, synthetics, traces)
  template_path TEXT, -- path to agent template for input packages (e.g. input.yml.hbs)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  configuration_links JSON, -- JSON-encoded ConfigurationLinks
  data_streams JSON, -- List of data streams compatible with the policy template.
  deployment_modes_agentless_division TEXT, -- The division responsible for the integration. This is used to tag the agentless agent deployments for monitoring.
  deployment_modes_agentless_enabled BOOLEAN, -- Indicates if the agentless deployment mode is available for this template policy. It is disabled by default.
  deployment_modes_agentless_is_default BOOLEAN, -- On policy templates that support multiple deployment modes, this setting can be set to true to use agentless mode by default.
  deployment_modes_agentless_organization TEXT, -- The responsible organization of the integration. This is used to tag the agentless agent deployments for monitoring.
  deployment_modes_agentless_resources_requests_cpu TEXT, -- The amount of CPUs that the Agentless deployment will be initially allocated.
  deployment_modes_agentless_resources_requests_memory TEXT, -- The amount of memory that the Agentless deployment will be initially allocated.
  deployment_modes_agentless_team TEXT, -- The team responsible for the integration. This is used to tag the agentless agent deployments for monitoring.
  deployment_modes_default_enabled BOOLEAN, -- Indicates if the default deployment mode is available for this template policy. It is enabled by default.
  description TEXT NOT NULL, -- Longer description of policy template.
  fips_compatible BOOLEAN, -- FipsCompatible
  multiple BOOLEAN, -- Multiple
  name TEXT NOT NULL, -- Name of policy template.
  title TEXT NOT NULL -- Title of policy template.
);

CREATE TABLE IF NOT EXISTS policy_template_categories (
  -- Categories assigned to a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  category TEXT NOT NULL, -- category value
  policy_template_id INTEGER NOT NULL REFERENCES policy_templates(id) -- foreign key to policy_templates
);

CREATE TABLE IF NOT EXISTS policy_template_icons (
  -- Icon definitions for a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_templates_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  dark_mode BOOLEAN, -- Is this icon to be shown in dark mode?
  size TEXT, -- Size of the icon.
  src TEXT NOT NULL, -- Relative path to the icon's image file.
  title TEXT, -- Title of icon.
  type TEXT -- MIME type of the icon image file.
);

CREATE TABLE IF NOT EXISTS policy_template_inputs (
  -- Inputs defined within a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_templates_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  deployment_modes JSON, -- List of deployment modes that this input is compatible with. If not specified, the input is compatible with all deployment modes.
  description TEXT NOT NULL, -- Longer description of input.
  hide_in_var_group_options JSON, -- HideInVarGroupOptions filters out specific var_group options for this input.
  input_group TEXT, -- Name of the input group
  multi BOOLEAN, -- Can input be defined multiple times
  template_path TEXT, -- Path of the config template for the input.
  title TEXT NOT NULL, -- Title of input.
  type TEXT NOT NULL -- Type of input.
);

CREATE TABLE IF NOT EXISTS policy_template_screenshots (
  -- Screenshot definitions for a policy template.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  policy_templates_id INTEGER NOT NULL REFERENCES policy_templates(id), -- foreign key to policy_templates
  size TEXT, -- Size of the screenshot.
  src TEXT NOT NULL, -- Relative path to the screenshot's image file.
  title TEXT NOT NULL, -- Title of screenshot.
  type TEXT -- MIME type of the screenshot image file.
);

CREATE TABLE IF NOT EXISTS policy_tests (
  -- Policy test cases for data streams and input packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  case_name TEXT NOT NULL, -- test case name extracted from filename
  data_streams_id INTEGER REFERENCES data_streams(id), -- foreign key to data_streams (set for integration packages)
  packages_id INTEGER REFERENCES packages(id), -- foreign key to packages (set for input packages)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  data_stream JSON, -- Configuration for the data stream.
  input TEXT, -- The input of the package to test.
  skip_link TEXT NOT NULL, -- Link to issue with more details about skipped test or to track re-enabling skipped test.
  skip_reason TEXT NOT NULL, -- Short explanation for why test has been skipped.
  vars JSON -- Variables used to configure settings defined in the package manifest.
);

CREATE TABLE IF NOT EXISTS routing_rules (
  -- Routing rules for rerouting documents from a source dataset (technical preview).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  "if" TEXT NOT NULL, -- Conditionally execute the processor
  namespace JSON, -- Namespace is the field reference or static value for the namespace part of the data stream name.
  target_dataset JSON -- TargetDataset is the field reference or static value for the dataset part of the data stream name.
);

CREATE TABLE IF NOT EXISTS sample_events (
  -- Sample event data for data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  event JSON NOT NULL -- sample event data (JSON)
);

CREATE TABLE IF NOT EXISTS static_tests (
  -- Static test cases for data streams.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  case_name TEXT NOT NULL, -- test case name extracted from filename
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  skip_link TEXT NOT NULL, -- Link to issue with more details about skipped test or to track re-enabling skipped test.
  skip_reason TEXT NOT NULL -- Short explanation for why test has been skipped.
);

CREATE TABLE IF NOT EXISTS streams (
  -- Streams offered by a data stream.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  description TEXT NOT NULL, -- Description of the stream. It should describe what is being collected and with what collector, following the structure "Collect X from Y with X".
  enabled BOOLEAN, -- Is stream enabled?
  input TEXT NOT NULL, -- Input
  template_path TEXT, -- Path to Elasticsearch index template for stream.
  title TEXT NOT NULL -- Title of the stream. It should include the source of the data that is being collected, and the kind of data collected such as logs or metrics. Words should be uppercased.
);

CREATE TABLE IF NOT EXISTS system_tests (
  -- System test cases for data streams and input packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  case_name TEXT NOT NULL, -- test case name extracted from filename
  data_streams_id INTEGER REFERENCES data_streams(id), -- foreign key to data_streams (set for integration packages)
  packages_id INTEGER REFERENCES packages(id), -- foreign key to packages (set for input packages)
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  agent_base_image TEXT, -- Elastic Agent image to be used for testing. Setting `default` will be used the same Elastic Agent image as the stack. Setting `systemd` will use the image containing all the binaries for running Be...
  agent_linux_capabilities JSON, -- Linux Capabilities that must been enabled in the system to run the Elastic Agent process
  agent_pid_mode TEXT, -- Control access to PID namespaces. When set to `host`, the Elastic Agent will have access to the PID namespace of the host.
  agent_ports JSON, -- List of ports to be exposed to access to the Elastic Agent
  agent_pre_start_script_contents TEXT NOT NULL, -- Code to run before starting the Elastic Agent.
  agent_pre_start_script_language TEXT, -- Programming language of the pre-start script. Currently, only "sh" is supported.
  agent_provisioning_script_contents TEXT NOT NULL, -- Code to run as a provisioning script.
  agent_provisioning_script_language TEXT, -- Programming language of the provisioning script.
  agent_runtime TEXT, -- Runtime to run the Elastic Agent process
  agent_user TEXT, -- User that runs the Elastic Agent process
  data_stream JSON, -- JSON-encoded DataStream
  skip_link TEXT NOT NULL, -- Link to issue with more details about skipped test or to track re-enabling skipped test.
  skip_reason TEXT NOT NULL, -- Short explanation for why test has been skipped.
  skip_ignored_fields JSON, -- If listed here, elastic-package system tests will not fail if values for the specified field names can't be indexed for any incoming documents. This should only be used if the failure is related to...
  vars JSON, -- Variables used to configure settings defined in the package manifest.
  wait_for_data_timeout TEXT -- Timeout for waiting for metrics data during a system test.
);

CREATE TABLE IF NOT EXISTS tags (
  -- Kibana tags associated with integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  asset_ids JSON, -- Asset IDs where this tag is going to be added. If two or more pacakges define the same tag, there will be just one tag created in Kibana and all the assets will be using the same tag.
  asset_types JSON, -- This tag will be added to all the assets of these types included in the package. If two or more pacakges define the same tag, there will be just one tag created in Kibana and all the assets will be...
  text TEXT -- Tag name.
);

CREATE TABLE IF NOT EXISTS transforms (
  -- Elasticsearch transform configurations within integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the transform
  manifest_destination_index_template JSON, -- Elasticsearch index template for the transform destination (JSON)
  manifest_start BOOLEAN, -- whether to start the transform upon installation
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  meta JSON, -- Meta holds user-defined metadata about the transform.
  description TEXT, -- Description
  dest JSON, -- JSON-encoded Dest
  frequency TEXT, -- Frequency
  latest JSON, -- JSON-encoded Latest
  pivot JSON, -- JSON-encoded Pivot
  retention_policy JSON, -- JSON-encoded RetentionPolicy
  settings JSON, -- JSON-encoded Settings
  source JSON, -- JSON-encoded Source
  sync JSON -- JSON-encoded Sync
);

CREATE TABLE IF NOT EXISTS transform_fields (
  -- Join table linking fields to transforms.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  field_id INTEGER NOT NULL REFERENCES fields(id), -- foreign key to fields
  transform_id INTEGER NOT NULL REFERENCES transforms(id) -- foreign key to transforms
);

CREATE TABLE IF NOT EXISTS vars (
  -- Input variable definitions. Linked to packages, policy templates, streams, or inputs via join tables.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  file_path TEXT, -- source file path
  file_line INTEGER, -- source file line number
  file_column INTEGER, -- source file column number
  "default" JSON, -- Default is the default value for the variable.
  description TEXT, -- Short description of variable.
  hide_in_deployment_modes JSON, -- Whether this variable should be hidden in the UI for agent policies intended to some specific deployment modes.
  max_duration TEXT, -- The maximum allowed duration value for duration data types. This property can only be used when the type is set to 'duration'.
  min_duration TEXT, -- The minimum allowed duration value for duration data types. This property can only be used when the type is set to 'duration'.
  multi BOOLEAN, -- Can variable contain multiple values?
  name TEXT NOT NULL, -- Variable name.
  options JSON, -- Options provides the list of selectable options when type is "select".
  required BOOLEAN, -- Is variable required?
  secret BOOLEAN, -- Specifying that a variable is secret means that Kibana will store the value separate from the package policy in a more secure index. This is useful for passwords and other sensitive information. On...
  show_user BOOLEAN, -- Should this variable be shown to the user by default?
  title TEXT, -- Title of variable.
  type TEXT NOT NULL, -- Data type of variable. A duration type is a sequence of decimal numbers, each with a unit suffix, such as "60s", "1m" or "2h45m". Duration values must follow these rules: - Use time units of "ms", ...
  url_allowed_schemes JSON -- List of allowed URL schemes for the url type. If empty, any scheme is allowed. An empty string can be used to indicate that the scheme is not mandatory.
);

CREATE TABLE IF NOT EXISTS deprecations (
  -- Deprecation notices for packages, policy templates, inputs, data streams, and vars. Each row links to exactly one parent entity via a nullable FK.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER REFERENCES data_streams(id), -- foreign key to data_streams (set when a data stream is deprecated)
  description TEXT NOT NULL, -- reason for deprecation
  packages_id INTEGER REFERENCES packages(id), -- foreign key to packages (set when a package is deprecated)
  policy_template_inputs_id INTEGER REFERENCES policy_template_inputs(id), -- foreign key to policy_template_inputs (set when an input is deprecated)
  policy_templates_id INTEGER REFERENCES policy_templates(id), -- foreign key to policy_templates (set when a policy template is deprecated)
  replaced_by_data_stream TEXT, -- name of the data stream that replaces the deprecated one
  replaced_by_input TEXT, -- name of the input that replaces the deprecated one
  replaced_by_package TEXT, -- name of the package that replaces the deprecated one
  replaced_by_policy_template TEXT, -- name of the policy template that replaces the deprecated one
  replaced_by_variable TEXT, -- name of the variable that replaces the deprecated one
  since TEXT NOT NULL, -- version since when deprecated
  vars_id INTEGER REFERENCES vars(id) -- foreign key to vars (set when a var is deprecated)
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
