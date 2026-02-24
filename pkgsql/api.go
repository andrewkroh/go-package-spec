package pkgsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/andrewkroh/go-package-spec/pkgreader"
	"github.com/andrewkroh/go-package-spec/pkgspec"
	dbpkg "github.com/andrewkroh/go-package-spec/pkgsql/internal/db"
)

// Option configures the behavior of WritePackages and WritePackage.
type Option func(*writeConfig)

type writeConfig struct {
	ecsLookup func(name string) *pkgspec.ECSFieldDefinition
	docReader DocReader
}

// WithECSLookup provides a callback to resolve external ECS field definitions
// during field flattening. When set, fields with "external: ecs" are enriched
// with the ECS type, description, and pattern before insertion. Without this
// option, external fields are inserted without enrichment.
func WithECSLookup(fn func(name string) *pkgspec.ECSFieldDefinition) Option {
	return func(c *writeConfig) {
		c.ecsLookup = fn
	}
}

// DocReader reads doc file content given a package path and doc-relative path.
// It is called for each doc file to obtain markdown content for the docs table.
type DocReader func(pkgPath, docPath string) ([]byte, error)

// WithDocContent enables loading doc file content during SQL writing.
// The provided DocReader is called for each doc file to obtain content.
// Without this option, the docs table is populated with paths and
// content_type only (content column is NULL).
func WithDocContent(reader DocReader) Option {
	return func(c *writeConfig) { c.docReader = reader }
}

// OSDocReader reads doc content from the OS filesystem by joining pkgPath
// (the package directory) and docPath (the package-relative file path, e.g.
// "docs/README.md") with filepath.Join.
func OSDocReader(pkgPath, docPath string) ([]byte, error) {
	return os.ReadFile(filepath.Join(pkgPath, docPath))
}

// TableSchemas returns the CREATE TABLE statements (and FTS5 virtual table
// statements) for all tables in dependency order. The statements include
// table and column comments inside the body, which are preserved in
// sqlite_master when the tables are created. This makes the database file
// self-documenting.
func TableSchemas() []string {
	return append(creates, ftsSchemas...)
}

// WritePackages creates tables (if not exist) and inserts each package
// within its own transaction. If any package fails, the error includes
// the package name. After all packages are inserted, it rebuilds the
// FTS5 full-text search index.
func WritePackages(ctx context.Context, db *sql.DB, pkgs []*pkgreader.Package, opts ...Option) error {
	// Create all tables (including FTS5 virtual tables).
	for _, ddl := range TableSchemas() {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("creating tables: %w", err)
		}
	}

	for _, pkg := range pkgs {
		if err := WritePackage(ctx, db, pkg, opts...); err != nil {
			name := ""
			if m := pkg.Manifest(); m != nil {
				name = m.Name + "-" + m.Version
			}
			return fmt.Errorf("writing package %s: %w", name, err)
		}
	}

	// Rebuild FTS5 indexes after all inserts.
	if err := RebuildFTS(ctx, db); err != nil {
		return fmt.Errorf("rebuilding FTS indexes: %w", err)
	}

	return nil
}

// WritePackage inserts a single package within a transaction. Tables must
// already exist (call WritePackages, or execute TableSchemas() manually).
// Callers using WritePackage directly must call [RebuildFTS] after all
// inserts are complete to populate the FTS5 search indexes.
func WritePackage(ctx context.Context, db *sql.DB, pkg *pkgreader.Package, opts ...Option) error {
	cfg := &writeConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	sc := newStmtCache(tx)
	defer sc.close()

	q := dbpkg.New(sc)

	if err := writePackage(ctx, q, pkg, cfg); err != nil {
		return err
	}

	return tx.Commit()
}

func writePackage(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, cfg *writeConfig) error {
	m := pkg.Manifest()
	if m == nil {
		return fmt.Errorf("package has no manifest")
	}

	// Derive dir_name from path.
	dirName := path.Base(pkg.Path())

	// Extract type-specific fields for the packages table.
	var (
		conditionsKibanaVersion        sql.NullString
		conditionsElasticSubscription  sql.NullString
		agentPrivilegesRoot            sql.NullBool
		elasticsearchPrivilegesCluster any
		policyTemplatesBehavior        sql.NullString
	)
	switch im := pkg.IntegrationManifest(); {
	case im != nil:
		conditionsKibanaVersion = toNullString(im.Conditions.Kibana.Version)
		conditionsElasticSubscription = toNullString(string(im.Conditions.Elastic.Subscription))
		agentPrivilegesRoot = toNullBool(im.Agent.Privileges.Root)
		elasticsearchPrivilegesCluster = jsonNullString(im.Elasticsearch.Privileges.Cluster)
		policyTemplatesBehavior = toNullString(im.PolicyTemplatesBehavior)
	default:
		if inp := pkg.InputManifest(); inp != nil {
			conditionsKibanaVersion = toNullString(inp.Conditions.Kibana.Version)
			conditionsElasticSubscription = toNullString(string(inp.Conditions.Elastic.Subscription))
			agentPrivilegesRoot = toNullBool(inp.Agent.Privileges.Root)
		} else if cm := pkg.ContentManifest(); cm != nil {
			conditionsKibanaVersion = toNullString(cm.Conditions.Kibana.Version)
			conditionsElasticSubscription = toNullString(string(cm.Conditions.Elastic.Subscription))
		}
	}

	// Insert package.
	pkgID, err := q.InsertPackages(ctx, mapPackagesParams(
		m,
		agentPrivilegesRoot,
		toNullString(pkg.Commit),
		conditionsElasticSubscription,
		conditionsKibanaVersion,
		dirName,
		elasticsearchPrivilegesCluster,
		policyTemplatesBehavior,
	))
	if err != nil {
		return fmt.Errorf("inserting package: %w", err)
	}

	// Insert package deprecation.
	if isDeprecated(m.Deprecated) {
		p := deprecationParams(m.Deprecated)
		p.PackagesID = sql.NullInt64{Int64: pkgID, Valid: true}
		if _, err := q.InsertDeprecations(ctx, p); err != nil {
			return fmt.Errorf("inserting package deprecation: %w", err)
		}
	}

	// Insert categories.
	for _, cat := range m.Categories {
		_, err := q.InsertPackageCategories(ctx, dbpkg.InsertPackageCategoriesParams{
			PackageID: pkgID,
			Category:  string(cat),
		})
		if err != nil {
			return fmt.Errorf("inserting category: %w", err)
		}
	}

	// Insert icons.
	for i := range m.Icons {
		_, err := q.InsertPackageIcons(ctx, mapPackageIconsParams(&m.Icons[i], pkgID))
		if err != nil {
			return fmt.Errorf("inserting icon: %w", err)
		}
	}

	// Insert screenshots.
	for i := range m.Screenshots {
		_, err := q.InsertPackageScreenshots(ctx, mapPackageScreenshotsParams(&m.Screenshots[i], pkgID))
		if err != nil {
			return fmt.Errorf("inserting screenshot: %w", err)
		}
	}

	// Insert changelog.
	for i := range pkg.Changelog {
		cl := &pkg.Changelog[i]
		clID, err := q.InsertChangelogs(ctx, mapChangelogsParams(cl, pkgID))
		if err != nil {
			return fmt.Errorf("inserting changelog: %w", err)
		}
		for j := range cl.Changes {
			_, err := q.InsertChangelogEntries(ctx, mapChangelogEntriesParams(&cl.Changes[j], clID))
			if err != nil {
				return fmt.Errorf("inserting changelog entry: %w", err)
			}
		}
	}

	// Insert tags.
	for i := range pkg.Tags {
		_, err := q.InsertTags(ctx, mapTagsParams(&pkg.Tags[i], pkgID))
		if err != nil {
			return fmt.Errorf("inserting tag: %w", err)
		}
	}

	// Insert images (if image metadata was loaded).
	if err := writeImages(ctx, q, pkg, pkgID); err != nil {
		return err
	}

	// Package type-specific data.
	switch m.Type {
	case pkgspec.ManifestTypeIntegration:
		if err := writeIntegration(ctx, q, pkg, pkgID, cfg); err != nil {
			return err
		}
	case pkgspec.ManifestTypeInput:
		if err := writeInput(ctx, q, pkg, pkgID, cfg); err != nil {
			return err
		}
	case pkgspec.ManifestTypeContent:
		if err := writeContent(ctx, q, pkg, pkgID); err != nil {
			return err
		}
	}

	// Insert docs.
	if err := writeDocs(ctx, q, pkg, pkgID, cfg); err != nil {
		return err
	}

	return nil
}

func writeIntegration(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, pkgID int64, cfg *writeConfig) error {
	im := pkg.IntegrationManifest()
	if im == nil {
		return nil
	}

	// Insert package-level vars.
	if err := writeVars(ctx, q, im.Vars, func(varID int64) error {
		_, err := q.InsertPackageVars(ctx, dbpkg.InsertPackageVarsParams{
			PackageID: pkgID,
			VarID:     varID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting package vars: %w", err)
	}

	// Insert policy templates.
	for i := range im.PolicyTemplates {
		pt := &im.PolicyTemplates[i]
		ptID, err := q.InsertPolicyTemplates(ctx, mapPolicyTemplatesParams(pt, pkgID))
		if err != nil {
			return fmt.Errorf("inserting policy template: %w", err)
		}

		// Insert policy template deprecation.
		if isDeprecated(pt.Deprecated) {
			p := deprecationParams(pt.Deprecated)
			p.PolicyTemplatesID = sql.NullInt64{Int64: ptID, Valid: true}
			if _, err := q.InsertDeprecations(ctx, p); err != nil {
				return fmt.Errorf("inserting policy template deprecation: %w", err)
			}
		}

		// Insert policy template categories.
		for _, cat := range pt.Categories {
			_, err := q.InsertPolicyTemplateCategories(ctx, dbpkg.InsertPolicyTemplateCategoriesParams{
				PolicyTemplateID: ptID,
				Category:         string(cat),
			})
			if err != nil {
				return fmt.Errorf("inserting policy template category: %w", err)
			}
		}

		// Insert policy template icons.
		for j := range pt.Icons {
			_, err := q.InsertPolicyTemplateIcons(ctx, mapPolicyTemplateIconsParams(&pt.Icons[j], ptID))
			if err != nil {
				return fmt.Errorf("inserting policy template icon: %w", err)
			}
		}

		// Insert policy template screenshots.
		for j := range pt.Screenshots {
			_, err := q.InsertPolicyTemplateScreenshots(ctx, mapPolicyTemplateScreenshotsParams(&pt.Screenshots[j], ptID))
			if err != nil {
				return fmt.Errorf("inserting policy template screenshot: %w", err)
			}
		}

		// Insert policy template vars.
		if err := writeVars(ctx, q, pt.Vars, func(varID int64) error {
			_, err := q.InsertPolicyTemplateVars(ctx, dbpkg.InsertPolicyTemplateVarsParams{
				PolicyTemplateID: ptID,
				VarID:            varID,
			})
			return err
		}); err != nil {
			return fmt.Errorf("inserting policy template vars: %w", err)
		}

		// Insert inputs.
		for j := range pt.Inputs {
			inp := &pt.Inputs[j]
			inpID, err := q.InsertPolicyTemplateInputs(ctx, mapPolicyTemplateInputsParams(inp, ptID))
			if err != nil {
				return fmt.Errorf("inserting policy template input: %w", err)
			}

			// Insert input deprecation.
			if isDeprecated(inp.Deprecated) {
				p := deprecationParams(inp.Deprecated)
				p.PolicyTemplateInputsID = sql.NullInt64{Int64: inpID, Valid: true}
				if _, err := q.InsertDeprecations(ctx, p); err != nil {
					return fmt.Errorf("inserting input deprecation: %w", err)
				}
			}

			// Insert input vars.
			if err := writeVars(ctx, q, inp.Vars, func(varID int64) error {
				_, err := q.InsertPolicyTemplateInputVars(ctx, dbpkg.InsertPolicyTemplateInputVarsParams{
					PolicyTemplateInputID: inpID,
					VarID:                 varID,
				})
				return err
			}); err != nil {
				return fmt.Errorf("inserting input vars: %w", err)
			}
		}
	}

	// Insert data streams.
	for dsName, ds := range pkg.DataStreams {
		if err := writeDataStream(ctx, q, dsName, ds, pkgID, cfg); err != nil {
			return fmt.Errorf("data stream %s: %w", dsName, err)
		}
	}

	// Insert transforms.
	for tName, td := range pkg.Transforms {
		tID, err := q.InsertTransforms(ctx, mapTransformsParams(
			&td.Transform,
			pkgID,
			tName,
			jsonNullString(transformManifestDestIndexTemplate(td.Manifest)),
			toNullBool(transformManifestStart(td.Manifest)),
		))
		if err != nil {
			return fmt.Errorf("inserting transform %s: %w", tName, err)
		}

		// Insert transform fields.
		if err := writeFields(ctx, q, td.Fields, cfg, func(fieldID int64) error {
			_, err := q.InsertTransformFields(ctx, dbpkg.InsertTransformFieldsParams{
				TransformID: tID,
				FieldID:     fieldID,
			})
			return err
		}); err != nil {
			return fmt.Errorf("inserting transform %s fields: %w", tName, err)
		}
	}

	// Insert build manifest.
	if pkg.Build != nil {
		_, err := q.InsertBuildManifests(ctx, mapBuildManifestsParams(pkg.Build, pkgID))
		if err != nil {
			return fmt.Errorf("inserting build manifest: %w", err)
		}
	}

	return nil
}

func writeInput(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, pkgID int64, cfg *writeConfig) error {
	im := pkg.InputManifest()
	if im == nil {
		return nil
	}

	// Insert package-level vars.
	if err := writeVars(ctx, q, im.Vars, func(varID int64) error {
		_, err := q.InsertPackageVars(ctx, dbpkg.InsertPackageVarsParams{
			PackageID: pkgID,
			VarID:     varID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting input package vars: %w", err)
	}

	// Insert fields (flattened).
	if err := writeFields(ctx, q, pkg.Fields, cfg, func(fieldID int64) error {
		_, err := q.InsertPackageFields(ctx, dbpkg.InsertPackageFieldsParams{
			PackageID: pkgID,
			FieldID:   fieldID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting input fields: %w", err)
	}

	// Insert input test configs.
	if pkg.InputTests != nil {
		if err := writeInputTests(ctx, q, pkg.InputTests, pkgID); err != nil {
			return fmt.Errorf("inserting input tests: %w", err)
		}
	}

	return nil
}

func writeContent(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, pkgID int64) error {
	cm := pkg.ContentManifest()
	if cm == nil {
		return nil
	}

	// Insert discovery fields.
	for _, df := range cm.Discovery.Fields {
		_, err := q.InsertDiscoveryFields(ctx, dbpkg.InsertDiscoveryFieldsParams{
			PackagesID: pkgID,
			Name:       df.Name,
		})
		if err != nil {
			return fmt.Errorf("inserting discovery field: %w", err)
		}
	}

	return nil
}

func writeDataStream(ctx context.Context, q *dbpkg.Queries, dsName string, ds *pkgreader.DataStream, pkgID int64, cfg *writeConfig) error {
	dsID, err := q.InsertDataStreams(ctx, mapDataStreamsParams(&ds.Manifest, pkgID, dsName))
	if err != nil {
		return fmt.Errorf("inserting data stream: %w", err)
	}

	// Insert data stream deprecation.
	if isDeprecated(ds.Manifest.Deprecated) {
		p := deprecationParams(ds.Manifest.Deprecated)
		p.DataStreamsID = sql.NullInt64{Int64: dsID, Valid: true}
		if _, err := q.InsertDeprecations(ctx, p); err != nil {
			return fmt.Errorf("inserting data stream deprecation: %w", err)
		}
	}

	// Insert sample event.
	if ds.SampleEvent != nil {
		_, err := q.InsertSampleEvents(ctx, dbpkg.InsertSampleEventsParams{
			DataStreamsID: dsID,
			Event:         string(ds.SampleEvent),
		})
		if err != nil {
			return fmt.Errorf("inserting sample event: %w", err)
		}
	}

	// Insert streams.
	for i := range ds.Manifest.Streams {
		stream := &ds.Manifest.Streams[i]
		streamID, err := q.InsertStreams(ctx, mapStreamsParams(stream, dsID))
		if err != nil {
			return fmt.Errorf("inserting stream: %w", err)
		}

		// Insert stream vars.
		if err := writeVars(ctx, q, stream.Vars, func(varID int64) error {
			_, err := q.InsertStreamVars(ctx, dbpkg.InsertStreamVarsParams{
				StreamID: streamID,
				VarID:    varID,
			})
			return err
		}); err != nil {
			return fmt.Errorf("inserting stream vars: %w", err)
		}
	}

	// Insert fields (flattened).
	if err := writeFields(ctx, q, ds.Fields, cfg, func(fieldID int64) error {
		_, err := q.InsertDataStreamFields(ctx, dbpkg.InsertDataStreamFieldsParams{
			DataStreamID: dsID,
			FieldID:      fieldID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting fields: %w", err)
	}

	// Insert ingest pipelines.
	for fileName, pf := range ds.Pipelines {
		pipeID, err := q.InsertIngestPipelines(ctx, mapIngestPipelinesParams(&pf.Pipeline, dsID, fileName))
		if err != nil {
			return fmt.Errorf("inserting pipeline: %w", err)
		}

		// Insert processors (flattened).
		if err := writeProcessors(ctx, q, pf.Pipeline.Processors, pipeID, "/processors"); err != nil {
			return fmt.Errorf("inserting processors: %w", err)
		}
		if err := writeProcessors(ctx, q, pf.Pipeline.OnFailure, pipeID, "/on_failure"); err != nil {
			return fmt.Errorf("inserting on_failure processors: %w", err)
		}
	}

	// Insert routing rules.
	for _, rrs := range ds.RoutingRules {
		for i := range rrs.Rules {
			_, err := q.InsertRoutingRules(ctx, mapRoutingRulesParams(&rrs.Rules[i], dsID))
			if err != nil {
				return fmt.Errorf("inserting routing rule: %w", err)
			}
		}
	}

	// Insert test configs.
	if ds.Tests != nil {
		if err := writeDataStreamTests(ctx, q, ds.Tests, dsID); err != nil {
			return fmt.Errorf("inserting tests: %w", err)
		}
	}

	return nil
}

func writeFields(ctx context.Context, q *dbpkg.Queries, fieldsMap map[string]*pkgreader.FieldsFile, cfg *writeConfig, link func(fieldID int64) error) error {
	if fieldsMap == nil {
		return nil
	}

	// Collect all fields from all files.
	var allFields []pkgspec.Field
	for _, ff := range fieldsMap {
		allFields = append(allFields, ff.Fields...)
	}

	// Flatten fields.
	flat := pkgspec.FlattenFields(allFields, cfg.ecsLookup)

	for i := range flat {
		fieldID, err := q.InsertFields(ctx, mapFieldsParams(&flat[i]))
		if err != nil {
			return fmt.Errorf("inserting field %s: %w", flat[i].Name, err)
		}
		if err := link(fieldID); err != nil {
			return fmt.Errorf("linking field %s: %w", flat[i].Name, err)
		}
	}
	return nil
}

func writeProcessors(ctx context.Context, q *dbpkg.Queries, processors []*pkgspec.Processor, pipeID int64, basePath string) error {
	for i, proc := range processors {
		pointer := fmt.Sprintf("%s/%d/%s", basePath, i, proc.Type)

		// Build attributes including on_failure so each row is self-contained.
		fullAttrs := make(map[string]any, len(proc.Attributes)+1)
		for k, v := range proc.Attributes {
			fullAttrs[k] = v
		}
		if len(proc.OnFailure) > 0 {
			fullAttrs["on_failure"] = proc.OnFailure
		}
		attrs, _ := json.Marshal(fullAttrs)

		var attrsVal any
		if len(attrs) > 2 {
			attrsVal = string(attrs)
		}

		_, err := q.InsertIngestProcessors(ctx, dbpkg.InsertIngestProcessorsParams{
			IngestPipelinesID: pipeID,
			Type:              proc.Type,
			Attributes:        attrsVal,
			JsonPointer:       pointer,
			Ordinal:           int64(i),
			FilePath:          toNullString(proc.FilePath()),
			FileLine:          toNullInt64(proc.Line()),
			FileColumn:        toNullInt64(proc.Column()),
		})
		if err != nil {
			return fmt.Errorf("inserting processor %s: %w", proc.Type, err)
		}

		// Recurse into on_failure processors.
		if len(proc.OnFailure) > 0 {
			onFailurePath := fmt.Sprintf("%s/%d/%s/on_failure", basePath, i, proc.Type)
			if err := writeProcessors(ctx, q, proc.OnFailure, pipeID, onFailurePath); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeImages(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, pkgID int64) error {
	for _, img := range pkg.Images {
		// Store src with leading "/" to match icon/screenshot src fields
		// for easy joins (e.g. images.src = package_icons.src).
		src := "/" + img.Path()

		_, err := q.InsertImages(ctx, dbpkg.InsertImagesParams{
			PackagesID: pkgID,
			Src:        src,
			Width:      toNullInt64(img.Width),
			Height:     toNullInt64(img.Height),
			ByteSize:   img.ByteSize,
			Sha256:     img.SHA256,
		})
		if err != nil {
			return fmt.Errorf("inserting image %s: %w", img.Path(), err)
		}
	}
	return nil
}

func writeDocs(ctx context.Context, q *dbpkg.Queries, pkg *pkgreader.Package, pkgID int64, cfg *writeConfig) error {
	for _, doc := range pkg.Docs {
		var content sql.NullString
		if cfg.docReader != nil {
			data, err := cfg.docReader(pkg.Path(), doc.FSPath())
			if err != nil {
				return fmt.Errorf("reading doc %s: %w", doc.Path(), err)
			}
			content = sql.NullString{String: stripFieldTables(string(data)), Valid: true}
		}
		_, err := q.InsertDocs(ctx, dbpkg.InsertDocsParams{
			PackagesID:  pkgID,
			FilePath:    doc.Path(),
			ContentType: string(doc.ContentType),
			Content:     content,
		})
		if err != nil {
			return fmt.Errorf("inserting doc %s: %w", doc.Path(), err)
		}
	}
	return nil
}

func writeVars(ctx context.Context, q *dbpkg.Queries, vars []pkgspec.Var, link func(varID int64) error) error {
	for i := range vars {
		varID, err := q.InsertVars(ctx, mapVarsParams(&vars[i]))
		if err != nil {
			return fmt.Errorf("inserting var %s: %w", vars[i].Name, err)
		}
		if err := link(varID); err != nil {
			return fmt.Errorf("linking var %s: %w", vars[i].Name, err)
		}

		// Insert var deprecation.
		if isDeprecated(vars[i].Deprecated) {
			p := deprecationParams(vars[i].Deprecated)
			p.VarsID = sql.NullInt64{Int64: varID, Valid: true}
			if _, err := q.InsertDeprecations(ctx, p); err != nil {
				return fmt.Errorf("inserting var %s deprecation: %w", vars[i].Name, err)
			}
		}
	}
	return nil
}

// isDeprecated reports whether a Deprecated struct indicates active deprecation.
func isDeprecated(d pkgspec.Deprecated) bool {
	return d.Since != ""
}

// deprecationParams builds the common dbpkg.InsertDeprecationsParams fields from a
// Deprecated struct. The caller must set the appropriate FK field (PackagesID,
// PolicyTemplatesID, etc.) before passing to InsertDeprecations.
func deprecationParams(d pkgspec.Deprecated) dbpkg.InsertDeprecationsParams {
	return dbpkg.InsertDeprecationsParams{
		Description:              d.Description,
		Since:                    d.Since,
		ReplacedByDataStream:     toNullString(d.ReplacedBy.DataStream),
		ReplacedByInput:          toNullString(d.ReplacedBy.Input),
		ReplacedByPackage:        toNullString(d.ReplacedBy.Package),
		ReplacedByPolicyTemplate: toNullString(d.ReplacedBy.PolicyTemplate),
		ReplacedByVariable:       toNullString(d.ReplacedBy.Variable),
	}
}

func writeDataStreamTests(ctx context.Context, q *dbpkg.Queries, tests *pkgreader.DataStreamTests, dsID int64) error {
	// Insert pipeline tests.
	for _, tc := range tests.Pipeline {
		if err := writePipelineTest(ctx, q, tc, dsID); err != nil {
			return fmt.Errorf("pipeline test %s: %w", tc.Name, err)
		}
	}

	// Insert system tests.
	for caseName, cfg := range tests.System {
		p := mapSystemTestsParams(cfg, caseName)
		p.DataStreamsID = sql.NullInt64{Int64: dsID, Valid: true}
		if _, err := q.InsertSystemTests(ctx, p); err != nil {
			return fmt.Errorf("system test %s: %w", caseName, err)
		}
	}

	// Insert static tests.
	for caseName, cfg := range tests.Static {
		p := mapStaticTestsParams(cfg, caseName)
		p.DataStreamsID = dsID
		if _, err := q.InsertStaticTests(ctx, p); err != nil {
			return fmt.Errorf("static test %s: %w", caseName, err)
		}
	}

	// Insert policy tests.
	for caseName, cfg := range tests.Policy {
		p := mapPolicyTestsParams(cfg, caseName)
		p.DataStreamsID = sql.NullInt64{Int64: dsID, Valid: true}
		if _, err := q.InsertPolicyTests(ctx, p); err != nil {
			return fmt.Errorf("policy test %s: %w", caseName, err)
		}
	}

	return nil
}

func writePipelineTest(ctx context.Context, q *dbpkg.Queries, tc *pkgreader.PipelineTestCase, dsID int64) error {
	p := dbpkg.InsertPipelineTestsParams{
		DataStreamsID: dsID,
		Name:          tc.Name,
		Format:        tc.Format,
		EventPath:     tc.EventPath,
		ExpectedPath:  toNullString(tc.ExpectedPath),
		ConfigPath:    toNullString(tc.ConfigPath),
	}

	// Extract fields from the per-case config if present.
	switch cfg := tc.Config.(type) {
	case *pkgspec.PipelineTestJSONConfig:
		p.SkipLink = toNullString(cfg.Skip.Link)
		p.SkipReason = toNullString(cfg.Skip.Reason)
		p.DynamicFields = jsonNullString(cfg.DynamicFields)
		p.Fields = jsonNullString(cfg.Fields)
		p.NumericKeywordFields = jsonNullString(cfg.NumericKeywordFields)
		p.StringNumberFields = jsonNullString(cfg.StringNumberFields)
	case *pkgspec.PipelineTestRawConfig:
		p.SkipLink = toNullString(cfg.Skip.Link)
		p.SkipReason = toNullString(cfg.Skip.Reason)
		p.DynamicFields = jsonNullString(cfg.DynamicFields)
		p.Fields = jsonNullString(cfg.Fields)
		p.NumericKeywordFields = jsonNullString(cfg.NumericKeywordFields)
		p.StringNumberFields = jsonNullString(cfg.StringNumberFields)
		p.Multiline = jsonNullString(cfg.Multiline)
	}

	_, err := q.InsertPipelineTests(ctx, p)
	return err
}

func writeInputTests(ctx context.Context, q *dbpkg.Queries, tests *pkgreader.InputPackageTests, pkgID int64) error {
	// Insert system tests.
	for caseName, cfg := range tests.System {
		p := mapSystemTestsParams(cfg, caseName)
		p.PackagesID = sql.NullInt64{Int64: pkgID, Valid: true}
		if _, err := q.InsertSystemTests(ctx, p); err != nil {
			return fmt.Errorf("system test %s: %w", caseName, err)
		}
	}

	// Insert policy tests.
	for caseName, cfg := range tests.Policy {
		p := mapPolicyTestsParams(cfg, caseName)
		p.PackagesID = sql.NullInt64{Int64: pkgID, Valid: true}
		if _, err := q.InsertPolicyTests(ctx, p); err != nil {
			return fmt.Errorf("policy test %s: %w", caseName, err)
		}
	}

	return nil
}

// transformManifestStart extracts the Start field from a TransformManifest.
func transformManifestStart(m *pkgspec.TransformManifest) *bool {
	if m == nil {
		return nil
	}
	return m.Start
}

// transformManifestDestIndexTemplate extracts the DestinationIndexTemplate
// for JSON serialization. Returns nil if no manifest or empty template.
func transformManifestDestIndexTemplate(m *pkgspec.TransformManifest) any {
	if m == nil {
		return nil
	}
	return m.DestinationIndexTemplate
}
