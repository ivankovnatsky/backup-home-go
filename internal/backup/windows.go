package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backup-home/internal/platform"

	"github.com/klauspost/compress/zstd"
)

func createWindowsArchive(source, backupPath string, compressionLevel int, verbose bool) error {
	outFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create a new zip archive
	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Configure compression
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return zstd.NewWriter(out, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(compressionLevel)))
	})

	startTime := time.Now()
	lastUpdate := time.Now()
	updateInterval := 5 * time.Second

	excludePatterns := platform.GetExcludePatterns()
	sugar.Infof("Using exclude patterns: [%s]", strings.Join(excludePatterns, ", "))

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

		// Normalize path for pattern matching and Windows paths
		normalizedPath := "./" + filepath.ToSlash(relPath)

		// Check exclude patterns
		for _, pattern := range excludePatterns {
			if strings.Contains(pattern, "**/") {
				dirName := strings.TrimPrefix(pattern, "./**/")
				dirName = strings.TrimSuffix(dirName, "/")
				segments := strings.Split(normalizedPath, "/")
				for _, segment := range segments {
					if segment == dirName {
						if verbose {
							sugar.Debugf("Excluding: %s (matched pattern %s)", normalizedPath, pattern)
						}
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
				}
			} else {
				matched, err := filepath.Match(pattern, normalizedPath)
				if err != nil {
					sugar.Debugf("Invalid pattern %s: %v", pattern, err)
					continue
				}
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
		}

		if verbose {
			sugar.Debugf("Including: %s", normalizedPath)
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("failed to create zip header: %w", err)
		}
		header.Name = relPath
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %w", err)
		}

		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				sugar.Debugf("Failed to open file %s: %v", path, err)
				return nil
			}
			defer file.Close()

			if _, err := io.Copy(writer, file); err != nil {
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
