package main

import (
	"fmt"
	"os"

	"backup-home/internal/backup"
	"backup-home/internal/upload"

	"github.com/mitchellh/go-homedir"
	_ "github.com/rclone/rclone/backend/all"   // import all backends
	_ "github.com/rclone/rclone/fs/operations" // import operations/* rc commands
	_ "github.com/rclone/rclone/fs/sync"       // import sync/*
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	version   = "dev"
	gitCommit = "none"
	buildTime = "unknown"
)

type CliOptions struct {
	source      string
	destination string
	compression int
	preview     bool
	verbose     bool
}

func main() {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync() // ignoring sync error as we're shutting down
	}()
	sugar := logger.Sugar()

	var opts CliOptions

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
				fmt.Printf("Destination: %s\n", opts.destination)
				fmt.Printf("Compression level: %d\n", opts.compression)
				fmt.Println("\nThis would:")
				fmt.Printf("1. Create backup archive of: %s\n", opts.source)
				fmt.Printf("2. Upload to: %s\n", opts.destination)
				fmt.Println("3. Clean up temporary files")
				return nil
			}

			// Create backup
			backupPath, err := backup.CreateBackup(opts.source, opts.compression, opts.verbose)
			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			// Upload backup
			if err := upload.UploadToRclone(backupPath, opts.destination, opts.verbose); err != nil {
				return fmt.Errorf("failed to upload backup: %w", err)
			}

			// Cleanup
			if err := os.Remove(backupPath); err != nil {
				sugar.Warnf("Failed to cleanup backup file: %v", err)
			}

			return nil
		},
	}

	rootCmd.Flags().StringVarP(&opts.source, "source", "s", "", "Source directory to backup (defaults to home directory)")
	rootCmd.Flags().StringVarP(&opts.destination, "destination", "d", "", "Destination path for rclone (e.g., \"drive:\", \"gdrive:backup/home\")")
	rootCmd.Flags().IntVarP(&opts.compression, "compression", "c", 6, "Compression level (0-9, default: 6)")
	rootCmd.Flags().BoolVar(&opts.preview, "preview", false, "Preview what would be done without actually doing it")
	rootCmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", false, "Enable verbose output")

	if err := rootCmd.MarkFlagRequired("destination"); err != nil {
		sugar.Fatal(err)
	}

	if err := rootCmd.Execute(); err != nil {
		sugar.Fatal(err)
	}
}
