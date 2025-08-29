package upload

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"backup-home/internal/logging"
	"github.com/melbahja/goph"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// UploadToSSHGoph uploads a backup file to a remote server using goph library
func UploadToSSHGoph(localPath string, config SSHConfig, verbose bool) error {
	sugar := logging.GetSugar()
	
	sugar.Infof("Starting SSH upload to %s@%s:%s using goph", config.User, config.Host, config.Port)
	startTime := time.Now()
	
	// Get file info
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}
	
	// Configure authentication
	var auth goph.Auth
	
	if config.KeyFile != "" {
		// Use specified key file
		auth, err = goph.Key(config.KeyFile, "")
		if err != nil {
			return fmt.Errorf("failed to load SSH key: %w", err)
		}
		sugar.Debugf("Using SSH key from: %s", config.KeyFile)
	} else if config.Password != "" {
		// Use password
		auth = goph.Password(config.Password)
		sugar.Debugf("Using password authentication")
	} else {
		// Skip SSH agent (it's not working properly with Go SSH library)
		// Go directly to trying default key locations
		sugar.Debugf("Checking for SSH keys in default locations")
		
		home, _ := os.UserHomeDir()
		keyPaths := []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
		
		for _, keyPath := range keyPaths {
			if _, err := os.Stat(keyPath); err == nil {
				auth, err = goph.Key(keyPath, "")
				if err == nil {
					sugar.Debugf("Using SSH key: %s", keyPath)
					break
				}
			}
		}
		
		if auth == nil {
			return fmt.Errorf("no SSH keys found in default locations")
		}
	}
	
	// Connect using goph with custom config to specify port
	portNum := uint(22)
	if config.Port != "" && config.Port != "22" {
		fmt.Sscanf(config.Port, "%d", &portNum)
	}
	
	client, err := goph.NewConn(&goph.Config{
		User:     config.User,
		Addr:     config.Host,
		Port:     portNum,
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer client.Close()
	
	// Build remote path with date directory structure
	hostname, _ := os.Hostname()
	currentTime := time.Now()
	dateDir := currentTime.Format("2006-01-02")
	remotePath := filepath.Join(config.RemotePath, hostname, "Users", dateDir)
	
	// Create remote directory
	sugar.Infof("Creating remote directory: %s", remotePath)
	_, err = client.Run(fmt.Sprintf("mkdir -p %s", remotePath))
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}
	
	// Build full remote file path
	fileName := filepath.Base(localPath)
	remoteFile := filepath.Join(remotePath, fileName)
	
	// Upload file with progress tracking
	sugar.Infof("Uploading %s to %s", localPath, remoteFile)
	sugar.Infof("File size: %.2f MB", float64(fileInfo.Size())/1024/1024)
	
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()
	
	// Get SFTP client from goph with balanced performance optimizations
	sftpClient, err := client.NewSftp(
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(32), // Conservative concurrent requests
		sftp.MaxPacketUnchecked(256*1024),     // 256KB packets (stable size)
	)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()
	
	// Create remote file
	remoteFileHandle, err := sftpClient.Create(remoteFile)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFileHandle.Close()
	
	// Copy with progress tracking (reuse progressReader from ssh.go)
	progressReader := &progressReader{
		reader:    localFile,
		total:     fileInfo.Size(),
		startTime: startTime,
		sugar:     sugar,
	}
	
	_, err = io.Copy(remoteFileHandle, progressReader)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	
	// Calculate and display upload statistics
	duration := time.Since(startTime)
	sizeMB := float64(fileInfo.Size()) / 1024 / 1024
	mbPerSec := sizeMB / duration.Seconds()
	
	sugar.Infof("Upload completed successfully!")
	sugar.Infof("Uploaded %.2f MB in %s (%.2f MB/s)", sizeMB, duration.Round(time.Second), mbPerSec)
	sugar.Infof("Remote path: %s:%s", config.Host, remoteFile)
	
	return nil
}