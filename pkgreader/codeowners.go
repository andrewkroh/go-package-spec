package pkgreader

import (
	"bufio"
	"os"
	"strings"
	"sync"
)

// codeownersRule represents a single CODEOWNERS rule.
type codeownersRule struct {
	pattern string
	owners  []string
}

// codeownersFile holds all parsed CODEOWNERS rules.
type codeownersFile struct {
	rules []codeownersRule
}

// codeownersCache avoids re-parsing the same CODEOWNERS file when Read()
// is called concurrently for many packages in the same repository.
var codeownersCache sync.Map // path → *codeownersFile

// loadCodeowners parses a CODEOWNERS file, caching the result by path.
func loadCodeowners(path string) (*codeownersFile, error) {
	if v, ok := codeownersCache.Load(path); ok {
		return v.(*codeownersFile), nil
	}

	cf, err := parseCodeowners(path)
	if err != nil {
		return nil, err
	}
	codeownersCache.Store(path, cf)
	return cf, nil
}

// parseCodeowners reads and parses a CODEOWNERS file.
func parseCodeowners(path string) (*codeownersFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rules []codeownersRule
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		pattern := fields[0]
		owners := make([]string, 0, len(fields)-1)
		for _, o := range fields[1:] {
			// Strip leading @ from owner names.
			owners = append(owners, strings.TrimPrefix(o, "@"))
		}

		rules = append(rules, codeownersRule{
			pattern: pattern,
			owners:  owners,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &codeownersFile{rules: rules}, nil
}

// matchOwner returns the first owner from the last matching rule for the
// given file path, or an empty string if no rule matches.
//
// CODEOWNERS uses last-match-wins semantics. Pattern matching covers
// simple path prefixes used in the integrations CODEOWNERS file
// (e.g. "/packages/aws" matches "/packages/aws/data_stream/cloudtrail").
func (cf *codeownersFile) matchOwner(filePath string) string {
	var lastOwner string
	for _, rule := range cf.rules {
		if matchPattern(rule.pattern, filePath) && len(rule.owners) > 0 {
			lastOwner = rule.owners[0]
		}
	}
	return lastOwner
}

// matchPattern checks if filePath matches a CODEOWNERS pattern.
// A pattern matches if it equals the path or is a prefix followed by "/".
func matchPattern(pattern, filePath string) bool {
	if filePath == pattern {
		return true
	}
	// Ensure the pattern acts as a directory prefix.
	p := pattern
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return strings.HasPrefix(filePath, p)
}
