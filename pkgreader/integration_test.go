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
	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		t.Fatalf("reading packages directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			t.Parallel()

			pkgPath := filepath.Join(packagesDir, entry.Name())
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
