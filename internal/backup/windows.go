package backup

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"backup-home/internal/logging"
	"backup-home/internal/platform"

	"github.com/klauspost/compress/zstd"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024) // 32KB buffers
	},
}

func createWindowsArchive(source, backupPath string, compressionLevel int, verbose bool) error {
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

	// Create a buffered writer to improve I/O performance
	bufferedWriter := bufio.NewWriterSize(outFile, 1024*1024) // 1MB buffer
	defer bufferedWriter.Flush()

	// Create a new zip archive
	zipWriter := zip.NewWriter(bufferedWriter)
	defer zipWriter.Close()

	// Configure compression
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return zstd.NewWriter(out,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(compressionLevel)),
			zstd.WithEncoderConcurrency(runtime.GOMAXPROCS(0)),
			zstd.WithWindowSize(32*1024*1024),
			zstd.WithZeroFrames(true),
		)
	})

	// Create worker pool for parallel processing
	numWorkers := runtime.GOMAXPROCS(0)
	filesChan := make(chan *fileToProcess, numWorkers*2)
	errorsChan := make(chan error, numWorkers)
	var wg sync.WaitGroup

	// Add mutex for zip writer
	var zipMutex sync.Mutex

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range filesChan {
				// Lock the zip writer during file addition
				zipMutex.Lock()
				err := addFileToZip(zipWriter, file.path, file.info, file.relPath)
				zipMutex.Unlock()

				if err != nil {
					errorsChan <- err
				}
			}
		}()
	}

	// Walk the directory and send files to workers
	startTime := time.Now()
	lastUpdate := time.Now()
	updateInterval := 5 * time.Second
	var totalSize int64

	excludePatterns := platform.GetExcludePatterns()
	var displayPatterns []string
	for _, pattern := range excludePatterns {
		// Keep Windows backslashes for display
		displayPatterns = append(displayPatterns, pattern)
	}
	sugar.Infof("Using exclude patterns: [%s]", strings.Join(displayPatterns, ", "))

	go func() {
		err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				sugar.Debugf("Error accessing path %s: %v", path, err)
				return nil
			}

			relPath, err := filepath.Rel(source, path)
			if err != nil {
				return nil
			}

			if isExcluded(relPath, excludePatterns) {
				if info.IsDir() {
					sugar.Debugf("Excluding directory: %s", relPath)
					return filepath.SkipDir
				}
				sugar.Debugf("Excluding file: %s", relPath)
				return nil
			}

			if verbose {
				sugar.Debugf("Including: %s", relPath)
			}

			if info.Mode().IsRegular() {
				totalSize += info.Size()
				filesChan <- &fileToProcess{
					path:    path,
					info:    info,
					relPath: relPath,
				}
			}

			// Progress update
			if time.Since(lastUpdate) > updateInterval {
				speed := float64(totalSize) / time.Since(startTime).Seconds() / (1024 * 1024)
				sugar.Infof("Archive size: %.2f MB (%.2f MB/s)", float64(totalSize)/(1024*1024), speed)
				lastUpdate = time.Now()
			}

			return nil
		})
		close(filesChan)
	}()

	// Wait for workers to finish
	wg.Wait()
	close(errorsChan)

	// Check for any errors
	for err := range errorsChan {
		if err != nil {
			return fmt.Errorf("error during archiving: %w", err)
		}
	}

	return nil
}

type fileToProcess struct {
	path    string
	info    os.FileInfo
	relPath string
}

// Helper function for adding files to zip
func addFileToZip(zipWriter *zip.Writer, path string, info os.FileInfo, relPath string) error {
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
			// Instead of returning error, log it and skip the file
			sugar.Warnf("Skipping file due to access denied: %s", path)
			return nil
		}
		defer file.Close()

		buf := bufferPool.Get().([]byte)
		defer bufferPool.Put(buf)

		_, err = io.CopyBuffer(writer, file, buf)
		if err != nil {
			// Log copy errors but don't fail the backup
			sugar.Warnf("Failed to copy file %s: %v", path, err)
			return nil
		}
	}

	return nil
}

// Add this helper function
func isExcluded(path string, excludePatterns []string) bool {
	// Convert Windows path to forward slashes for consistent matching
	normalizedPath := filepath.ToSlash(path)

	for _, pattern := range excludePatterns {
		// Convert pattern to use forward slashes
		normalizedPattern := filepath.ToSlash(pattern)

		// Check if the path starts with or matches the pattern
		if strings.HasPrefix(normalizedPath, normalizedPattern) ||
			strings.Contains(normalizedPath, "/"+normalizedPattern) {
			return true
		}

		// Try matching with wildcard patterns
		if matched, _ := filepath.Match(normalizedPattern, normalizedPath); matched {
			return true
		}
	}
	return false
}
