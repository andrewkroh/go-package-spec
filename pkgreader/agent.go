package pkgreader

import (
	"io/fs"
	"path"
	"strings"
)

// AgentTemplate represents a single agent Handlebars template file (.yml.hbs).
type AgentTemplate struct {
	Content string // raw Handlebars template content
	path    string
}

// Path returns the file path relative to the package root.
func (t *AgentTemplate) Path() string {
	return t.path
}

// readAgentTemplates reads all .yml.hbs files from the agent directory.
// For integration packages: agent/input/stream/*.yml.hbs
// For input packages: agent/input/*.yml.hbs
// For data streams: agent/stream/*.yml.hbs
func readAgentTemplates(fsys fs.FS, agentDir string) (map[string]*AgentTemplate, error) {
	return readAgentTemplatesFromDir(fsys, agentDir)
}

func readAgentTemplatesFromDir(fsys fs.FS, dir string) (map[string]*AgentTemplate, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var result map[string]*AgentTemplate
	for _, entry := range entries {
		name := entry.Name()
		entryPath := path.Join(dir, name)

		if entry.IsDir() {
			sub, err := readAgentTemplatesFromDir(fsys, entryPath)
			if err != nil {
				return nil, err
			}
			for k, v := range sub {
				if result == nil {
					result = make(map[string]*AgentTemplate)
				}
				result[k] = v
			}
			continue
		}

		if !strings.HasSuffix(name, ".yml.hbs") {
			continue
		}

		data, err := fs.ReadFile(fsys, entryPath)
		if err != nil {
			return nil, err
		}

		if result == nil {
			result = make(map[string]*AgentTemplate)
		}
		result[entryPath] = &AgentTemplate{
			Content: string(data),
			path:    entryPath,
		}
	}

	return result, nil
}
