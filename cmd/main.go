package main

import (
	"log/slog"

	"seer/cli"
)

var (
	version   string
	buildTime string
)

func main() {
	cli.Version = version
	cli.BuildTime = buildTime
	// start the cobra cli
	if err := cli.Execute(); err != nil {
		slog.Error("Unable to execute", "error", err)
	}
}
