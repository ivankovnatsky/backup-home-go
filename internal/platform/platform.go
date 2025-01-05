package platform

import (
	"os"
	"runtime"
)

// GetExcludePatterns returns platform-specific exclude patterns
func GetExcludePatterns() []string {
	switch runtime.GOOS {
	case "windows":
		return getWindowsExcludes()
	case "darwin":
		return getMacOSExcludes()
	case "linux":
		return getLinuxExcludes()
	default:
		return []string{}
	}
}

// GetTempDir returns the system's temporary directory
func GetTempDir() (string, error) {
	return os.TempDir(), nil
}
