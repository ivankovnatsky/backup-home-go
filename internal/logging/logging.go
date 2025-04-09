package logging

import (
	"sync"

	"go.uber.org/zap"
)

var (
	sugar        *zap.SugaredLogger
	logger       *zap.Logger
	loggerOnce   sync.Once
	currentLevel zap.AtomicLevel
)

// InitLogger initializes the package-level logger
func InitLogger(verbose bool) error {
	var err error
	loggerOnce.Do(func() {
		// Create a user-friendly console logger configuration
		config := zap.NewDevelopmentConfig()
		currentLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
		
		// Set the log level based on verbose flag
		if verbose {
			currentLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
		}
		
		config.Level = currentLevel
		// Use the same user-friendly format for both modes
		logger, err = config.Build()
		if err != nil {
			return
		}
		sugar = logger.Sugar()
	})
	
	// If logger is already initialized but verbose flag changed,
	// update the level dynamically
	if logger != nil && verbose && currentLevel.Level() != zap.DebugLevel {
		currentLevel.SetLevel(zap.DebugLevel)
	} else if logger != nil && !verbose && currentLevel.Level() != zap.InfoLevel {
		currentLevel.SetLevel(zap.InfoLevel)
	}
	
	return err
}

// GetLogger returns the package-level logger
func GetLogger() *zap.Logger {
	return logger
}

// GetSugar returns the sugared logger
func GetSugar() *zap.SugaredLogger {
	return sugar
}

// SyncLogger flushes any buffered log entries
func SyncLogger() {
	if logger != nil {
		_ = logger.Sync() // ignoring sync error as it's expected during shutdown
	}
}