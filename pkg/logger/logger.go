package logger

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
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

func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	file, line := getCallerInfo(4)
	msg := fmt.Sprintf(format, v...)
	color := colorForLevel(level)
	timestamp := time.Now().Format("2006-01-02 15:04:05") // Добавляем временную метку

	logEntry := fmt.Sprintf("%s[%s]%s %s %s:%d - %s\n",
		color,
		strings.ToUpper([]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[level]),
		reset,
		timestamp, // Добавляем временную метку
		file,
		line,
		msg,
	)

	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprint(l.output, logEntry)

	if level == FATAL {
		os.Exit(1)
	}
}

// Debugw logs a debug message
func (l *Logger) Debugw(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

// Infow logs an info message
func (l *Logger) Infow(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

// Warnw logs a warning message
func (l *Logger) Warnw(format string, v ...interface{}) {
	l.log(WARN, format, v...)
}

// Errorw logs an error message
func (l *Logger) Errorw(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

// Fatalw logs a fatal message and exits
func (l *Logger) Fatalw(format string, v ...interface{}) {
	l.log(FATAL, format, v...)
}
