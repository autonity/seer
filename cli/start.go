package cli

import (
	"context"
	"log"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"Seer/config"
	"Seer/db"
	"Seer/listener"
	"Seer/schema"
)

var startCommand = &cobra.Command{
	Use:   "start",
	Short: "start seer",
	Run:   start,
}

func init() {
	rootCmd.AddCommand(startCommand)
}

func start(cmd *cobra.Command, args []string) {
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
	slog.Info("starting seer")

	handler := db.NewHandler(cfg.InfluxDB)
	parser := schema.NewABIParser(cfg.ABIs, handler)
	err := parser.Start()
	if err != nil {
		slog.Error("Error parsing ", "error ", err)
		return
	}
	l := listener.NewListener(cfg.Node, parser, handler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)
	l.Stop()
}
