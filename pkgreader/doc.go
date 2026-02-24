package pkgreader

import (
	"io/fs"
	"path"
	"strings"
)

// DocContentType classifies the kind of documentation file.
type DocContentType string

const (
	// DocContentTypeReadme is the main README.md file.
	DocContentTypeReadme DocContentType = "readme"

	// DocContentTypeDoc is a documentation file in the docs/ directory.
	DocContentTypeDoc DocContentType = "doc"

	// DocContentTypeKnowledgeBase is a file in the docs/knowledge_base/ directory.
	DocContentTypeKnowledgeBase DocContentType = "knowledge_base"
)

// DocFile represents a documentation file within a package.
type DocFile struct {
	// ContentType classifies the file (readme, doc, or knowledge_base).
	ContentType DocContentType
	path        string // display path (may include WithPathPrefix)
	fsPath      string // original path within the fs.FS (for reading content)
}

// Path returns the file path. When WithPathPrefix is used, this includes
// the prefix (e.g. "packages/aws/docs/README.md").
func (d *DocFile) Path() string { return d.path }

// FSPath returns the path within the package's fs.FS, suitable for reading
// the file content. This is always relative to the package root regardless
// of any WithPathPrefix setting.
func (d *DocFile) FSPath() string { return d.fsPath }

// readDocs discovers markdown documentation files under root/docs/.
// It returns nil, nil if the docs/ directory does not exist.
func readDocs(fsys fs.FS, root string) ([]*DocFile, error) {
	docsDir := path.Join(root, "docs")

	entries, err := fs.ReadDir(fsys, docsDir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var docs []*DocFile
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "knowledge_base" {
				kbDir := path.Join(docsDir, "knowledge_base")
				kbDocs, err := readKnowledgeBaseDocs(fsys, kbDir)
				if err != nil {
					return nil, err
				}
				docs = append(docs, kbDocs...)
			}
			continue
		}
		if !isMarkdown(entry.Name()) {
			continue
		}

		ct := DocContentTypeDoc
		if strings.EqualFold(entry.Name(), "README.md") {
			ct = DocContentTypeReadme
		}

		p := path.Join(docsDir, entry.Name())
		docs = append(docs, &DocFile{
			ContentType: ct,
			path:        p,
			fsPath:      p,
		})
	}

	return docs, nil
}

// readKnowledgeBaseDocs reads markdown files from the docs/knowledge_base/ directory.
func readKnowledgeBaseDocs(fsys fs.FS, dir string) ([]*DocFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if isNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var docs []*DocFile
	for _, entry := range entries {
		if entry.IsDir() || !isMarkdown(entry.Name()) {
			continue
		}
		p := path.Join(dir, entry.Name())
		docs = append(docs, &DocFile{
			ContentType: DocContentTypeKnowledgeBase,
			path:        p,
			fsPath:      p,
		})
	}
	return docs, nil
}

// isMarkdown reports whether the file name has a markdown extension.
func isMarkdown(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".md")
}
