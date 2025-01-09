package backup

import (
	"fmt"
	"runtime"
)

const defaultCompressionLevel = 6

// createArchive delegates to the appropriate platform-specific implementation
func createArchive(source, backupPath string, compressionLevel int, verbose bool) error {
	switch runtime.GOOS {
	case "darwin":
		return createMacOSArchive(source, backupPath, compressionLevel, verbose)
	case "linux":
		return createLinuxArchive(source, backupPath, compressionLevel, verbose)
	case "windows":
		return createWindowsArchive(source, backupPath, compressionLevel, verbose)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
