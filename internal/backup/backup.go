package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

const defaultCompressionLevel = 6

// CreateBackup creates a backup of the specified source directory
func CreateBackup(source string, compressionLevel int) (string, error) {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync() // ignoring sync error as we're shutting down
	}()
	sugar := logger.Sugar()

	// Validate source path exists
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return "", fmt.Errorf("source directory does not exist: %s", source)
	}

	// Use provided compression level or default to 6
	if compressionLevel < 0 || compressionLevel > 9 {
		compressionLevel = defaultCompressionLevel
	}

	// Get temporary directory for backup
	tempDir := os.TempDir()
	username, err := getUsername()
	if err != nil {
		return "", fmt.Errorf("failed to get username: %w", err)
	}

	backupPath := filepath.Join(tempDir, fmt.Sprintf("%s.%s", username, getArchiveExtension()))

	sugar.Infof("Creating backup of: %s", source)
	sugar.Infof("Backup file: %s", backupPath)
	sugar.Infof("Using compression level: %d", compressionLevel)

	// Create platform-specific archive
	if err := createArchive(source, backupPath, compressionLevel); err != nil {
		return "", fmt.Errorf("failed to create archive: %w", err)
	}

	return backupPath, nil
}

func getUsername() (string, error) {
	username := os.Getenv("USER")
	if username == "" {
		home, err := homedir.Dir()
		if err != nil {
			return "", err
		}
		username = filepath.Base(home)
	}
	return username, nil
}

func getArchiveExtension() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}
	return "tar.gz"
}

// createArchive delegates to the appropriate platform-specific implementation
func createArchive(source, backupPath string, compressionLevel int) error {
	switch runtime.GOOS {
	case "darwin":
		return createMacOSArchive(source, backupPath, compressionLevel)
	case "windows":
		return createWindowsArchive(source, backupPath, compressionLevel)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
