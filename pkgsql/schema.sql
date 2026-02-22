CREATE TABLE IF NOT EXISTS fields (
  -- Elasticsearch field definitions, flattened from nested YAML into dotted-path names.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  analyzer TEXT, -- Analyzer name of the analyzer to use for indexing. Unless search_analyzer is specified this analyzer is used for both indexing and searching. Only valid for 'type: text'.
  copy_to TEXT, -- CopyTo the copy_to parameter allows you to copy the values of multiple fields into a group field, which can then be queried as a single field.
  date_format TEXT, -- DateFormat the date format(s) that can be parsed. Type date format default to `strict_date_optional_time||epoch_millis`, see the [doc]. In JSON documents, dates are represented as strings. Elastics...
  default_metric TEXT, -- JSON-encoded DefaultMetric
  description TEXT, -- Description short description of field
  dimension BOOLEAN, -- Dimension declare a field as dimension of time series. This is attached to the field as a `time_series_dimension` mapping parameter.
  doc_values BOOLEAN, -- DocValues controls whether doc values are enabled for a field. All fields which support doc values have them enabled by default. If you are sure that you don’t need to sort or aggregate on a fiel...
  dynamic TEXT, -- Dynamic controls whether new fields are added dynamically. Accepts true, false, "strict", or "runtime".
  enabled BOOLEAN, -- Enabled the enabled setting, which can be applied only to the top-level mapping definition and to object fields, causes Elasticsearch to skip parsing of the contents of the field entirely. The JSON...
  example TEXT, -- Example values for this field.
  expected_values TEXT, -- ExpectedValues an array of expected values for the field. When defined, these are the only expected values.
  external TEXT, -- External source reference
  ignore_above INTEGER, -- IgnoreAbove strings longer than the ignore_above setting will not be indexed or stored. For arrays of strings, ignore_above will be applied for each array element separately and string elements lon...
  ignore_malformed BOOLEAN, -- IgnoreMalformed trying to index the wrong data type into a field throws an exception by default, and rejects the whole document. The ignore_malformed parameter, if set to true, allows the exception...
  include_in_parent BOOLEAN, -- IncludeInParent for nested field types, this specifies if all fields in the nested object are also added to the parent document as standard (flat) fields.
  include_in_root BOOLEAN, -- IncludeInRoot for nested field types, this specifies if all fields in the nested object are also added to the root document as standard (flat) fields.
  "index" BOOLEAN, -- Index the index option controls whether field values are indexed. Fields that are not indexed are typically not queryable.
  inference_id TEXT, -- InferenceID for semantic_text fields, this specifies the id of the inference endpoint associated with the field
  metric_type TEXT, -- MetricType the metric type of a numeric field. This is attached to the field as a `time_series_metric` mapping parameter. A gauge is a single-value measurement that can go up or down over time, suc...
  metrics TEXT, -- JSON-encoded Metrics
  name TEXT NOT NULL, -- Name of field. Names containing dots are automatically split into sub-fields. Names with wildcards generate dynamic mappings.
  normalize TEXT, -- Normalize specifies the expected normalizations for a field. `array` normalization implies that the values in the field should always be an array, even if they are single values.
  normalizer TEXT, -- Normalizer specifies the name of a normalizer to apply to keyword fields. A simple normalizer called lowercase ships with elasticsearch and can be used. Custom normalizers can be defined as part of...
  null_value TEXT, -- NullValue the null_value parameter allows you to replace explicit null values with the specified value so that it can be indexed and searched. A null value cannot be indexed or searched. When a fie...
  object_type TEXT, -- ObjectType type of the members of the object when `type: object` is used. In these cases a dynamic template is created so direct subobjects of this field have the type indicated. When `object_type_...
  object_type_mapping_type TEXT, -- ObjectTypeMappingType type that members of a field of with `type: object` must have in the source document. This type corresponds to the data type detected by the JSON parser, and is translated to ...
  path TEXT, -- Path for alias type fields this is the path to the target field. Note that this must be the full path, including any parent objects (e.g. object1.object2.field).
  pattern TEXT, -- Pattern regular expression pattern matching the allowed values for the field. This is used for development-time data validation.
  runtime TEXT, -- Runtime specifies if this field is evaluated at query time. Can be a boolean or a script string.
  scaling_factor INTEGER, -- ScalingFactor the scaling factor to use when encoding values. Values will be multiplied by this factor at index time and rounded to the closest long value. For instance, a scaled_float with a scali...
  search_analyzer TEXT, -- SearchAnalyzer name of the analyzer to use for searching. Only valid for 'type: text'.
  store BOOLEAN, -- Store by default, field values are indexed, but not stored. This means that the field can be queried, but the original field cannot be retrieved. Setting this value to true ensures that the field i...
  subobjects BOOLEAN, -- Subobjects specifies if field names containing dots should be expanded into subobjects. For example, if this is set to `true`, a field named `foo.bar` will be expanded into an object with a field n...
  type TEXT, -- Type datatype of field. If the type is set to object, a dynamic mapping is created. In this case, if the name doesn't contain any wildcard, the wildcard is added as the last segment of the path.
  unit TEXT, -- Unit type to associate with a numeric field. This is attached to the field as metadata (via `meta`). By default, a field does not have a unit. The convention for percents is to use value 1 to mean ...
  value TEXT -- Value the value to associate with a constant_keyword field.
);

CREATE TABLE IF NOT EXISTS packages (
  -- Fleet packages (integration, input, or content). Each row is one package version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  dir_name TEXT NOT NULL UNIQUE, -- directory name of the package
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description
  format_version TEXT NOT NULL, -- FormatVersion the version of the package specification format used by this package.
  name TEXT NOT NULL, -- Name the name of the package.
  owner_github TEXT NOT NULL, -- Github team name of the package maintainer.
  owner_type TEXT NOT NULL, -- Type describes who owns the package and the level of support that is provided. The 'elastic' value indicates that the package is built and maintained by Elastic. The 'partner' value indicates that ...
  source_license TEXT, -- License identifier of the license of the package, as specified in https://spdx.org/licenses/.
  title TEXT NOT NULL, -- Title
  type TEXT NOT NULL, -- Type the type of package.
  version TEXT NOT NULL -- Version the version of the package.
);

CREATE TABLE IF NOT EXISTS build_manifests (
  -- Build configuration for integration packages (_dev/build/build.yml).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dependencies_ecs_import_mappings BOOLEAN, -- ImportMappings whether or not import common used dynamic templates and properties into the package
  dependencies_ecs_reference TEXT NOT NULL -- Reference is the ECS version source reference. Values begin with "git@" (e.g. "git@v8.11.0").
);

CREATE TABLE IF NOT EXISTS changelogs (
  -- Changelog versions for a package. Each row is one version entry with its release date.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  version TEXT NOT NULL, -- Version package version.
  date TEXT -- Date is the approximate release date, populated via git blame when WithGitMetadata is used.
);

CREATE TABLE IF NOT EXISTS changelog_entries (
  -- Individual changelog entries within a changelog version.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  changelogs_id INTEGER NOT NULL REFERENCES changelogs(id), -- foreign key to changelogs
  description TEXT NOT NULL, -- Description of change.
  link TEXT NOT NULL, -- Link to issue or PR describing change in detail.
  type TEXT NOT NULL -- Type of change.
);

CREATE TABLE IF NOT EXISTS data_streams (
  -- Data streams within integration packages. Each row is one data stream with its Elasticsearch and agent config.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the data stream
  dataset TEXT, -- Dataset name of data set.
  dataset_is_prefix BOOLEAN, -- DatasetIsPrefix if true, the index pattern in the ES template will contain the dataset as a prefix only
  deprecated TEXT, -- JSON-encoded Deprecated
  elasticsearch_dynamic_dataset BOOLEAN, -- DynamicDataset when set to true, agents running this integration are granted data stream privileges for all datasets of its type
  elasticsearch_dynamic_namespace BOOLEAN, -- DynamicNamespace when set to true, agents running this integration are granted data stream privileges for all namespaces of its type
  elasticsearch_index_mode TEXT, -- Elasticsearch.IndexMode
  elasticsearch_index_template TEXT, -- JSON-encoded IndexTemplate
  elasticsearch_privileges TEXT, -- Privileges elasticsearch privilege requirements
  elasticsearch_source_mode TEXT, -- SourceMode source mode to use. This configures how the document source (`_source`) is stored for this data stream. If configured as `default`, this mode is not configured and it uses Elasticsearch ...
  hidden BOOLEAN, -- Hidden specifies if a data stream is hidden, resulting in dot prefixed system indices. To set the data stream hidden without those dot prefixed indices, check `elasticsearch.index_template.data_str...
  ilm_policy TEXT, -- ILMPolicy the name of an existing ILM (Index Lifecycle Management) policy
  "release" TEXT, -- Release stability of data stream.
  title TEXT NOT NULL, -- Title of data stream. It should include the source of the data that is being collected, and the kind of data collected such as logs or metrics. Words should be uppercased.
  type TEXT -- Type of data stream
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
  description TEXT -- Description of the pipeline.
);

CREATE TABLE IF NOT EXISTS ingest_processors (
  -- Individual ingest processors flattened from pipelines. Nested on_failure handlers are included as separate rows.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  ingest_pipelines_id INTEGER NOT NULL REFERENCES ingest_pipelines(id), -- foreign key to ingest_pipelines
  type TEXT NOT NULL, -- processor type (e.g. set, grok, rename)
  attributes TEXT, -- JSON-encoded processor attributes
  json_pointer TEXT NOT NULL, -- RFC 6901 JSON Pointer location within the pipeline
  ordinal INTEGER NOT NULL -- order of processor within the pipeline
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
  dark_mode BOOLEAN, -- DarkMode is this icon to be shown in dark mode?
  size TEXT, -- Size of the icon.
  src TEXT NOT NULL, -- Src relative path to the icon's image file.
  title TEXT, -- Title of icon.
  type TEXT -- Type MIME type of the icon image file.
);

CREATE TABLE IF NOT EXISTS package_screenshots (
  -- Screenshot definitions for a package.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  size TEXT, -- Size of the screenshot.
  src TEXT NOT NULL, -- Src relative path to the screenshot's image file.
  title TEXT NOT NULL, -- Title of screenshot.
  type TEXT -- Type MIME type of the screenshot image file.
);

CREATE TABLE IF NOT EXISTS policy_templates (
  -- Policy templates offered by integration packages. Defines how an integration is configured in Fleet.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  configuration_links TEXT, -- JSON-encoded ConfigurationLinks
  data_streams TEXT, -- DataStreams list of data streams compatible with the policy template.
  deployment_modes TEXT, -- JSON-encoded DeploymentModes
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description longer description of policy template.
  fips_compatible BOOLEAN, -- FipsCompatible
  multiple BOOLEAN, -- Multiple
  name TEXT NOT NULL, -- Name of policy template.
  title TEXT NOT NULL -- Title of policy template.
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
  deployment_modes TEXT, -- DeploymentModes list of deployment modes that this input is compatible with. If not specified, the input is compatible with all deployment modes.
  deprecated TEXT, -- JSON-encoded Deprecated
  description TEXT NOT NULL, -- Description longer description of input.
  hide_in_var_group_options TEXT, -- HideInVarGroupOptions filters out specific var_group options for this input.
  input_group TEXT, -- InputGroup name of the input group
  multi BOOLEAN, -- Multi can input be defined multiple times
  template_path TEXT, -- TemplatePath path of the config template for the input.
  title TEXT NOT NULL, -- Title of input.
  type TEXT NOT NULL -- Type of input.
);

CREATE TABLE IF NOT EXISTS routing_rules (
  -- Routing rules for rerouting documents from a source dataset (technical preview).
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  "if" TEXT NOT NULL, -- If conditionally execute the processor
  namespace TEXT, -- Namespace is the field reference or static value for the namespace part of the data stream name.
  target_dataset TEXT -- TargetDataset is the field reference or static value for the dataset part of the data stream name.
);

CREATE TABLE IF NOT EXISTS streams (
  -- Streams offered by a data stream.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  data_streams_id INTEGER NOT NULL REFERENCES data_streams(id), -- foreign key to data_streams
  description TEXT NOT NULL, -- Description of the stream. It should describe what is being collected and with what collector, following the structure "Collect X from Y with X".
  enabled BOOLEAN, -- Enabled is stream enabled?
  input TEXT NOT NULL, -- Input
  template_path TEXT, -- TemplatePath path to Elasticsearch index template for stream.
  title TEXT NOT NULL -- Title of the stream. It should include the source of the data that is being collected, and the kind of data collected such as logs or metrics. Words should be uppercased.
);

CREATE TABLE IF NOT EXISTS tags (
  -- Kibana tags associated with integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  asset_ids TEXT, -- AssetIDs asset IDs where this tag is going to be added. If two or more pacakges define the same tag, there will be just one tag created in Kibana and all the assets will be using the same tag.
  asset_types TEXT, -- AssetTypes this tag will be added to all the assets of these types included in the package. If two or more pacakges define the same tag, there will be just one tag created in Kibana and all the ass...
  text TEXT -- Text tag name.
);

CREATE TABLE IF NOT EXISTS transforms (
  -- Elasticsearch transform configurations within integration packages.
  id INTEGER PRIMARY KEY AUTOINCREMENT, -- unique identifier
  packages_id INTEGER NOT NULL REFERENCES packages(id), -- foreign key to packages
  dir_name TEXT NOT NULL, -- directory name of the transform
  meta TEXT, -- Meta holds user-defined metadata about the transform.
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
  "default" TEXT, -- Default is the default value for the variable.
  description TEXT, -- Description short description of variable.
  hide_in_deployment_modes TEXT, -- HideInDeploymentModes whether this variable should be hidden in the UI for agent policies intended to some specific deployment modes.
  max_duration TEXT, -- MaxDuration the maximum allowed duration value for duration data types. This property can only be used when the type is set to 'duration'.
  min_duration TEXT, -- MinDuration the minimum allowed duration value for duration data types. This property can only be used when the type is set to 'duration'.
  multi BOOLEAN, -- Multi can variable contain multiple values?
  name TEXT NOT NULL, -- Name variable name.
  options TEXT, -- Options provides the list of selectable options when type is "select".
  required BOOLEAN, -- Required is variable required?
  secret BOOLEAN, -- Secret specifying that a variable is secret means that Kibana will store the value separate from the package policy in a more secure index. This is useful for passwords and other sensitive informat...
  show_user BOOLEAN, -- ShowUser should this variable be shown to the user by default?
  title TEXT, -- Title of variable.
  type TEXT NOT NULL, -- Type data type of variable. A duration type is a sequence of decimal numbers, each with a unit suffix, such as "60s", "1m" or "2h45m". Duration values must follow these rules: - Use time units of "...
  url_allowed_schemes TEXT -- URLAllowedSchemes list of allowed URL schemes for the url type. If empty, any scheme is allowed. An empty string can be used to indicate that the scheme is not mandatory.
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
  var_id INTEGER NOT NULL REFERENCES vars(id), -- foreign key to vars
  policy_template_input_id INTEGER NOT NULL REFERENCES policy_template_inputs(id) -- foreign key to policy_template_inputs
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
