package upload

import (
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"backup-home/internal/logging"

	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSH configuration constants from the PowerShell script
const (
	DefaultTargetMachine = "ivans-mac-mini.local"
	DefaultBackupPath    = "/Volumes/Storage/Data/Drive/Crypt/Machines/"
	DefaultSSHUser       = "ivan"
	DefaultSSHPort       = "22"
)

// SSHConfig holds SSH connection configuration
type SSHConfig struct {
	Host       string
	Port       string
	User       string
	Password   string
	KeyFile    string
	RemotePath string
}

// UploadToSSH uploads a backup file to a remote machine via SSH/SFTP
func UploadToSSH(localPath string, config SSHConfig, verbose bool) error {
	return UploadToSSHBinary(localPath, config, verbose)
}

// UploadToSSHOriginal is the original SSH implementation (kept for reference)
func UploadToSSHOriginal(localPath string, config SSHConfig, verbose bool) error {
	// Get the sugar reference for this package
	sugar := logging.GetSugar()

	sugar.Infof("Starting SSH upload to %s@%s:%s", config.User, config.Host, config.Port)
	startTime := time.Now()

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // In production, verify host key
		Timeout:         30 * time.Second,
	}

	// Configure authentication
	if config.KeyFile != "" {
		key, err := os.ReadFile(config.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to read SSH key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %w", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else if config.Password != "" {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(config.Password)}
	} else {
		// Skip SSH agent (it's not working properly with Go SSH library)
		// Go directly to trying default key locations
		sugar.Debugf("Checking for SSH keys in default locations")
		
		keyAuth, err := tryDefaultKeys()
		if err != nil {
			return fmt.Errorf("no SSH keys found in default locations")
		}
		
		sshConfig.Auth = keyAuth
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer sshClient.Close()

	// Create SFTP client with balanced performance optimizations
	sftpClient, err := sftp.NewClient(sshClient,
		sftp.UseConcurrentReads(true),
		sftp.UseConcurrentWrites(true),
		sftp.MaxConcurrentRequestsPerFile(32), // Conservative concurrent requests
		sftp.MaxPacketUnchecked(256*1024),     // 256KB packets (stable size)
	)
	if err != nil {
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}
	defer sftpClient.Close()

	// Build remote path with date directory structure
	hostname, _ := os.Hostname()
	dateDir := time.Now().Format("2006-01-02")
	remotePath := path.Join(config.RemotePath, hostname, "Users", dateDir)

	// Create remote directory structure
	sugar.Debugf("Creating remote directory: %s", remotePath)
	if err := sftpClient.MkdirAll(remotePath); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info for progress tracking
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	// Create remote file
	remoteFileName := filepath.Base(localPath)
	remoteFilePath := path.Join(remotePath, remoteFileName)
	sugar.Infof("Uploading to: %s", remoteFilePath)

	remoteFile, err := sftpClient.Create(remoteFilePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy file content with progress reporting
	progressReader := &progressReader{
		reader:    localFile,
		total:     fileInfo.Size(),
		startTime: startTime,
		sugar:     sugar,
	}
	
	bytesCopied, err := io.Copy(remoteFile, progressReader)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Calculate and log statistics
	elapsed := time.Since(startTime).Seconds()
	fileSizeMB := float64(fileInfo.Size()) / 1024 / 1024
	mbPerSec := fileSizeMB / elapsed

	sugar.Infof("SSH upload completed: %.2f MB transferred (%.2f MB/s)", fileSizeMB, mbPerSec)
	sugar.Infof("Remote file: %s", remoteFilePath)
	sugar.Debugf("Bytes copied: %d", bytesCopied)

	return nil
}

// sshAgentAuth attempts to connect to SSH agent for authentication
func sshAgentAuth() (ssh.AuthMethod, error) {
	agentSock := os.Getenv("SSH_AUTH_SOCK")
	if agentSock == "" {
		return nil, fmt.Errorf("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", agentSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers), nil
}

// progressReader wraps an io.Reader to provide upload progress reporting
type progressReader struct {
	reader      io.Reader
	total       int64
	transferred int64
	startTime   time.Time
	sugar       *zap.SugaredLogger
	lastReport  time.Time
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.transferred += int64(n)
	
	// Report progress every 5 seconds or at completion
	now := time.Now()
	if now.Sub(pr.lastReport) >= 5*time.Second || pr.transferred == pr.total || err == io.EOF {
		pr.lastReport = now
		
		elapsed := now.Sub(pr.startTime).Seconds()
		if elapsed > 0 {
			percentage := float64(pr.transferred) / float64(pr.total) * 100
			transferredMB := float64(pr.transferred) / 1024 / 1024
			totalMB := float64(pr.total) / 1024 / 1024
			mbPerSec := transferredMB / elapsed
			
			if pr.transferred == pr.total || err == io.EOF {
				pr.sugar.Infof("Upload completed: %.2f MB (%.2f MB/s)", totalMB, mbPerSec)
			} else {
				pr.sugar.Infof("Upload progress: %.1f%% (%.2f/%.2f MB, %.2f MB/s)", 
					percentage, transferredMB, totalMB, mbPerSec)
			}
		}
	}
	
	return n, err
}

// tryDefaultKeys attempts to load SSH keys from default locations
func tryDefaultKeys() ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod
	
	// Get user home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	
	// Common SSH key locations
	keyPaths := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
	
	for _, keyPath := range keyPaths {
		if _, err := os.Stat(keyPath); os.IsNotExist(err) {
			continue
		}
		
		key, err := os.ReadFile(keyPath)
		if err != nil {
			continue // Skip this key if we can't read it
		}
		
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue // Skip this key if we can't parse it
		}
		
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid SSH keys found in default locations")
	}
	
	return authMethods, nil
}