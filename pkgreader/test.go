package pkgreader

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/andrewkroh/go-package-spec/pkgspec"
)

// DataStreamTests holds test configs from data_stream/<name>/_dev/test/.
type DataStreamTests struct {
	Pipeline []*PipelineTestCase
	System   map[string]*pkgspec.SystemTestConfig // keyed by case name
	Static   map[string]*pkgspec.StaticTestConfig // keyed by case name
	Policy   map[string]*pkgspec.PolicyTestConfig // keyed by case name
}

// PipelineTestCase represents a pipeline test discovered by scanning for event files.
type PipelineTestCase struct {
	Name         string                            // stem, e.g. "test-example"
	Format       string                            // "json" or "raw"
	Config       any                               // *pkgspec.PipelineTestJSONConfig or *pkgspec.PipelineTestRawConfig, nil if absent
	CommonConfig *pkgspec.PipelineTestCommonConfig // shared, nil if absent
	EventPath    string                            // path to event file
	ExpectedPath string                            // path to expected file, empty if absent
	ConfigPath   string                            // path to per-case config, empty if absent
}

// InputPackageTests holds test configs for input packages.
type InputPackageTests struct {
	System map[string]*pkgspec.SystemTestConfig
	Policy map[string]*pkgspec.PolicyTestConfig
}

// readDataStreamTests loads all test configuration from the _dev/test/ directory
// within a data stream.
func readDataStreamTests(fsys fs.FS, dsPath string, cfg *config) (*DataStreamTests, error) {
	testDir := path.Join(dsPath, "_dev", "test")

	// Check if test directory exists.
	_, err := fs.Stat(fsys, testDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking test directory: %w", err)
	}

	tests := &DataStreamTests{}

	// Read pipeline tests.
	pipelineDir := path.Join(testDir, "pipeline")
	pipelineTests, err := readPipelineTests(fsys, pipelineDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline tests: %w", err)
	}
	tests.Pipeline = pipelineTests

	// Read system test configs (test-*-config.yml).
	systemDir := path.Join(testDir, "system")
	systemTests, err := readTestConfigs[pkgspec.SystemTestConfig](fsys, systemDir, cfg, "-config.yml")
	if err != nil {
		return nil, fmt.Errorf("reading system tests: %w", err)
	}
	tests.System = systemTests

	// Read static test configs (test-*-config.yml).
	staticDir := path.Join(testDir, "static")
	staticTests, err := readTestConfigs[pkgspec.StaticTestConfig](fsys, staticDir, cfg, "-config.yml")
	if err != nil {
		return nil, fmt.Errorf("reading static tests: %w", err)
	}
	tests.Static = staticTests

	// Read policy test configs (test-*.yml, note: NOT *-config.yml).
	policyDir := path.Join(testDir, "policy")
	policyTests, err := readTestConfigs[pkgspec.PolicyTestConfig](fsys, policyDir, cfg, ".yml")
	if err != nil {
		return nil, fmt.Errorf("reading policy tests: %w", err)
	}
	tests.Policy = policyTests

	return tests, nil
}

// readTestConfigs reads YAML test configuration files matching the pattern test-*<suffix>
// from the given directory, returning a map keyed by the case name extracted from the filename.
// For example, with suffix "-config.yml", the file "test-default-config.yml" yields case name "default".
// With suffix ".yml", the file "test-default.yml" yields case name "default".
func readTestConfigs[T any](fsys fs.FS, dir string, cfg *config, suffix string) (map[string]*T, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var result map[string]*T
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		// Must match test-*<suffix>.
		if !strings.HasPrefix(name, "test-") || !strings.HasSuffix(name, suffix) {
			continue
		}

		// Extract case name: strip "test-" prefix and suffix.
		caseName := name[len("test-") : len(name)-len(suffix)]
		if caseName == "" {
			continue
		}

		filePath := path.Join(dir, name)
		var v T
		if err := decodeYAML(fsys, filePath, &v, cfg.knownFields); err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		pkgspec.AnnotateFileMetadata(filePath, &v)

		if result == nil {
			result = make(map[string]*T)
		}
		result[caseName] = &v
	}

	return result, nil
}

// readPipelineTests scans for pipeline event files (test-*.json, test-*.log) and
// loads their optional per-case configs and the shared common config.
func readPipelineTests(fsys fs.FS, dir string, cfg *config) ([]*PipelineTestCase, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	// Build a set of filenames for quick lookup.
	fileSet := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			fileSet[entry.Name()] = true
		}
	}

	// Load common config if present.
	var commonConfig *pkgspec.PipelineTestCommonConfig
	commonConfigPath := path.Join(dir, "test-common-config.yml")
	cc, err := readOptionalYAML[pkgspec.PipelineTestCommonConfig](fsys, commonConfigPath, cfg.knownFields)
	if err != nil {
		return nil, fmt.Errorf("reading common config: %w", err)
	}
	if cc != nil {
		pkgspec.AnnotateFileMetadata(commonConfigPath, cc)
		commonConfig = cc
	}

	// Scan for event files.
	var cases []*PipelineTestCase
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		if !strings.HasPrefix(name, "test-") {
			continue
		}

		var format string
		var stem string
		switch {
		case strings.HasSuffix(name, ".json") && !strings.HasSuffix(name, "-config.yml") && !strings.HasSuffix(name, "-expected.json"):
			// Match test-*.json but not test-*-expected.json.
			if strings.HasSuffix(name, "-expected.json") {
				continue
			}
			// Also skip config files like test-*.json-config.yml (they don't end in .json, but
			// just in case future naming). Already handled by suffix check.
			stem = strings.TrimSuffix(name, ".json")
			format = "json"
		case strings.HasSuffix(name, ".log"):
			stem = strings.TrimSuffix(name, ".log")
			format = "raw"
		default:
			continue
		}

		tc := &PipelineTestCase{
			Name:         stem,
			Format:       format,
			CommonConfig: commonConfig,
			EventPath:    path.Join(dir, name),
		}

		// Check for expected file.
		expectedName := stem + "." + formatExtension(format) + "-expected.json"
		if fileSet[expectedName] {
			tc.ExpectedPath = path.Join(dir, expectedName)
		}

		// Check for per-case config file.
		configName := stem + "." + formatExtension(format) + "-config.yml"
		if fileSet[configName] {
			tc.ConfigPath = path.Join(dir, configName)

			switch format {
			case "json":
				var c pkgspec.PipelineTestJSONConfig
				if err := decodeYAML(fsys, tc.ConfigPath, &c, cfg.knownFields); err != nil {
					return nil, fmt.Errorf("reading %s: %w", configName, err)
				}
				pkgspec.AnnotateFileMetadata(tc.ConfigPath, &c)
				tc.Config = &c
			case "raw":
				var c pkgspec.PipelineTestRawConfig
				if err := decodeYAML(fsys, tc.ConfigPath, &c, cfg.knownFields); err != nil {
					return nil, fmt.Errorf("reading %s: %w", configName, err)
				}
				pkgspec.AnnotateFileMetadata(tc.ConfigPath, &c)
				tc.Config = &c
			}
		}

		cases = append(cases, tc)
	}

	return cases, nil
}

// formatExtension returns the file extension used in config/expected filenames
// for a given pipeline test format.
func formatExtension(format string) string {
	switch format {
	case "json":
		return "json"
	case "raw":
		return "log"
	default:
		return format
	}
}

// readInputPackageTests loads test cases from an input package's _dev/test/ directory.
func readInputPackageTests(fsys fs.FS, testDir string, cfg *config) (*InputPackageTests, error) {
	_, err := fs.Stat(fsys, testDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("checking test directory: %w", err)
	}

	tests := &InputPackageTests{}

	systemDir := path.Join(testDir, "system")
	systemTests, err := readTestConfigs[pkgspec.SystemTestConfig](fsys, systemDir, cfg, "-config.yml")
	if err != nil {
		return nil, fmt.Errorf("reading system tests: %w", err)
	}
	tests.System = systemTests

	policyDir := path.Join(testDir, "policy")
	policyTests, err := readTestConfigs[pkgspec.PolicyTestConfig](fsys, policyDir, cfg, ".yml")
	if err != nil {
		return nil, fmt.Errorf("reading policy tests: %w", err)
	}
	tests.Policy = policyTests

	return tests, nil
}
