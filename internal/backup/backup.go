package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

var sugar *zap.SugaredLogger

// CreateBackup creates a backup of the specified source directory
func CreateBackup(source string, backupPath string, compressionLevel int, verbose bool) (string, error) {
	var logger *zap.Logger
	var err error
	if verbose {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		return "", fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	sugar = logger.Sugar()

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

	sugar.Infof("Creating backup of: %s", source)
	sugar.Infof("Backup file: %s", backupPath)
	sugar.Infof("Using compression level: %d", compressionLevel)

	if err := createArchive(source, backupPath, compressionLevel, verbose); err != nil {
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
