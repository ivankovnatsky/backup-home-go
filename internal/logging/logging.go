package logging

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	sugar        *zap.SugaredLogger
	logger       *zap.Logger
	loggerOnce   sync.Once
	currentLevel zap.AtomicLevel
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
)

// getColoredLevelEncoder returns a level encoder with colorized output
func getColoredLevelEncoder() zapcore.LevelEncoder {
	return func(l zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
		var levelStr string
		
		switch l {
		case zapcore.DebugLevel:
			levelStr = colorCyan + "DEBUG" + colorReset
		case zapcore.InfoLevel:
			levelStr = colorGreen + "INFO" + colorReset
		case zapcore.WarnLevel:
			levelStr = colorYellow + "WARN" + colorReset
		case zapcore.ErrorLevel:
			levelStr = colorRed + "ERROR" + colorReset
		case zapcore.DPanicLevel:
			levelStr = colorPurple + "DPANIC" + colorReset
		case zapcore.PanicLevel:
			levelStr = colorPurple + "PANIC" + colorReset
		case zapcore.FatalLevel:
			levelStr = colorRed + "FATAL" + colorReset
		default:
			levelStr = colorGray + l.String() + colorReset
		}
		
		enc.AppendString(levelStr)
	}
}

// InitLogger initializes the package-level logger with colored output
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
		
		// Configure custom encoder with colors
		config.EncoderConfig.EncodeLevel = getColoredLevelEncoder()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		
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
