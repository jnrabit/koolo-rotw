package utils

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// LogRotationConfig holds configuration for log file rotation
type LogRotationConfig struct {
	MaxFileSize int64 // Maximum size of log file in bytes (default: 100MB)
	MaxBackups  int   // Maximum number of backup files to keep (default: 5)
	MaxAgeDays  int   // Maximum age of log files in days (default: 30)
	Compress    bool  // Whether to compress rotated logs (default: true)
}

// RotateLogsIfNeeded checks if log file needs rotation and performs it
func RotateLogsIfNeeded(logPath string, config *LogRotationConfig) error {
	if config == nil {
		config = &LogRotationConfig{
			MaxFileSize: 104857600, // 100MB
			MaxBackups:  5,
			MaxAgeDays:  30,
			Compress:    true,
		}
	}

	fileInfo, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, nothing to rotate
		}
		return fmt.Errorf("failed to check log file size: %w", err)
	}

	// Check if file exceeds max size
	if fileInfo.Size() < config.MaxFileSize {
		return nil // File is still within limits
	}

	// Rotate the log file
	return rotateLogFile(logPath, config)
}

// rotateLogFile performs the actual log rotation
func rotateLogFile(logPath string, config *LogRotationConfig) error {
	dir := filepath.Dir(logPath)
	filename := filepath.Base(logPath)
	ext := filepath.Ext(filename)
	base := filename[:len(filename)-len(ext)]

	// Create timestamped backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupName := fmt.Sprintf("%s.%s%s", base, timestamp, ext)
	backupPath := filepath.Join(dir, backupName)

	// Rename current log file
	if err := os.Rename(logPath, backupPath); err != nil {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	slog.Info("log file rotated", "from", logPath, "to", backupPath)

	// Clean up old backups
	if err := cleanupOldLogs(dir, base, ext, config); err != nil {
		slog.Warn("failed to cleanup old logs", "error", err)
	}

	return nil
}

// cleanupOldLogs removes old log files exceeding retention policy
func cleanupOldLogs(dir, base, ext string, config *LogRotationConfig) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	now := time.Now()
	pattern := base + "."
	deletedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !isLogFile(name, pattern, ext) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check if file is too old
		age := now.Sub(info.ModTime()).Hours() / 24
		if age > float64(config.MaxAgeDays) {
			filePath := filepath.Join(dir, name)
			if err := os.Remove(filePath); err != nil {
				slog.Warn("failed to delete old log", "file", name, "error", err)
				continue
			}
			deletedCount++
		}
	}

	if deletedCount > 0 {
		slog.Debug("cleaned up old log files", "count", deletedCount)
	}

	return nil
}

// isLogFile checks if a filename matches the log file pattern
func isLogFile(name, pattern, ext string) bool {
	return len(name) > len(pattern)+len(ext) &&
		name[:len(pattern)] == pattern &&
		name[len(name)-len(ext):] == ext
}
