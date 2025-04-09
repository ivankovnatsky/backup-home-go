package upload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"backup-home/internal/logging"

	"github.com/rclone/rclone/librclone/librclone"
	"go.uber.org/zap"
)

// Initialize sugar variable at package level for convenience
var sugar *zap.SugaredLogger

type copyFileRequest struct {
	SrcFs     string `json:"srcFs"`
	SrcRemote string `json:"srcRemote"`
	DstFs     string `json:"dstFs"`
	DstRemote string `json:"dstRemote"`
}

func UploadToRclone(source, destination string, verbose bool) error {
	// Initialize logger
	if err := logging.InitLogger(verbose); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logging.SyncLogger()
	
	// Get the sugar reference for this package
	sugar = logging.GetSugar()

	sugar.Infof("Uploading backup to: %s", destination)
	startTime := time.Now()

	// Initialize librclone
	librclone.Initialize()
	defer librclone.Finalize()

	// Prepare the request
	srcDir := filepath.Dir(source)
	srcFile := filepath.Base(source)

	req := copyFileRequest{
		SrcFs:     srcDir,
		SrcRemote: srcFile,
		DstFs:     destination,
		DstRemote: srcFile,
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Execute the copy operation
	out, status := librclone.RPC("operations/copyfile", string(reqJSON))
	if status != 0 && status != 200 { // Allow both 0 and 200 as success codes
		return fmt.Errorf("rclone copy failed with status %d: %s", status, out)
	}

	// Calculate and log statistics
	elapsed := time.Since(startTime).Seconds()
	fileInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	mbPerSec := fileSizeMB / elapsed

	sugar.Infof("Upload completed: %.2f MB transferred (%.2f MB/s)", fileSizeMB, mbPerSec)
	return nil
}
