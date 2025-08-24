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
		// Try SSH agent if available
		if agentAuth, err := sshAgentAuth(); err == nil {
			sshConfig.Auth = []ssh.AuthMethod{agentAuth}
		} else {
			return fmt.Errorf("no authentication method available (provide --ssh-key or --ssh-password)")
		}
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server: %w", err)
	}
	defer sshClient.Close()

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
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

	// Copy file content
	bytesCopied, err := io.Copy(remoteFile, localFile)
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