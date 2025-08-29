package upload

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"backup-home/internal/logging"
)

// UploadToSSHBinary uploads using system scp binary for maximum performance verification
func UploadToSSHBinary(localPath string, config SSHConfig, verbose bool) error {
	sugar := logging.GetSugar()
	
	sugar.Infof("Starting binary scp upload to %s@%s:%s using system scp command", config.User, config.Host, config.Port)
	startTime := time.Now()
	
	// Get file info
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	
	// Build remote path with date directory structure
	hostname, _ := os.Hostname()
	currentTime := time.Now()
	dateDir := currentTime.Format("2006-01-02")
	remotePath := filepath.Join(config.RemotePath, hostname, "Users", dateDir)
	
	// Create remote directory first via SSH
	mkdirArgs := []string{
		config.User + "@" + config.Host,
		fmt.Sprintf("mkdir -p %s", remotePath),
	}
	
	if config.Port != "" && config.Port != "22" {
		mkdirArgs = append([]string{"-p", config.Port}, mkdirArgs...)
	}
	if config.KeyFile != "" {
		mkdirArgs = append([]string{"-i", config.KeyFile}, mkdirArgs...)
	}
	
	sugar.Infof("Creating remote directory: %s", remotePath)
	mkdirCmd := exec.Command("ssh", mkdirArgs...)
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}
	
	// Build scp command arguments
	fileName := filepath.Base(localPath)
	remoteTarget := fmt.Sprintf("%s@%s:%s/%s", config.User, config.Host, remotePath, fileName)
	
	scpArgs := []string{}
	
	// Add port if not default
	if config.Port != "" && config.Port != "22" {
		scpArgs = append(scpArgs, "-P", config.Port)
	}
	
	// Add key file if specified
	if config.KeyFile != "" {
		scpArgs = append(scpArgs, "-i", config.KeyFile)
	}
	
	// Add verbose flag
	if verbose {
		scpArgs = append(scpArgs, "-v")
	}
	
	// Add source and destination
	scpArgs = append(scpArgs, localPath, remoteTarget)
	
	sugar.Infof("Uploading %s to %s", localPath, remoteTarget)
	sugar.Infof("File size: %.2f MB", float64(fileInfo.Size())/1024/1024)
	sugar.Debugf("Running: scp %v", scpArgs)
	
	// Execute scp command
	scpCmd := exec.Command("scp", scpArgs...)
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr
	
	err = scpCmd.Run()
	if err != nil {
		return fmt.Errorf("scp command failed: %w", err)
	}
	
	// Calculate and display upload statistics
	duration := time.Since(startTime)
	sizeMB := float64(fileInfo.Size()) / 1024 / 1024
	mbPerSec := sizeMB / duration.Seconds()
	
	sugar.Infof("Binary scp upload completed successfully!")
	sugar.Infof("Uploaded %.2f MB in %s (%.2f MB/s)", sizeMB, duration.Round(time.Second), mbPerSec)
	
	return nil
}