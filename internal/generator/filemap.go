package generator

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// FileMap assigns Go types to output files.
type FileMap struct {
	Files map[string][]string `yaml:"files"` // filename → list of type names
	// Reverse lookup: type name → filename.
	lookup map[string]string
}

// LoadFileMap reads and parses a filemap.yml file.
func LoadFileMap(path string) (*FileMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading filemap: %w", err)
	}

	var fm FileMap
	if err := yaml.Unmarshal(data, &fm); err != nil {
		return nil, fmt.Errorf("parsing filemap: %w", err)
	}

	fm.buildLookup()
	return &fm, nil
}

// buildLookup creates the reverse lookup table.
func (fm *FileMap) buildLookup() {
	fm.lookup = make(map[string]string)
	for file, types := range fm.Files {
		for _, typeName := range types {
			fm.lookup[typeName] = file
		}
	}
}

// OutputFileFor returns the output file name for a given Go type name.
// If the type is not in the map, it returns the defaultFile.
func (fm *FileMap) OutputFileFor(typeName, defaultFile string) string {
	if fm == nil || fm.lookup == nil {
		return defaultFile
	}
	if file, ok := fm.lookup[typeName]; ok {
		return file
	}
	return defaultFile
}

// AssignOutputFiles sets the OutputFile field on each GoType based on
// the file map. Types not in the map get assigned to "types.go".
func (fm *FileMap) AssignOutputFiles(types map[string]*GoType) {
	for _, goType := range types {
		goType.OutputFile = fm.OutputFileFor(goType.Name, "types.go")
	}
}
