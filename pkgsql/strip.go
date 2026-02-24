package pkgsql

import (
	"strings"
)

// stripFieldTables removes auto-generated field tables and example event JSON
// blocks from rendered package documentation markdown.
//
// Most package READMEs are generated from Go text/templates that expand
// {{fields "stream"}} and {{event "stream"}} markers into large markdown
// field tables and fenced JSON code blocks. These dominate the document and
// are redundant with the structured fields and sample_events tables in the
// database. Stripping them improves FTS search quality by preventing field
// names and example JSON from polluting relevance ranking — for example,
// searching "timeout" won't match every package that has a *.timeout field.
//
// The function removes:
//
//  1. Field tables: markdown tables whose header row matches
//     "| Field | Description | Type" (with optional extra columns like Unit
//     or Metric Type). The preceding "**Exported fields**" or
//     "*Exported fields*" header line is also removed.
//
//  2. Example event JSON blocks: fenced code blocks tagged ```json that are
//     preceded (within 2 lines) by a line matching "An example event for".
//     The intro line, intervening blank lines, and the entire fenced block
//     are removed.
//
// Non-field markdown tables (e.g. "| Job | Description |") are preserved
// because their header row does not match the field table pattern.
func stripFieldTables(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))

	// skipTrailingBlanks advances i past any blank lines that immediately
	// follow a stripped section. This avoids double blank lines where the
	// stripped content was, since there is typically already a blank line
	// preceding the stripped section in the output.
	skipTrailingBlanks := func(i int) int {
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
		return i
	}

	i := 0
	for i < len(lines) {
		line := lines[i]

		// Check for "**Exported fields**" or "*Exported fields*" header
		// followed by a field table.
		if isExportedFieldsHeader(line) {
			// Look ahead for the field table (skip blank lines).
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j < len(lines) && isFieldTableHeader(lines[j]) {
				// Skip the exported fields header, blank lines, and the table.
				i = skipTrailingBlanks(skipTable(lines, j))
				continue
			}
		}

		// Check for a standalone field table (no preceding exported fields header).
		if isFieldTableHeader(line) {
			i = skipTrailingBlanks(skipTable(lines, i))
			continue
		}

		// Check for example event intro line.
		if isExampleEventIntro(line) {
			// Look ahead for ```json within 2 lines.
			j := i + 1
			blanks := 0
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" && blanks < 2 {
				j++
				blanks++
			}
			if j < len(lines) && isFencedJSONOpen(lines[j]) {
				// Skip intro, blanks, and the fenced block.
				i = skipTrailingBlanks(skipFencedBlock(lines, j))
				continue
			}
		}

		out = append(out, line)
		i++
	}

	return strings.Join(out, "\n")
}

// isExportedFieldsHeader returns true if the line is an "Exported fields"
// header in bold or italic markdown.
func isExportedFieldsHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "**Exported fields**" || trimmed == "*Exported fields*"
}

// isFieldTableHeader returns true if the line looks like the header row of
// an auto-generated field table: "| Field | Description | Type |" with
// optional extra columns.
func isFieldTableHeader(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "| Field | Description | Type")
}

// skipTable advances past a markdown table starting at lines[start] (the
// header row). It skips the header, separator, and all contiguous rows
// that start with "|". Returns the index of the first line after the table.
func skipTable(lines []string, start int) int {
	i := start
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || !strings.HasPrefix(trimmed, "|") {
			break
		}
		i++
	}
	return i
}

// isExampleEventIntro returns true if the line matches the pattern
// "An example event for" which precedes auto-generated example event blocks.
func isExampleEventIntro(line string) bool {
	return strings.Contains(line, "An example event for")
}

// isFencedJSONOpen returns true if the line opens a fenced JSON code block.
func isFencedJSONOpen(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "```json"
}

// skipFencedBlock advances past a fenced code block starting at lines[start]
// (the opening ``` line). Returns the index of the first line after the
// closing ```.
func skipFencedBlock(lines []string, start int) int {
	i := start + 1 // skip opening fence
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "```" {
			return i + 1 // skip closing fence
		}
		i++
	}
	// No closing fence found; skip to end.
	return i
}
