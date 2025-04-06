// internal/logger/logger.go
package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// Log levels
	INFO  = "INFO"
	DEBUG = "DEBUG"
	ERROR = "ERROR"

	// Default log file
	DefaultLogFile = "hf-lms-sync.log"
)

// Logger is the central logging facility for the application
type Logger struct {
	Verbose      bool
	fileLogger   *log.Logger
	consoleLogger *log.Logger
	file         *os.File
	mu           sync.Mutex // Ensures thread-safety for logging
}

// New creates a new logger instance
func New(verbose bool) (*Logger, error) {
	logger := &Logger{
		Verbose: verbose,
	}

	// Always set up console logger
	logger.consoleLogger = log.New(os.Stdout, "", 0)

	// Only set up file logger if verbose mode is enabled
	if verbose {
		// Try to open the log file in append mode, or create it if it doesn't exist
		file, err := os.OpenFile(DefaultLogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %v", err)
		}
		logger.file = file
		logger.fileLogger = log.New(file, "", 0)

		// Log the application start - only to file in verbose mode
		logEntry := formatLogEntry(INFO, "LOGGER", "Application started with verbose logging")
		logger.fileLogger.Println(logEntry)
	}

	return logger, nil
}

// Close closes the log file if it's open
func (l *Logger) Close() error {
	if l.Verbose && l.file != nil {
		logEntry := formatLogEntry(INFO, "LOGGER", "Application shutting down")
		if l.fileLogger != nil {
			l.fileLogger.Println(logEntry)
		}
		return l.file.Close()
	}
	return nil
}

// formatLogEntry formats a log entry with timestamp, level, component, and message
func formatLogEntry(level, component, format string, v ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, v...)
	return fmt.Sprintf("[%s] [%s] [%s] %s", timestamp, level, component, message)
}

// Info logs an informational message
func (l *Logger) Info(component, format string, v ...interface{}) {
	if l.Verbose {
		l.mu.Lock()
		defer l.mu.Unlock()
		logEntry := formatLogEntry(INFO, component, format, v...)
		if l.fileLogger != nil {
			l.fileLogger.Println(logEntry)
		}
		// Do not log to console to avoid messing up the UI
	}
}

// Debug logs a debug message
func (l *Logger) Debug(component, format string, v ...interface{}) {
	if l.Verbose {
		l.mu.Lock()
		defer l.mu.Unlock()
		logEntry := formatLogEntry(DEBUG, component, format, v...)
		if l.fileLogger != nil {
			l.fileLogger.Println(logEntry)
		}
		// Debug messages only go to the file, not console
	}
}

// Error logs an error message
func (l *Logger) Error(component, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	logEntry := formatLogEntry(ERROR, component, format, v...)
	
	// Only log to file in verbose mode to avoid disrupting UI
	if l.Verbose && l.fileLogger != nil {
		l.fileLogger.Println(logEntry)
	} else if !l.Verbose {
		// Only log to console if not in verbose mode
		l.consoleLogger.Println(logEntry)
	}
}

// GetLogPath returns the absolute path to the log file
func GetLogPath() string {
	absPath, _ := filepath.Abs(DefaultLogFile)
	return absPath
}
