package pkgreader

import (
	"testing"
	"testing/fstest"
)

func TestReadDocs(t *testing.T) {
	fsys := fstest.MapFS{
		"docs/README.md":                    {Data: []byte("# Test Package\n")},
		"docs/getting-started.md":           {Data: []byte("# Getting Started\n")},
		"docs/knowledge_base/context.md":    {Data: []byte("# Context\n")},
		"docs/knowledge_base/debugging.md":  {Data: []byte("# Debugging\n")},
		"docs/not-a-doc.txt":                {Data: []byte("ignored")},
		"docs/subdir/nested.md":             {Data: []byte("ignored")},
		"docs/knowledge_base/subdir/foo.md": {Data: []byte("ignored")},
	}

	docs, err := readDocs(fsys, ".")
	if err != nil {
		t.Fatalf("readDocs: %v", err)
	}

	if len(docs) != 4 {
		t.Fatalf("expected 4 docs, got %d", len(docs))
	}

	// Build a map for easier assertions.
	byPath := make(map[string]*DocFile, len(docs))
	for _, d := range docs {
		byPath[d.Path()] = d
	}

	// Verify README classification.
	if d, ok := byPath["docs/README.md"]; !ok {
		t.Error("missing docs/README.md")
	} else if d.ContentType != DocContentTypeReadme {
		t.Errorf("expected readme, got %s", d.ContentType)
	}

	// Verify doc classification.
	if d, ok := byPath["docs/getting-started.md"]; !ok {
		t.Error("missing docs/getting-started.md")
	} else if d.ContentType != DocContentTypeDoc {
		t.Errorf("expected doc, got %s", d.ContentType)
	}

	// Verify knowledge_base classification.
	if d, ok := byPath["docs/knowledge_base/context.md"]; !ok {
		t.Error("missing docs/knowledge_base/context.md")
	} else if d.ContentType != DocContentTypeKnowledgeBase {
		t.Errorf("expected knowledge_base, got %s", d.ContentType)
	}

	if d, ok := byPath["docs/knowledge_base/debugging.md"]; !ok {
		t.Error("missing docs/knowledge_base/debugging.md")
	} else if d.ContentType != DocContentTypeKnowledgeBase {
		t.Errorf("expected knowledge_base, got %s", d.ContentType)
	}

	// Verify non-.md files and nested subdirs are excluded.
	if _, ok := byPath["docs/not-a-doc.txt"]; ok {
		t.Error("non-.md file should be excluded")
	}
	if _, ok := byPath["docs/subdir/nested.md"]; ok {
		t.Error("nested subdir files should be excluded (only knowledge_base is recursed)")
	}
	if _, ok := byPath["docs/knowledge_base/subdir/foo.md"]; ok {
		t.Error("knowledge_base subdirs should not be recursed")
	}
}

func TestReadDocsNoDocs(t *testing.T) {
	fsys := fstest.MapFS{
		"manifest.yml": {Data: []byte("name: test\n")},
	}

	docs, err := readDocs(fsys, ".")
	if err != nil {
		t.Fatalf("readDocs: %v", err)
	}
	if docs != nil {
		t.Errorf("expected nil docs when docs/ doesn't exist, got %v", docs)
	}
}
