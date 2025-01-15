// internal/logger/logger.go
package logger

import (
    "fmt"
    "os"
    "time"
    "path/filepath"
    "sync"
)

type Logger struct {
    logFile *os.File
    mu      sync.Mutex
}

type TimedOperation struct {
    StartTime time.Time
    Name      string
    logger    *Logger
}

func NewLogger(logPath string) (*Logger, error) {
    // Create logs directory if it doesn't exist
    logDir := filepath.Dir(logPath)
    if err := os.MkdirAll(logDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create log directory: %w", err)
    }

    // Open log file with append mode
    file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, fmt.Errorf("failed to open log file: %w", err)
    }

    return &Logger{
        logFile: file,
    }, nil
}

func (l *Logger) Close() error {
    return l.logFile.Close()
}

func (l *Logger) logEntry(level, message string, elapsed *time.Duration) {
    l.mu.Lock()
    defer l.mu.Unlock()

    timestamp := time.Now().Format("2006-01-02 15:04:05.000")
    var logMessage string
    
    if elapsed != nil {
        logMessage = fmt.Sprintf("[%s] %s: %s (took: %v)\n", timestamp, level, message, *elapsed)
    } else {
        logMessage = fmt.Sprintf("[%s] %s: %s\n", timestamp, level, message)
    }

    l.logFile.WriteString(logMessage)
    // Also print to stdout for development
    fmt.Print(logMessage)
}

func (l *Logger) Info(message string) {
    l.logEntry("INFO", message, nil)
}

func (l *Logger) Error(message string) {
    l.logEntry("ERROR", message, nil)
}

func (l *Logger) TimeOperation(name string) *TimedOperation {
    return &TimedOperation{
        StartTime: time.Now(),
        Name:      name,
        logger:    l,
    }
}

func (t *TimedOperation) End() time.Duration {
    elapsed := time.Since(t.StartTime)
    t.logger.logEntry("TIMING", fmt.Sprintf("%s completed", t.Name), &elapsed)
    return elapsed
}
