package generator

import (
	"strings"
	"unicode"
)

// knownAbbreviations maps lowercase abbreviations to their Go-conventional
// uppercase forms. When a word segment matches one of these entries during
// identifier construction the uppercase form is used instead.
var knownAbbreviations = map[string]string{
	"id":    "ID",
	"ids":   "IDs",
	"url":   "URL",
	"urls":  "URLs",
	"uri":   "URI",
	"cpu":   "CPU",
	"ilm":   "ILM",
	"ip":    "IP",
	"api":   "API",
	"ssl":   "SSL",
	"tls":   "TLS",
	"http":  "HTTP",
	"https": "HTTPS",
	"ecs":   "ECS",
	"ui":    "UI",
	"svg":   "SVG",
	"json":  "JSON",
	"yaml":  "YAML",
	"xml":   "XML",
	"csv":   "CSV",
	"html":  "HTML",
	"css":   "CSS",
	"sql":   "SQL",
	"tcp":   "TCP",
	"udp":   "UDP",
	"dns":   "DNS",
	"ssh":   "SSH",
	"vm":    "VM",
	"os":    "OS",
	"ca":    "CA",
	"ttl":   "TTL",
	"hbs":   "HBS",
}

// ToGoName converts a JSON property name (e.g. "format_version") into
// an exported Go identifier (e.g. "FormatVersion"). It handles snake_case,
// kebab-case, dot-separated, and camelCase input. Known abbreviations are
// uppercased per Go convention.
func ToGoName(jsonName string) string {
	words := splitWords(jsonName)
	var b strings.Builder
	for _, w := range words {
		if upper, ok := knownAbbreviations[strings.ToLower(w)]; ok {
			b.WriteString(upper)
		} else {
			b.WriteString(capitalize(w))
		}
	}
	return b.String()
}

// ToTypeName derives a unique Go type name from a schema context.
// It uses defName if available (the name from $defs or definitions),
// falling back to constructing a name from the schema file and parent type.
func ToTypeName(schemaFile, defName, parentType string) string {
	if defName != "" {
		return ToGoName(defName)
	}

	// Derive from schema file name (e.g. "manifest.jsonschema.json" → "Manifest").
	base := schemaFile
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		base = base[idx+1:]
	}
	base = strings.TrimSuffix(base, ".jsonschema.json")
	base = strings.TrimSuffix(base, ".json")

	name := ToGoName(base)
	if parentType != "" {
		name = parentType + name
	}
	return name
}

// splitWords breaks an identifier string into its component words. It
// handles snake_case, kebab-case, dot-separated, and camelCase boundaries.
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
			// Check for runs of uppercase (e.g., "URLParser" → "URL", "Parser").
			if current.Len() > 0 && i > 0 && unicode.IsLower(runes[i-1]) {
				flush()
			} else if current.Len() > 1 && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
				// In "URLParser", when we reach 'P' after "URL", flush "UR",
				// the 'L' was already in current. Actually let's re-think:
				// "URLParser" → runes: U R L P a r s e r
				// At 'P' (i=3): current="URL", next is 'a' (lowercase)
				// We should flush "URL" minus last char, but current approach
				// is simpler: flush the accumulated upper run minus last char.
				// Actually, the simple approach: if we're in an upper run and
				// next char is lower, start a new word at this char.
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

// capitalize returns s with its first rune uppercased and the rest lowercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	// Keep remaining characters as-is to handle mixed case.
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}
