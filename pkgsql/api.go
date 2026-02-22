package pkgsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path"

	"github.com/andrewkroh/go-package-spec/pkgreader"
	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// Option configures the behavior of WritePackages and WritePackage.
type Option func(*writeConfig)

type writeConfig struct {
	ecsLookup func(name string) *pkgspec.ECSFieldDefinition
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

// TableSchemas returns the CREATE TABLE statements for all tables in
// dependency order. The statements include table and column comments
// inside the body, which are preserved in sqlite_master when the tables
// are created. This makes the database file self-documenting.
func TableSchemas() []string {
	return Creates
}

// WritePackages creates tables (if not exist) and inserts each package
// within its own transaction. If any package fails, the error includes
// the package name.
func WritePackages(ctx context.Context, db *sql.DB, pkgs []*pkgreader.Package, opts ...Option) error {
	// Create all tables.
	for _, ddl := range Creates {
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
	return nil
}

// WritePackage inserts a single package within a transaction. Tables must
// already exist (call WritePackages, or execute TableSchemas() manually).
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

	q := New(tx)

	if err := writePackage(ctx, q, pkg, cfg); err != nil {
		return err
	}

	return tx.Commit()
}

func writePackage(ctx context.Context, q *Queries, pkg *pkgreader.Package, cfg *writeConfig) error {
	m := pkg.Manifest()
	if m == nil {
		return fmt.Errorf("package has no manifest")
	}

	// Derive dir_name from path.
	dirName := path.Base(pkg.Path())

	// Insert package.
	pkgID, err := q.InsertPackages(ctx, mapPackagesParams(m, dirName))
	if err != nil {
		return fmt.Errorf("inserting package: %w", err)
	}

	// Insert categories.
	for _, cat := range m.Categories {
		_, err := q.InsertPackageCategories(ctx, InsertPackageCategoriesParams{
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
	}

	return nil
}

func writeIntegration(ctx context.Context, q *Queries, pkg *pkgreader.Package, pkgID int64, cfg *writeConfig) error {
	im := pkg.IntegrationManifest()
	if im == nil {
		return nil
	}

	// Insert package-level vars.
	if err := writeVars(ctx, q, im.Vars, func(varID int64) error {
		_, err := q.InsertPackageVars(ctx, InsertPackageVarsParams{
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

		// Insert policy template categories.
		for _, cat := range pt.Categories {
			_, err := q.InsertPolicyTemplateCategories(ctx, InsertPolicyTemplateCategoriesParams{
				PolicyTemplateID: ptID,
				Category:         string(cat),
			})
			if err != nil {
				return fmt.Errorf("inserting policy template category: %w", err)
			}
		}

		// Insert policy template vars.
		if err := writeVars(ctx, q, pt.Vars, func(varID int64) error {
			_, err := q.InsertPolicyTemplateVars(ctx, InsertPolicyTemplateVarsParams{
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

			// Insert input vars.
			if err := writeVars(ctx, q, inp.Vars, func(varID int64) error {
				_, err := q.InsertPolicyTemplateInputVars(ctx, InsertPolicyTemplateInputVarsParams{
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
		_, err := q.InsertTransforms(ctx, mapTransformsParams(&td.Transform, pkgID, tName))
		if err != nil {
			return fmt.Errorf("inserting transform %s: %w", tName, err)
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

func writeInput(ctx context.Context, q *Queries, pkg *pkgreader.Package, pkgID int64, cfg *writeConfig) error {
	im := pkg.InputManifest()
	if im == nil {
		return nil
	}

	// Insert package-level vars.
	if err := writeVars(ctx, q, im.Vars, func(varID int64) error {
		_, err := q.InsertPackageVars(ctx, InsertPackageVarsParams{
			PackageID: pkgID,
			VarID:     varID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting input package vars: %w", err)
	}

	// Insert fields (flattened).
	if err := writeFields(ctx, q, pkg.Fields, cfg, func(fieldID int64) error {
		_, err := q.InsertPackageFields(ctx, InsertPackageFieldsParams{
			PackageID: pkgID,
			FieldID:   fieldID,
		})
		return err
	}); err != nil {
		return fmt.Errorf("inserting input fields: %w", err)
	}

	return nil
}

func writeDataStream(ctx context.Context, q *Queries, dsName string, ds *pkgreader.DataStream, pkgID int64, cfg *writeConfig) error {
	dsID, err := q.InsertDataStreams(ctx, mapDataStreamsParams(&ds.Manifest, pkgID, dsName))
	if err != nil {
		return fmt.Errorf("inserting data stream: %w", err)
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
			_, err := q.InsertStreamVars(ctx, InsertStreamVarsParams{
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
		_, err := q.InsertDataStreamFields(ctx, InsertDataStreamFieldsParams{
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

	return nil
}

func writeFields(ctx context.Context, q *Queries, fieldsMap map[string]*pkgreader.FieldsFile, cfg *writeConfig, link func(fieldID int64) error) error {
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

func writeProcessors(ctx context.Context, q *Queries, processors []*pkgspec.Processor, pipeID int64, basePath string) error {
	for i, proc := range processors {
		pointer := fmt.Sprintf("%s/%d/%s", basePath, i, proc.Type)
		attrs, _ := json.Marshal(proc.Attributes)

		_, err := q.InsertIngestProcessors(ctx, InsertIngestProcessorsParams{
			IngestPipelinesID: pipeID,
			Type:              proc.Type,
			Attributes:        sql.NullString{String: string(attrs), Valid: len(attrs) > 2},
			JsonPointer:       pointer,
			Ordinal:           int64(i),
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

func writeVars(ctx context.Context, q *Queries, vars []pkgspec.Var, link func(varID int64) error) error {
	for i := range vars {
		varID, err := q.InsertVars(ctx, mapVarsParams(&vars[i]))
		if err != nil {
			return fmt.Errorf("inserting var %s: %w", vars[i].Name, err)
		}
		if err := link(varID); err != nil {
			return fmt.Errorf("linking var %s: %w", vars[i].Name, err)
		}
	}
	return nil
}
