package main

import (
	"fmt"
	"log"
	"os"

	"backup-home/internal/backup"
	"backup-home/internal/logging"
	"backup-home/internal/upload"

	"github.com/mitchellh/go-homedir"
	_ "github.com/rclone/rclone/backend/all"   // import all backends
	_ "github.com/rclone/rclone/fs/operations" // import operations/* rc commands
	_ "github.com/rclone/rclone/fs/sync"       // import sync/*
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitCommit = "none"
	buildTime = "unknown"
)

type options struct {
	source        string
	rclone        string
	backupPath    string
	compression   int
	verbose       bool
	preview       bool
	skipOnError   bool
	skipUpload    bool
	keepBackup    bool
	ignoreExcludes bool
	backupOnly    bool
	skipBackup    bool
	// SSH upload options
	useSSH       bool
	sshHost      string
	sshPort      string
	sshUser      string
	sshPassword  string
	sshKeyFile   string
	sshRemotePath string
}

func main() {
	var opts options

	// We'll update the logger with the verbose flag after parsing args
	// but initialize with defaults for now
	if err := logging.InitLogger(false); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logging.SyncLogger()

	// Get sugar for local use
	sugar := logging.GetSugar()

	var rootCmd = &cobra.Command{
		Use:     "backup-home",
		Short:   "Backup home directory to cloud storage",
		Version: fmt.Sprintf("%s (commit: %s, built at: %s)", version, gitCommit, buildTime),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get source directory or default to home
			if opts.source == "" {
				home, err := homedir.Dir()
				if err != nil {
					return fmt.Errorf("could not determine home directory: %w", err)
				}
				opts.source = home
			}

			if opts.preview {
				fmt.Println("\nPreview summary:")
				fmt.Println("---------------")
				fmt.Printf("Source: %s\n", opts.source)
				if !opts.skipUpload && !opts.backupOnly {
					if opts.useSSH {
						fmt.Printf("SSH Destination: %s@%s:%s%s\n", opts.sshUser, opts.sshHost, opts.sshRemotePath, "[hostname]/Users/[date]/")
					} else {
						fmt.Printf("Rclone destination: %s\n", opts.rclone)
					}
				}
				fmt.Printf("Compression level: %d\n", opts.compression)
				if opts.ignoreExcludes {
					fmt.Println("Ignore excludes: Yes (backing up everything)")
				}
				fmt.Println("\nThis would:")
				fmt.Printf("1. Create backup archive of: %s\n", opts.source)
				if opts.backupOnly {
					fmt.Println("2. Keep backup file locally (backup-only mode)")
				} else if !opts.skipUpload {
					if opts.useSSH {
						fmt.Printf("2. Upload via SSH to: %s@%s\n", opts.sshUser, opts.sshHost)
					} else {
						fmt.Printf("2. Upload to: %s\n", opts.rclone)
					}
					if !opts.keepBackup {
						fmt.Println("3. Clean up temporary files")
					} else {
						fmt.Println("3. Keep backup file after upload")
					}
				} else {
					fmt.Println("2. Skip upload (backup file will be preserved)")
				}
				return nil
			}

			// Create or use existing backup
			var backupPath string
			var err error
			if opts.skipBackup {
				if opts.backupPath == "" {
					return fmt.Errorf("--backup-path is required when using --skip-backup")
				}
				if _, err := os.Stat(opts.backupPath); os.IsNotExist(err) {
					return fmt.Errorf("backup file not found: %s", opts.backupPath)
				}
				backupPath = opts.backupPath
				sugar.Infof("Using existing backup file: %s", backupPath)
			} else {
				backupPath, err = backup.CreateBackup(opts.source, opts.backupPath, opts.compression, opts.verbose, opts.ignoreExcludes, opts.skipOnError)
			}
			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			// Handle upload based on mode
			if opts.backupOnly {
				sugar.Infof("Backup-only mode. Backup file is available at: %s", backupPath)
			} else if !opts.skipUpload {
				var uploadErr error
				
				if opts.useSSH {
					// Upload via SSH
					sshConfig := upload.SSHConfig{
						Host:       opts.sshHost,
						Port:       opts.sshPort,
						User:       opts.sshUser,
						Password:   opts.sshPassword,
						KeyFile:    opts.sshKeyFile,
						RemotePath: opts.sshRemotePath,
					}
					uploadErr = upload.UploadToSSH(backupPath, sshConfig, opts.verbose)
				} else {
					// Upload via rclone
					uploadErr = upload.UploadToRclone(backupPath, opts.rclone, opts.verbose)
				}

				if uploadErr != nil {
					sugar.Errorf("Upload failed: %v", uploadErr)
					sugar.Infof("Backup file preserved at: %s", backupPath)
					return fmt.Errorf("failed to upload backup: %w", uploadErr)
				}

				// Cleanup only after successful upload and if not keeping backup
				if !opts.keepBackup {
					if err := os.Remove(backupPath); err != nil {
						sugar.Warnf("Failed to cleanup backup file after successful upload: %v", err)
					} else {
						sugar.Infof("Successfully uploaded and cleaned up backup file")
					}
				} else {
					sugar.Infof("Upload completed successfully. Backup file preserved at: %s", backupPath)
				}
			} else {
				sugar.Infof("Upload skipped. Backup file is available at: %s", backupPath)
			}

			return nil
		},
	}

	homeDir, err := homedir.Dir()
	if err != nil {
		// Before logger is fully initialized, fall back to standard log
		log.Fatalf("failed to get home directory: %v", err)
	}

	rootCmd.Flags().StringVarP(&opts.source, "source", "s", homeDir, "Source directory to backup (defaults to home directory)")
	rootCmd.Flags().StringVarP(&opts.rclone, "rclone", "r", "", "Rclone destination path (e.g., \"drive:\", \"gdrive:backup/home\")")
	rootCmd.Flags().StringVar(&opts.backupPath, "backup-path", "", "Custom path for temporary backup file (defaults to system temp directory)")
	rootCmd.Flags().IntVarP(&opts.compression, "compression", "c", 6, "Compression level (0-9, default: 6)")
	rootCmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().Bool("preview", false, "Preview what would be done without actually doing it")
	rootCmd.Flags().BoolVar(&opts.skipOnError, "skip-errors", true, "Skip files that can't be accessed instead of failing")
	rootCmd.Flags().BoolVar(&opts.skipUpload, "skip-upload", false, "Skip uploading the backup archive")
	rootCmd.Flags().BoolVar(&opts.keepBackup, "keep-backup", false, "Keep the backup file after uploading")
	rootCmd.Flags().BoolVar(&opts.ignoreExcludes, "ignore-excludes", false, "Ignore exclude patterns and backup everything")
	rootCmd.Flags().BoolVar(&opts.backupOnly, "backup-only", false, "Create backup archive only, skip all uploads")
	rootCmd.Flags().BoolVar(&opts.skipBackup, "skip-backup", false, "Skip backup creation and upload existing backup file (requires --backup-path)")
	// SSH upload flags
	rootCmd.Flags().BoolVar(&opts.useSSH, "ssh", false, "Use SSH/SCP upload instead of rclone")
	rootCmd.Flags().StringVar(&opts.sshHost, "ssh-host", upload.DefaultTargetMachine, "SSH host to upload to")
	rootCmd.Flags().StringVar(&opts.sshPort, "ssh-port", upload.DefaultSSHPort, "SSH port")
	rootCmd.Flags().StringVar(&opts.sshUser, "ssh-user", upload.DefaultSSHUser, "SSH username")
	rootCmd.Flags().StringVar(&opts.sshPassword, "ssh-password", "", "SSH password (not recommended, use key file instead)")
	rootCmd.Flags().StringVar(&opts.sshKeyFile, "ssh-key", "", "SSH private key file path (defaults to SSH agent)")
	rootCmd.Flags().StringVar(&opts.sshRemotePath, "ssh-remote-path", upload.DefaultBackupPath, "Remote base path for backups")

	// Update logger and validate flags before running
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Update logger with verbose flag
		if err := logging.InitLogger(opts.verbose); err != nil {
			return fmt.Errorf("failed to reinitialize logger: %w", err)
		}

		// Set default upload mode to SSH if no mode is specified
		skipUpload, _ := cmd.Flags().GetBool("skip-upload")
		if !skipUpload && !opts.backupOnly && opts.rclone == "" && !opts.useSSH {
			opts.useSSH = true
		}
		
		// Validate configuration based on selected mode
		if !skipUpload && !opts.backupOnly {
			if opts.useSSH {
				// Validate SSH configuration
				if opts.sshHost == "" {
					return fmt.Errorf("SSH host is required when using SSH upload")
				}
			} else if opts.rclone != "" {
				// rclone mode - no additional validation needed
			} else {
				return fmt.Errorf("must specify upload mode: --rclone (rclone upload), --ssh (SSH upload), or --backup-only (local only)")
			}
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
