package backup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"backup-home/internal/platform"
)

func createMacOSArchive(source, backupPath string, compressionLevel int) error {
	// Build the tar command with explicit path to macOS tar
	tarCmd := exec.Command("/usr/bin/tar",
		"--strip-components=2",
		"-cvf",
		"-")

	// Add exclude patterns
	excludePatterns := platform.GetExcludePatterns()
	for _, pattern := range excludePatterns {
		tarCmd.Args = append(tarCmd.Args, "--exclude", pattern)
	}

	// Add source last
	tarCmd.Args = append(tarCmd.Args, source)

	// Debug: Print the command and its arguments
	fmt.Printf("Executing command: /usr/bin/tar %s\n", strings.Join(tarCmd.Args[1:], " "))

	// Set up the pipeline
	tarCmd.Stderr = os.Stderr
	tarStdout, err := tarCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create tar stdout pipe: %w", err)
	}

	// Start tar
	if err := tarCmd.Start(); err != nil {
		return fmt.Errorf("failed to start tar: %w", err)
	}

	// Set up pigz command
	pigzCmd := exec.Command("pigz",
		"-c",
		fmt.Sprintf("-%d", compressionLevel))
	pigzCmd.Stdin = tarStdout
	pigzCmd.Stderr = os.Stderr

	// Create output file
	outFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	pigzCmd.Stdout = outFile

	// Start pigz
	if err := pigzCmd.Start(); err != nil {
		return fmt.Errorf("failed to start pigz: %w", err)
	}

	// Wait for both commands to complete
	if err := tarCmd.Wait(); err != nil {
		return fmt.Errorf("tar command failed: %w", err)
	}
	if err := pigzCmd.Wait(); err != nil {
		return fmt.Errorf("pigz command failed: %w", err)
	}

	return nil
}
