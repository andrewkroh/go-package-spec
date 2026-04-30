package pkgreader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAllPackages(t *testing.T) {
	dir := os.Getenv("INTEGRATIONS_DIR")
	if dir == "" {
		t.Skip("INTEGRATIONS_DIR not set")
	}

	packagesDir := filepath.Join(dir, "packages")
	pkgPaths, err := ListPackages(packagesDir)
	if err != nil {
		t.Fatalf("listing packages: %v", err)
	}

	for _, pkgPath := range pkgPaths {
		rel, err := filepath.Rel(packagesDir, pkgPath)
		if err != nil {
			t.Fatalf("computing relative path: %v", err)
		}

		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			pkg, err := Read(pkgPath, WithKnownFields())
			if err != nil {
				t.Fatalf("reading package: %v", err)
			}

			m := pkg.Manifest()
			if m == nil {
				t.Fatal("Manifest() returned nil")
			}
			if m.Name == "" {
				t.Error("manifest name is empty")
			}
			if len(pkg.Changelog) == 0 {
				t.Error("changelog is empty")
			}
		})
	}
}
