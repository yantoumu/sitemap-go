package logger

import (
	"sync"
)

var (
	globalLogger *Logger
	once         sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	once.Do(func() {
		// Initialize with default config if not already set
		if globalLogger == nil {
			globalLogger = New(Config{
				Level:      "info",
				Format:     "json",
				Output:     "stdout",
				TimeFormat: "",
			})
		}
	})
	return globalLogger
}

// SetLogger sets the global logger instance
func SetLogger(logger *Logger) {
	globalLogger = logger
}

// Debug logs a debug message
func Debug(msg string) {
	GetLogger().Debug(msg)
}

// Info logs an info message
func Info(msg string) {
	GetLogger().Info(msg)
}

// Warn logs a warning message
func Warn(msg string) {
	GetLogger().Warn(msg)
}

// Error logs an error message
func Error(msg string) {
	GetLogger().Error(msg)
}

// Fatal logs a fatal message and exits
func Fatal(msg string) {
	GetLogger().Fatal(msg)
}

// WithField adds a field to the logger
func WithField(key string, value interface{}) *Logger {
	return GetLogger().WithField(key, value)
}

// WithFields adds multiple fields to the logger
func WithFields(fields map[string]interface{}) *Logger {
	return GetLogger().WithFields(fields)
}

// WithError adds an error to the logger
func WithError(err error) *Logger {
	return GetLogger().WithError(err)
}