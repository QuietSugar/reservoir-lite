package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reservoir/config"
)

var logLevel slog.LevelVar

func SetLogLevel(level slog.Level) {
	logLevel.Set(level)
}

func Init() {
	logToStdOut := config.Global.LogToStdout.Read()
	logFilePath := config.Global.LogFile.Read()

	logLevel.Set(config.Global.LogLevel.Read())

	// Subscribe to log level changes
	config.Global.LogLevel.OnChange(func(newLevel slog.Level) {
		logLevel.Set(newLevel)
		slog.Info("Log level changed by configuration", "new_level", newLevel)
	})

	slog.Info("Initializing logging...")

	slog.Info("Setting up log writers...")
	var writers []io.Writer = []io.Writer{}
	if logToStdOut {
		writers = append(writers, os.Stdout)
		slog.Info("Added Stdout writer")
	}

	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err == nil {
			f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				writers = append(writers, f)
				slog.Info("Added LogFile writer", "path", logFilePath)
			} else {
				slog.Error("Failed to open log file", "path", logFilePath, "error", err)
			}
		}
	}

	slog.Info("Setting up slog handler...")
	mw := io.MultiWriter(writers...)
	handler := slog.NewTextHandler(mw, &slog.HandlerOptions{
		Level: &logLevel,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("Slog handler and multi-writer set up successfully", "log-level", logLevel.String())
}
