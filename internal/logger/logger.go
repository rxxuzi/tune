package logger

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

// LogLevel defines the level of logging
type LogLevel int

const (
	FATAL LogLevel = iota
	ERROR
	WARN
	INFO
	DEBUG
	TRACE
)

var (
	level       LogLevel    = INFO
	ignoreLevel []LogLevel  = []LogLevel{}
	logger      *log.Logger = log.New(os.Stdout, "", 0)
	useColors   bool        = true
)

// ANSI color codes
const (
	Reset  = "\033[0m"
	Purple = "\033[35m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	White  = "\033[37m"
)

// initializeColors initializes ANSI color support on Windows
func initializeColors() {
	if runtime.GOOS == "windows" {
		stdout := windows.Handle(os.Stdout.Fd())
		var originalMode uint32
		windows.GetConsoleMode(stdout, &originalMode)
		windows.SetConsoleMode(stdout, originalMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
	}
}

// SetLevel sets the global log level
func SetLevel(l LogLevel) {
	level = l
}

// SetIgnoreLevel sets the log levels to ignore
func SetIgnoreLevel(ignore []LogLevel) {
	ignoreLevel = ignore
}

// DisableColors disables ANSI color codes
func DisableColors() {
	useColors = false
}

// Fatal logs a fatal message and exits the program
func Fatal(format string, args ...interface{}) {
	logMessage(FATAL, "FATAL", Purple, format, args...)
	os.Exit(1)
}

// Err logs an error message
func Err(format string, args ...interface{}) {
	logMessage(ERROR, "ERROR", Red, format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	logMessage(WARN, "WARN", Yellow, format, args...)
}

// Info logs an informational message
func Info(format string, args ...interface{}) {
	logMessage(INFO, "INFO", White, format, args...)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	logMessage(DEBUG, "DEBUG", White, format, args...)
}

// Trace logs a trace message
func Trace(format string, args ...interface{}) {
	logMessage(TRACE, "TRACE", White, format, args...)
}

// logMessage logs a message if the log level is appropriate and not ignored
func logMessage(lvl LogLevel, levelStr, color, format string, args ...interface{}) {
	if shouldIgnore(lvl) {
		return
	}
	if level >= lvl {
		message := fmt.Sprintf(format, args...)
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		if useColors {
			logger.Printf("%s[%s] [%s] %s%s", color, levelStr, timestamp, message, Reset)
		} else {
			logger.Printf("[%s] [%s] %s", levelStr, timestamp, message)
		}
	}
}

// shouldIgnore checks if the log level should be ignored
func shouldIgnore(lvl LogLevel) bool {
	for _, ignoreLvl := range ignoreLevel {
		if lvl == ignoreLvl {
			return true
		}
	}
	return false
}
