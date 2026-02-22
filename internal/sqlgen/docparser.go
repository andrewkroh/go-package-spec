package sqlgen

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DocMap maps "TypeName.FieldName" to a cleaned doc comment string.
type DocMap map[string]string

// ParseDocComments parses Go source files in the given directories and
// extracts struct field doc comments.
func ParseDocComments(dirs []string) (DocMap, error) {
	docs := make(DocMap)
	fset := token.NewFileSet()

	for _, dir := range dirs {
		pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("parsing Go source in %s: %w", dir, err)
		}

		for _, pkg := range pkgs {
			for _, file := range pkg.Files {
				extractFieldDocs(file, docs)
			}
		}
	}

	return docs, nil
}

// extractFieldDocs walks a single AST file and extracts struct field doc comments.
func extractFieldDocs(file *ast.File, docs DocMap) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			typeName := ts.Name.Name
			for _, field := range st.Fields.List {
				if field.Doc == nil || len(field.Names) == 0 {
					continue
				}

				comment := cleanComment(field.Doc)
				if comment == "" {
					continue
				}

				for _, name := range field.Names {
					if !name.IsExported() {
						continue
					}
					docs[typeName+"."+name.Name] = comment
				}
			}
		}
	}
}

// linkRefRe matches Go doc link reference definition lines like:
//
//	[Some Link]: https://example.com
var linkRefRe = regexp.MustCompile(`(?m)^\[[\w\s-]+\]:\s*https?://\S+\s*$`)

// cleanComment extracts and cleans a doc comment from an *ast.CommentGroup.
func cleanComment(cg *ast.CommentGroup) string {
	text := cg.Text()

	// Remove Go doc link reference lines.
	text = linkRefRe.ReplaceAllString(text, "")

	// Collapse whitespace and trim.
	text = strings.Join(strings.Fields(text), " ")
	text = strings.TrimSpace(text)

	// Truncate to 200 characters.
	if len(text) > 200 {
		text = text[:197] + "..."
	}

	return text
}

// findModuleRoot walks up from startDir to find the directory containing go.mod.
func findModuleRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", startDir)
		}
		dir = parent
	}
}

// readModulePath reads the module path from go.mod in the given directory.
func readModulePath(moduleRoot string) (string, error) {
	f, err := os.Open(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

// SourceDirsFromRegistry returns the absolute source directories for all
// packages referenced in the type registry. It strips the module path prefix
// from each type's PkgPath to determine the relative directory.
func SourceDirsFromRegistry(moduleRoot string) ([]string, error) {
	modulePath, err := readModulePath(moduleRoot)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, pkgPath := range RegisteredPkgPaths() {
		relDir := strings.TrimPrefix(pkgPath, modulePath+"/")
		if relDir == pkgPath {
			// pkgPath doesn't start with the module path — skip.
			continue
		}
		absDir := filepath.Join(moduleRoot, relDir)
		seen[absDir] = true
	}

	dirs := make([]string, 0, len(seen))
	for d := range seen {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs, nil
}
