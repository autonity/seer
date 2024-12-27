package main

import (
	"log/slog"
	"os"

	"github.com/spf13/viper"

	"Seer/cli"
)

var (
	version   string
	buildTime string
)

func initLogging() {
	logLevel := viper.GetString("logging.level")
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "error":
		level = slog.LevelError
	case "warn":
		level = slog.LevelWarn
	default:
		level = slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))
}

func main() {
	// setup logs
	initLogging()

	cli.Version = version
	cli.BuildTime = buildTime
	// start the cobra cli
	if err := cli.Execute(); err != nil {
		slog.Error("Unable to execute", "error", err)
	}
}
