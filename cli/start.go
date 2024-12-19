package cli

import (
	"log"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"Seer/config"
)

var startCommand = &cobra.Command{
	Use:   "start",
	Short: "start seer",
	Run: func(cmd *cobra.Command, args []string) {
		//TODO: config being initialized in root and start commands (revise)
		var config config.Config
		if err := viper.Unmarshal(&config); err != nil {
			log.Fatalf("Failed to unmarshal config: %v", err)
		}
		slog.Info("starting seer")
		slog.Info("connecting to node", "rpc", config.Node.RPC)
		slog.Info("connecting to DB", "url", config.InfluxDB.URL)

	},
}

func init() {
	rootCmd.AddCommand(startCommand)
}
