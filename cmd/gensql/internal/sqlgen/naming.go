package sqlgen

import (
	"strings"
	"unicode"
)

// ToSQLName converts a Go identifier (e.g. "FormatVersion") to a SQL column
// name (e.g. "format_version"). It handles camelCase, known abbreviations
// like ILM and URL, and preserves existing underscores.
func ToSQLName(s string) string {
	words := splitWords(s)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

// splitWords breaks an identifier string into its component words.
// It handles snake_case, kebab-case, dot-separated, and camelCase boundaries.
// Duplicated from cmd/generate/internal/generator/naming.go to avoid cross-package coupling.
func splitWords(s string) []string {
	var words []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
	}

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == '_' || r == '-' || r == '.':
			flush()
		case unicode.IsUpper(r):
			if current.Len() > 0 && i > 0 && unicode.IsLower(runes[i-1]) {
				flush()
			} else if current.Len() > 1 && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				flush()
			}
			current.WriteRune(r)
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return words
}
