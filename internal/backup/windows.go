package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"backup-home/internal/platform"

	"go.uber.org/zap"
)

func createWindowsArchive(source, backupPath string, compressionLevel int) error {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync() // ignoring sync error as we're shutting down
	}()
	sugar := logger.Sugar()

	// Check if 7z is available
	if _, err := exec.LookPath("7z"); err != nil {
		return fmt.Errorf("7z is not found in PATH. Please install 7-Zip first")
	}

	// Build 7z command
	args := []string{
		"a",                                     // Add to archive
		"-tzip",                                 // ZIP format
		"-y",                                    // Yes to all queries
		"-ssw",                                  // Compress files open for writing
		"-r",                                    // Recursive
		fmt.Sprintf("-mx=%d", compressionLevel), // Compression level
		backupPath,                              // Output file
		filepath.Join(source, "*"),              // Source directory with all files
	}

	// Add exclude patterns
	for _, pattern := range platform.GetExcludePatterns() {
		args = append(args, fmt.Sprintf("-xr!%s", pattern))
	}

	cmd := exec.Command("7z", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Check specific exit codes
			switch exitErr.ExitCode() {
			case 1:
				sugar.Info("Archive created successfully with some files skipped")
				return nil
			case 2:
				sugar.Info("Archive created with some files skipped (locked files or permissions)")
				return nil
			default:
				return fmt.Errorf("7-Zip failed with exit code: %d", exitErr.ExitCode())
			}
		}
		return fmt.Errorf("failed to execute 7z: %w", err)
	}

	sugar.Info("Archive created successfully with no warnings")

	// Verify the archive was created
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("failed to create backup archive")
	}

	return nil
}
