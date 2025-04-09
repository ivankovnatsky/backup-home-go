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
	source      string
	destination string
	backupPath  string
	compression int
	verbose     bool
	preview     bool
	skipOnError bool
	skipUpload  bool
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
				if !opts.skipUpload {
					fmt.Printf("Destination: %s\n", opts.destination)
				}
				fmt.Printf("Compression level: %d\n", opts.compression)
				fmt.Println("\nThis would:")
				fmt.Printf("1. Create backup archive of: %s\n", opts.source)
				if !opts.skipUpload {
					fmt.Printf("2. Upload to: %s\n", opts.destination)
					fmt.Println("3. Clean up temporary files")
				} else {
					fmt.Println("2. Skip upload (backup file will be preserved)")
				}
				return nil
			}

			// Create backup
			backupPath, err := backup.CreateBackup(opts.source, opts.backupPath, opts.compression, opts.verbose)
			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			// Upload backup if not skipped
			if !opts.skipUpload {
				if err := upload.UploadToRclone(backupPath, opts.destination, opts.verbose); err != nil {
					return fmt.Errorf("failed to upload backup: %w", err)
				}

				// Cleanup only if uploaded
				if err := os.Remove(backupPath); err != nil {
					sugar.Warnf("Failed to cleanup backup file: %v", err)
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
	rootCmd.Flags().StringVarP(&opts.destination, "destination", "d", "", "Destination path for rclone (e.g., \"drive:\", \"gdrive:backup/home\")")
	rootCmd.Flags().StringVar(&opts.backupPath, "backup-path", "", "Custom path for temporary backup file (defaults to system temp directory)")
	rootCmd.Flags().IntVarP(&opts.compression, "compression", "c", 6, "Compression level (0-9, default: 6)")
	rootCmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.Flags().Bool("preview", false, "Preview what would be done without actually doing it")
	rootCmd.Flags().BoolVar(&opts.skipOnError, "skip-errors", true, "Skip files that can't be accessed instead of failing")
	rootCmd.Flags().BoolVar(&opts.skipUpload, "skip-upload", false, "Skip uploading the backup archive")

	// Update logger and validate flags before running 
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		// Update logger with verbose flag
		if err := logging.InitLogger(opts.verbose); err != nil {
			return fmt.Errorf("failed to reinitialize logger: %w", err)
		}
		
		// Check if destination is provided when needed
		skipUpload, _ := cmd.Flags().GetBool("skip-upload")
		if !skipUpload {
			if opts.destination == "" {
				return fmt.Errorf("required flag \"destination\" not set")
			}
		}
		return nil
	}

	if err := rootCmd.Execute(); err != nil {
		// Get the latest sugar in case it was updated by PreRunE
		logging.GetSugar().Fatal(err)
	}
}
