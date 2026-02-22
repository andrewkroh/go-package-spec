package pkgspec

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// StringOrStrings is a type that accepts either a single string or a list of
// strings in YAML/JSON. It always normalizes to a []string internally: a bare
// string becomes a single-element slice.
type StringOrStrings []string

// UnmarshalYAML implements [yaml.Unmarshaler] for StringOrStrings.
func (s *StringOrStrings) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*s = StringOrStrings{node.Value}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := node.Decode(&list); err != nil {
			return err
		}
		*s = list
		return nil
	default:
		return fmt.Errorf("expected string or list of strings, got YAML kind %d", node.Kind)
	}
}

// UnmarshalJSON implements [json.Unmarshaler] for StringOrStrings.
func (s *StringOrStrings) UnmarshalJSON(data []byte) error {
	// Try string first.
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = StringOrStrings{single}
		return nil
	}
	// Try array of strings.
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("expected string or list of strings: %w", err)
	}
	*s = list
	return nil
}

// MarshalYAML implements [yaml.Marshaler] for StringOrStrings.
// A single-element slice is marshaled as a bare string for round-trip fidelity.
func (s *StringOrStrings) MarshalYAML() (any, error) {
	if len(*s) == 1 {
		return (*s)[0], nil
	}
	return []string(*s), nil
}

// MarshalJSON implements [json.Marshaler] for StringOrStrings.
// A single-element slice is marshaled as a bare string for round-trip fidelity.
func (s *StringOrStrings) MarshalJSON() ([]byte, error) {
	if len(*s) == 1 {
		return json.Marshal((*s)[0])
	}
	return json.Marshal([]string(*s))
}
