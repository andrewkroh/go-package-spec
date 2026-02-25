package pkgreader

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andrewkroh/go-package-spec/pkgspec"
)

var (
	headCache     sync.Map // repo root path → commit ID
	toplevelCache sync.Map // dir path → repo root path
)

// gitRevParseHEAD returns the current HEAD commit ID for the given directory.
// Results are cached per repository root so that repeated calls from different
// subdirectories of the same repo avoid spawning redundant git processes.
func gitRevParseHEAD(dir string) (string, error) {
	root, err := gitToplevel(dir)
	if err != nil {
		// Fallback: run rev-parse without caching.
		return gitRevParseHEADUncached(dir)
	}

	if v, ok := headCache.Load(root); ok {
		return v.(string), nil
	}

	commit, err := gitRevParseHEADUncached(root)
	if err != nil {
		return "", err
	}
	headCache.Store(root, commit)
	return commit, nil
}

// gitToplevel returns the absolute path of the repository root.
// Results are cached because the repo root is the same for all
// subdirectories within a repository.
func gitToplevel(dir string) (string, error) {
	if v, ok := toplevelCache.Load(dir); ok {
		return v.(string), nil
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	root := strings.TrimSpace(string(out))
	toplevelCache.Store(dir, root)
	return root, nil
}

func gitRevParseHEADUncached(dir string) (string, error) {
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
// It streams output from git blame to avoid buffering the entire output.
func gitBlameTimestamps(dir, filePath string) (map[int]time.Time, error) {
	cmd := exec.Command("git", "blame", "--porcelain", filePath)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git blame %s: %w", filePath, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("git blame %s: %w", filePath, err)
	}

	result := make(map[int]time.Time)
	scanner := bufio.NewScanner(stdout)

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

	scanErr := scanner.Err()

	// Always call Wait to reap the child process and avoid zombies.
	waitErr := cmd.Wait()

	if scanErr != nil {
		return nil, fmt.Errorf("git blame %s: scan: %w", filePath, scanErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("git blame %s: %w", filePath, waitErr)
	}

	return result, nil
}

// annotateChangelogDates uses git blame to populate the Date field on
// changelog entries.
func annotateChangelogDates(changelog []pkgspec.Changelog, dir, changelogPath string) error {
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
