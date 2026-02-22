package pkgspec

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// Processor represents a single processor in an ingest pipeline.
// It normalizes the YAML/JSON map format {type: {attributes...}} into
// a struct with Type, Attributes, and OnFailure fields.
type Processor struct {
	Type         string
	Attributes   map[string]any
	OnFailure    []*Processor
	FileMetadata `json:"-" yaml:"-"`
}

// UnmarshalYAML implements [yaml.Unmarshaler] for Processor.
// It decodes the {type: {attributes...}} map format into the struct.
func (p *Processor) UnmarshalYAML(node *yaml.Node) error {
	var procMap map[string]struct {
		Attributes map[string]any `yaml:",inline"`
		OnFailure  []*Processor   `yaml:"on_failure"`
	}
	if err := node.Decode(&procMap); err != nil {
		return err
	}

	for k, v := range procMap {
		p.Type = k
		p.Attributes = v.Attributes
		p.OnFailure = v.OnFailure
		break
	}

	p.FileMetadata.line = node.Line
	p.FileMetadata.column = node.Column

	return nil
}

// MarshalYAML implements [yaml.Marshaler] for Processor.
// It encodes the struct back into the {type: {attributes...}} map format.
func (p *Processor) MarshalYAML() (any, error) {
	return map[string]any{
		p.Type: struct {
			Attributes map[string]any `yaml:",inline"`
			OnFailure  []*Processor   `yaml:"on_failure,omitempty"`
		}{p.Attributes, p.OnFailure},
	}, nil
}

// MarshalJSON implements [json.Marshaler] for Processor.
// It encodes the struct back into the {type: {attributes...}} map format.
func (p *Processor) MarshalJSON() ([]byte, error) {
	properties := make(map[string]any, len(p.Attributes)+1)
	for k, v := range p.Attributes {
		properties[k] = v
	}
	if len(p.OnFailure) > 0 {
		properties["on_failure"] = p.OnFailure
	}
	return json.Marshal(map[string]any{
		p.Type: properties,
	})
}
