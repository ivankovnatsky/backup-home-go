package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"backup-home/internal/logging"

	"github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

// Initialize sugar variable at package level for convenience
var sugar *zap.SugaredLogger

// CreateBackup creates a backup of the specified source directory
func CreateBackup(source string, backupPath string, compressionLevel int, verbose bool, ignoreExcludes bool, skipOnError bool) (string, error) {
	// Initialize logger
	if err := logging.InitLogger(verbose); err != nil {
		return "", fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logging.SyncLogger()

	// Get the sugar reference for this package
	sugar = logging.GetSugar()

	if _, err := os.Stat(source); os.IsNotExist(err) {
		return "", fmt.Errorf("source directory does not exist: %s", source)
	}

	if compressionLevel < 0 || compressionLevel > 9 {
		compressionLevel = defaultCompressionLevel
	}

	// Use provided backup path or create default one
	if backupPath == "" {
		tempDir := os.TempDir()
		username, err := getUsername()
		if err != nil {
			return "", fmt.Errorf("failed to get username: %w", err)
		}
		backupPath = filepath.Join(tempDir, fmt.Sprintf("%s.%s", username, getArchiveExtension()))
	}

	// Check if backup file already exists
	if _, err := os.Stat(backupPath); err == nil {
		sugar.Infof("Backup file already exists: %s", backupPath)
		sugar.Infof("Skipping backup creation and using existing file")
		return backupPath, nil
	}

	sugar.Infof("Creating backup of: %s", source)
	sugar.Infof("Backup file: %s", backupPath)
	sugar.Infof("Using compression level: %d", compressionLevel)
	if ignoreExcludes {
		sugar.Infof("Ignoring exclude patterns - backing up everything")
	}

	if err := createArchive(source, backupPath, compressionLevel, verbose, ignoreExcludes, skipOnError); err != nil {
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
