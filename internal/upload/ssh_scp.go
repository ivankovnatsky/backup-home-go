package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"backup-home/internal/logging"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
)

// UploadToSSHSCP uploads a backup file using native SCP protocol for maximum speed
func UploadToSSHSCP(localPath string, config SSHConfig, verbose bool) error {
	sugar := logging.GetSugar()
	
	sugar.Infof("Starting SCP upload to %s@%s:%s using native SCP protocol", config.User, config.Host, config.Port)
	startTime := time.Now()
	
	// Get file info
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	
	// Configure authentication
	var clientConfig ssh.ClientConfig
	
	if config.KeyFile != "" {
		// Use specified key file
		clientConfig, err = auth.PrivateKey(config.User, config.KeyFile, ssh.InsecureIgnoreHostKey())
		if err != nil {
			return fmt.Errorf("failed to load SSH key: %w", err)
		}
		sugar.Debugf("Using SSH key from: %s", config.KeyFile)
	} else if config.Password != "" {
		// Use password
		clientConfig, err = auth.PasswordKey(config.User, config.Password, ssh.InsecureIgnoreHostKey())
		if err != nil {
			return fmt.Errorf("failed to configure password authentication: %w", err)
		}
		sugar.Debugf("Using password authentication")
	} else {
		// Try default key locations
		sugar.Debugf("Checking for SSH keys in default locations")
		
		home, _ := os.UserHomeDir()
		keyPaths := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
		
		for _, keyPath := range keyPaths {
			if _, err := os.Stat(keyPath); err == nil {
				clientConfig, err = auth.PrivateKey(config.User, keyPath, ssh.InsecureIgnoreHostKey())
				if err == nil {
					sugar.Debugf("Using SSH key: %s", keyPath)
					break
				}
			}
		}
		
		if clientConfig.User == "" {
			return fmt.Errorf("no SSH keys found in default locations")
		}
	}
	
	// Create SCP client
	scpClient := scp.NewClient(fmt.Sprintf("%s:%s", config.Host, config.Port), &clientConfig)
	
	// Connect to the remote server
	err = scpClient.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer scpClient.Close()
	
	// Build remote path with date directory structure
	hostname, _ := os.Hostname()
	currentTime := time.Now()
	dateDir := currentTime.Format("2006-01-02")
	remotePath := filepath.Join(config.RemotePath, hostname, "Users", dateDir)
	
	// Create remote directory using SSH session
	session, err := scpClient.SSHClient().NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	
	sugar.Infof("Creating remote directory: %s", remotePath)
	_, err = session.CombinedOutput(fmt.Sprintf("mkdir -p %s", remotePath))
	session.Close()
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}
	
	// Build full remote file path
	fileName := filepath.Base(localPath)
	remoteFile := filepath.Join(remotePath, fileName)
	
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()
	
	sugar.Infof("Uploading %s to %s", localPath, remoteFile)
	sugar.Infof("File size: %.2f MB", float64(fileInfo.Size())/1024/1024)
	
	// Upload using SCP protocol with progress tracking
	err = scpClient.CopyFromFilePassThru(context.Background(), *localFile, remoteFile, "0644", func(r io.Reader, total int64) io.Reader {
		return &progressReader{
			reader:    r,
			total:     total,
			startTime: startTime,
			sugar:     sugar,
		}
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	
	// Calculate and display upload statistics
	duration := time.Since(startTime)
	sizeMB := float64(fileInfo.Size()) / 1024 / 1024
	mbPerSec := sizeMB / duration.Seconds()
	
	sugar.Infof("SCP upload completed successfully!")
	sugar.Infof("Uploaded %.2f MB in %s (%.2f MB/s)", sizeMB, duration.Round(time.Second), mbPerSec)
	sugar.Infof("Remote path: %s:%s", config.Host, remoteFile)
	
	return nil
}