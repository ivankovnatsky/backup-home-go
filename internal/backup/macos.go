package backup

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-home/internal/logging"
	"backup-home/internal/platform"

	"github.com/klauspost/pgzip"
)

func createMacOSArchive(source, backupPath string, compressionLevel int, verbose bool, ignoreExcludes bool) error {
	// Initialize logger (this is safe to call multiple times)
	if err := logging.InitLogger(verbose); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Get the sugar reference for this package
	sugar = logging.GetSugar()

	outFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Use parallel gzip compression with number of CPU cores
	gzipWriter, err := pgzip.NewWriterLevel(outFile, compressionLevel)
	if err != nil {
		return fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	startTime := time.Now()
	lastUpdate := time.Now()
	updateInterval := 5 * time.Second

	// Get exclude patterns
	var excludePatterns []string
	if !ignoreExcludes {
		excludePatterns = platform.GetExcludePatterns()
		sugar.Infof("Using exclude patterns: [%s]", strings.Join(excludePatterns, ", "))
	} else {
		sugar.Info("Ignoring exclude patterns - backing up everything")
	}

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			sugar.Debugf("Error accessing path %s: %v", path, err)
			return nil
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		if relPath == "." {
			return nil
		}

		// Normalize path for pattern matching
		normalizedPath := "./" + filepath.ToSlash(relPath)

		// Check exclude patterns
		for _, pattern := range excludePatterns {
			segments := strings.Split(pattern, "/")
			pathSegments := strings.Split(normalizedPath, "/")

			matched := matchPattern(segments, pathSegments)
			if matched {
				if verbose {
					sugar.Debugf("Excluding: %s (matched pattern %s)", normalizedPath, pattern)
				}
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if verbose {
			sugar.Debugf("Including: %s", normalizedPath)
		}

		// Create and write header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				sugar.Debugf("Failed to open file %s: %v", path, err)
				return nil
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				sugar.Debugf("Failed to write file %s: %v", path, err)
				return nil
			}
		}

		// Progress reporting
		if time.Since(lastUpdate) >= updateInterval {
			if stat, err := outFile.Stat(); err == nil {
				sizeMB := float64(stat.Size()) / 1024 / 1024
				elapsed := time.Since(startTime).Seconds()
				mbPerSec := sizeMB / elapsed

				sugar.Infof(
					"Archive size: %.2f MB (%.2f MB/s)",
					sizeMB,
					mbPerSec,
				)
			}
			lastUpdate = time.Now()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Final statistics
	if stat, err := outFile.Stat(); err == nil {
		sizeMB := float64(stat.Size()) / 1024 / 1024
		elapsed := time.Since(startTime).Seconds()
		mbPerSec := sizeMB / elapsed

		sugar.Infof(
			"Final archive size: %.2f MB (average speed: %.2f MB/s)",
			sizeMB,
			mbPerSec,
		)
	}

	return nil
}
