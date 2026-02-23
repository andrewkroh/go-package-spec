-- name: InsertFields :one
INSERT INTO fields (
  analyzer,
  copy_to,
  date_format,
  default_metric,
  description,
  dimension,
  doc_values,
  dynamic,
  enabled,
  example,
  expected_values,
  external,
  ignore_above,
  ignore_malformed,
  include_in_parent,
  include_in_root,
  "index",
  inference_id,
  metric_type,
  metrics,
  multi_fields,
  name,
  normalize,
  normalizer,
  null_value,
  object_type,
  object_type_mapping_type,
  path,
  pattern,
  runtime,
  scaling_factor,
  search_analyzer,
  store,
  subobjects,
  type,
  unit,
  value,
  json_pointer
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPackages :one
INSERT INTO packages (
  agent_privileges_root,
  elasticsearch_privileges_cluster,
  policy_templates_behavior,
  dir_name,
  conditions_kibana_version,
  conditions_elastic_subscription,
  deprecated,
  description,
  format_version,
  name,
  owner_github,
  owner_type,
  source_license,
  title,
  type,
  version
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertBuildManifests :one
INSERT INTO build_manifests (
  packages_id,
  dependencies_ecs_import_mappings,
  dependencies_ecs_reference
) VALUES (
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertChangelogs :one
INSERT INTO changelogs (
  packages_id,
  version,
  date
) VALUES (
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertChangelogEntries :one
INSERT INTO changelog_entries (
  changelogs_id,
  description,
  link,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertDataStreams :one
INSERT INTO data_streams (
  packages_id,
  dir_name,
  dataset,
  dataset_is_prefix,
  deprecated,
  elasticsearch_dynamic_dataset,
  elasticsearch_dynamic_namespace,
  elasticsearch_index_mode,
  elasticsearch_index_template,
  elasticsearch_privileges,
  elasticsearch_source_mode,
  hidden,
  ilm_policy,
  "release",
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertDataStreamFields :one
INSERT INTO data_stream_fields (
  data_stream_id,
  field_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertDiscoveryFields :one
INSERT INTO discovery_fields (
  packages_id,
  name
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertImages :one
INSERT INTO images (
  height,
  byte_size,
  sha256,
  packages_id,
  src,
  width
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertIngestPipelines :one
INSERT INTO ingest_pipelines (
  data_streams_id,
  file_name,
  description
) VALUES (
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertIngestProcessors :one
INSERT INTO ingest_processors (
  ingest_pipelines_id,
  type,
  attributes,
  json_pointer,
  ordinal
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPackageCategories :one
INSERT INTO package_categories (
  package_id,
  category
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertPackageFields :one
INSERT INTO package_fields (
  package_id,
  field_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertPackageIcons :one
INSERT INTO package_icons (
  packages_id,
  dark_mode,
  size,
  src,
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPackageScreenshots :one
INSERT INTO package_screenshots (
  packages_id,
  size,
  src,
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplates :one
INSERT INTO policy_templates (
  packages_id,
  configuration_links,
  data_streams,
  deployment_modes,
  deprecated,
  description,
  fips_compatible,
  multiple,
  name,
  title
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateCategories :one
INSERT INTO policy_template_categories (
  policy_template_id,
  category
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateIcons :one
INSERT INTO policy_template_icons (
  policy_templates_id,
  dark_mode,
  size,
  src,
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateInputs :one
INSERT INTO policy_template_inputs (
  policy_templates_id,
  deployment_modes,
  deprecated,
  description,
  hide_in_var_group_options,
  input_group,
  multi,
  template_path,
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateScreenshots :one
INSERT INTO policy_template_screenshots (
  policy_templates_id,
  size,
  src,
  title,
  type
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertRoutingRules :one
INSERT INTO routing_rules (
  data_streams_id,
  "if",
  namespace,
  target_dataset
) VALUES (
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertSampleEvents :one
INSERT INTO sample_events (
  data_streams_id,
  event
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertStreams :one
INSERT INTO streams (
  data_streams_id,
  description,
  enabled,
  input,
  template_path,
  title
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertTags :one
INSERT INTO tags (
  packages_id,
  asset_ids,
  asset_types,
  text
) VALUES (
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertTransforms :one
INSERT INTO transforms (
  packages_id,
  dir_name,
  manifest_start,
  manifest_destination_index_template,
  meta,
  description,
  dest,
  frequency,
  latest,
  pivot,
  retention_policy,
  settings,
  source,
  sync
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertTransformFields :one
INSERT INTO transform_fields (
  transform_id,
  field_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertVars :one
INSERT INTO vars (
  "default",
  description,
  hide_in_deployment_modes,
  max_duration,
  min_duration,
  multi,
  name,
  options,
  required,
  secret,
  show_user,
  title,
  type,
  url_allowed_schemes
) VALUES (
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?,
  ?
) RETURNING id;

-- name: InsertPackageVars :one
INSERT INTO package_vars (
  package_id,
  var_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateInputVars :one
INSERT INTO policy_template_input_vars (
  policy_template_input_id,
  var_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertPolicyTemplateVars :one
INSERT INTO policy_template_vars (
  policy_template_id,
  var_id
) VALUES (
  ?,
  ?
) RETURNING id;

-- name: InsertStreamVars :one
INSERT INTO stream_vars (
  stream_id,
  var_id
) VALUES (
  ?,
  ?
) RETURNING id;
