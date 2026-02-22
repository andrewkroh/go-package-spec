package reader

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/andrewkroh/go-package-spec/packagespec"
)

// gitRevParseHEAD returns the current HEAD commit ID for the given directory.
func gitRevParseHEAD(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// blameEntry holds the parsed result of a single git blame output line.
type blameEntry struct {
	line int
	date time.Time
}

// gitBlameTimestamps returns the author time for each line of the given file.
func gitBlameTimestamps(dir, filePath string) (map[int]time.Time, error) {
	cmd := exec.Command("git", "blame", "--porcelain", filePath)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git blame %s: %w", filePath, err)
	}

	result := make(map[int]time.Time)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))

	var currentLine int
	for scanner.Scan() {
		line := scanner.Text()

		// Lines starting with a commit hash (40 hex chars) contain
		// the line number as the third field.
		if len(line) > 40 && isHexString(line[:40]) {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				if n, err := strconv.Atoi(parts[2]); err == nil {
					currentLine = n
				}
			}
			continue
		}

		if strings.HasPrefix(line, "author-time ") {
			ts := strings.TrimPrefix(line, "author-time ")
			epoch, err := strconv.ParseInt(ts, 10, 64)
			if err == nil && currentLine > 0 {
				result[currentLine] = time.Unix(epoch, 0).UTC()
			}
		}
	}

	return result, scanner.Err()
}

// annotateChangelogDates uses git blame to populate the Date field on
// changelog entries.
func annotateChangelogDates(changelog []packagespec.Changelog, dir, changelogPath string) error {
	timestamps, err := gitBlameTimestamps(dir, changelogPath)
	if err != nil {
		return err
	}

	for i := range changelog {
		line := changelog[i].Line()
		if line > 0 {
			if ts, ok := timestamps[line]; ok {
				changelog[i].Date = &ts
			}
		}
	}

	return nil
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
