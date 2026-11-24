package logging

import (
	"io"
	"log/slog"
	"os"
	"reservoir/config"
)

var logLevel slog.LevelVar

func SetLogLevel(level slog.Level) {
	logLevel.Set(level)
}

func Init() {
	logToStdOut := config.Global.LogToStdout.Read()

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

	slog.Info("Setting up slog handler...")
	mw := io.MultiWriter(writers...)
	handler := slog.NewTextHandler(mw, &slog.HandlerOptions{
		Level: &logLevel,
	})
	slog.SetDefault(slog.New(handler))

	slog.Info("Slog handler and multi-writer set up successfully", "log-level", logLevel.String())
}
