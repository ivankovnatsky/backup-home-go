package upload

import (
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

func UploadToRclone(source, destination string) error {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync()
	}()
	sugar := logger.Sugar()

	sugar.Infof("Uploading backup to: %s", destination)

	cmd := exec.Command("rclone",
		"copy",
		"--progress",
		source,
		destination)

	// Connect command's stdout and stderr to our process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone upload failed: %w", err)
	}

	sugar.Info("Upload completed successfully")
	return nil
}
