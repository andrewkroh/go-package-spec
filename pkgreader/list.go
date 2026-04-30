package pkgreader

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
)

// ListPackages walks dir top-down and returns the OS paths of all valid
// packages found. A directory qualifies as a package when it contains a
// manifest.yml whose format_version, name, type, and version fields are all
// non-empty. The walk does not descend into a directory once it has been
// identified as a package, since packages never nest.
//
// This algorithm matches the canonical implementation in
// elastic/integrations' dev/citools.ListPackages and supports both the flat
// layout (packages/<name>/) and the nested layout (packages/<group>/<name>/).
//
// Stray or partial manifest.yml files at higher directory levels do not mask
// valid packages below them: if a manifest is invalid, the walk continues
// into its subdirectories.
func ListPackages(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		manifestPath := filepath.Join(p, "manifest.yml")
		if _, err := os.Stat(manifestPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("stat %s: %w", manifestPath, err)
		}
		valid, err := isValidPackageManifest(os.DirFS(p), "manifest.yml")
		if err != nil {
			return fmt.Errorf("validating %s: %w", manifestPath, err)
		}
		if !valid {
			return nil
		}
		paths = append(paths, p)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("listing packages in %s: %w", dir, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// ListPackagesFS is the [io/fs.FS] equivalent of [ListPackages]. It walks
// fsys starting at dir and returns forward-slash paths (relative to fsys)
// for every valid package directory. Use [ListPackages] when working with
// the OS filesystem.
func ListPackagesFS(fsys fs.FS, dir string) ([]string, error) {
	var paths []string
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		manifestPath := path.Join(p, "manifest.yml")
		if _, err := fs.Stat(fsys, manifestPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("stat %s: %w", manifestPath, err)
		}
		valid, err := isValidPackageManifest(fsys, manifestPath)
		if err != nil {
			return fmt.Errorf("validating %s: %w", manifestPath, err)
		}
		if !valid {
			return nil
		}
		paths = append(paths, p)
		return fs.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("listing packages in %s: %w", dir, err)
	}
	sort.Strings(paths)
	return paths, nil
}

// packageManifestValidator captures the fields required to consider a
// manifest a valid package, mirroring elastic/integrations'
// dev/citools.packageManifest.IsValid().
type packageManifestValidator struct {
	FormatVersion string `yaml:"format_version"`
	Name          string `yaml:"name"`
	Type          string `yaml:"type"`
	Version       string `yaml:"version"`
}

func (m *packageManifestValidator) isValid() bool {
	return m.FormatVersion != "" && m.Name != "" && m.Type != "" && m.Version != ""
}

func isValidPackageManifest(fsys fs.FS, manifestPath string) (bool, error) {
	var m packageManifestValidator
	if err := decodeYAML(fsys, manifestPath, &m, false); err != nil {
		return false, err
	}
	return m.isValid(), nil
}
