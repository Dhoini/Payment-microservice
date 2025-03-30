package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
)

// Color codes for console output
// LogLevel defines the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// Color codes for console output
const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	purple = "\033[35m"
)

// Logger is a custom logging structure
type Logger struct {
	mu       sync.Mutex
	level    LogLevel
	output   io.Writer
	tracking bool
}

// New creates a new Logger instance
func New(level LogLevel) *Logger {
	return &Logger{
		level:    level,
		output:   os.Stdout,
		tracking: true,
	}
}

// getCallerInfo retrieves file and line of the caller
func getCallerInfo(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???", 0
	}

	// Trim the full path to just the last few path components
	parts := strings.Split(file, "/")
	if len(parts) > 3 {
		file = strings.Join(parts[len(parts)-3:], "/")
	}

	return file, line
}

// colorForLevel returns the color based on log level
func colorForLevel(level LogLevel) string {
	switch level {
	case DEBUG:
		return blue
	case INFO:
		return green
	case WARN:
		return yellow
	case ERROR:
		return red
	case FATAL:
		return purple
	default:
		return reset
	}
}

// log writes a formatted log message
func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	// Skip 2 stack frames to get the correct caller
	file, line := getCallerInfo(2)

	// Prepare log message
	msg := fmt.Sprintf(format, v...)

	// Get color for level
	color := colorForLevel(level)

	// Construct log entry
	logEntry := fmt.Sprintf("%s[%s]%s %s:%d - %s\n",
		color,
		strings.ToUpper([]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[level]),
		reset,
		file,
		line,
		msg,
	)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprint(l.output, logEntry)

	// Handle fatal level
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(FATAL, format, v...)
}

// Example usage
/*
func main() {
	// Create a new logger with DEBUG level
	logger := logger.New(logger.DEBUG)

	logger.Debug("This is a debug message")
	logger.Info("Application started with version %d", 1)
	logger.Warn("Potential performance issue detected")
	logger.Error("Failed to connect to database: %v", err)

	// This will log and then exit the program
	// logger.Fatal("Critical error occurred")
}
*/
