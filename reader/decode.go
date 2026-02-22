package reader

import (
	"errors"
	"fmt"
	"io"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// decodeYAML reads a YAML file from fsys and decodes it into v.
func decodeYAML(fsys fs.FS, filePath string, v any, knownFields bool) error {
	f, err := fsys.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening %s: %w", filePath, err)
	}
	defer f.Close()

	dec := yaml.NewDecoder(f)
	if knownFields {
		dec.KnownFields(true)
	}

	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decoding %s: %w", filePath, err)
	}
	return nil
}

// readOptionalYAML reads an optional YAML file. If the file does not exist,
// it returns (nil, nil). On success it returns the decoded value.
func readOptionalYAML[T any](fsys fs.FS, filePath string, knownFields bool) (*T, error) {
	var v T
	err := decodeYAML(fsys, filePath, &v, knownFields)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}
