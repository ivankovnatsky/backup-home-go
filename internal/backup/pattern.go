package backup

import (
	"path/filepath"
	"runtime"
	"strings"
)

// matchPattern checks if path segments match the pattern segments
func matchPattern(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}

	if len(path) == 0 {
		return false
	}

	// Handle extension patterns (e.g., "**/*.dll")
	if strings.HasPrefix(pattern[0], "*") && strings.Contains(pattern[0], ".") {
		ext := pattern[0][strings.LastIndex(pattern[0], "."):]
		// Ensure case-insensitive matching on Windows
		if runtime.GOOS == "windows" {
			ext = strings.ToLower(ext)
			return strings.HasSuffix(strings.ToLower(path[len(path)-1]), ext)
		}
		return strings.HasSuffix(path[len(path)-1], ext)
	}

	// Handle ** pattern
	if pattern[0] == "**" {
		// Try matching rest of pattern with remaining path
		for i := 0; i <= len(path); i++ {
			if matchPattern(pattern[1:], path[i:]) {
				return true
			}
		}
		return false
	}

	// Handle normal glob pattern
	// On Windows, do case-insensitive matching
	if runtime.GOOS == "windows" {
		matched, err := filepath.Match(strings.ToLower(pattern[0]), strings.ToLower(path[0]))
		if err != nil || !matched {
			return false
		}
	} else {
		matched, err := filepath.Match(pattern[0], path[0])
		if err != nil || !matched {
			return false
		}
	}

	return matchPattern(pattern[1:], path[1:])
}
