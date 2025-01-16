package cli

import (
	"context"
	"log"
	"log/slog"

	"github.com/autonity/autonity/ethclient"
	"github.com/autonity/autonity/rpc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"seer/config"
	"seer/core"
	"seer/db"
	"seer/net"
	"seer/schema"
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
	rpcPool := net.NewConnectionPool[*rpc.Client](cfg.Node.RPC, 10)
	wsPool := net.NewConnectionPool[*ethclient.Client](cfg.Node.WS, 10)
	cp := net.NewConnectionProvider(wsPool, rpcPool)
	l := core.New(cfg.Node, parser, handler, cp)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	l.Start(ctx)
	l.Stop()
}
