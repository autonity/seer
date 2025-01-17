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

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull a range of blocks and process events",
	Run: func(cmd *cobra.Command, args []string) {
		var cfg config.Config
		if err := viper.Unmarshal(&cfg); err != nil {
			log.Fatalf("Failed to unmarshal config: %v", err)
		}

		startBlock, _ := cmd.Flags().GetUint64("start-block")
		endBlock, _ := cmd.Flags().GetUint64("end-block")

		if endBlock <= startBlock {
			log.Fatalf("Invalid block range: startBlock=%d, endBlock=%d", startBlock, endBlock)
		}

		handler := db.NewHandler(cfg.InfluxDB)
		parser := schema.NewABIParser(cfg.ABIs, handler)
		err := parser.Start()
		if err != nil {
			slog.Error("Error parsing ", "error ", err)
			return
		}

		rpcPool := net.NewConnectionPool[*rpc.Client](cfg.Node.RPC.URLs, cfg.Node.RPC.MaxConnections)
		wsPool := net.NewConnectionPool[*ethclient.Client](cfg.Node.WS.URLs, cfg.Node.WS.MaxConnections)
		cp := net.NewConnectionProvider(wsPool, rpcPool)

		l := core.New(cfg.Node, parser, handler, cp)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		l.ProcessRange(ctx, startBlock, endBlock)
		l.Stop()
	},
}

func init() {
	pullCmd.Flags().Uint64("start-block", 0, "Start block for processing")
	pullCmd.Flags().Uint64("end-block", 0, "End block for processing")
	rootCmd.AddCommand(pullCmd)
}
