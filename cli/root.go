package cli

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "seer",
	Short: "Seer monitors on chain events and stores them in db",
	Long: "Seer listens to on-chain events, parses them and matches them against the known " +
		"contract abi schemas, and stores them in DB if matched to visualize later on grafana",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			return
		}
	},
}

func Execute() error {
	slog.Info("starting seer")
	err := rootCmd.Execute()
	if err != nil {
		return errors.New("unable to run root command")
	}
	return nil
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().String("config", "", "path to the configuration file")
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	//TODO: what else needs to be a cli flag
	rootCmd.PersistentFlags().String("node.rpc", "", "autonity node rpc url")
	_ = viper.BindPFlag("node.rpc", rootCmd.PersistentFlags().Lookup("node.rpc"))
	rootCmd.PersistentFlags().String("db.url", "", "influx db url")
	_ = viper.BindPFlag("db.url", rootCmd.PersistentFlags().Lookup("db.url"))
}

func initConfig() {
	configFile := viper.GetString("config")
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		fmt.Println("config key nil")
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("SEER")
	viper.AutomaticEnv()
	viper.SetDefault("seer.logLevel", "info")

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Failed to read config file: %v", err)
	}
	initLogging()
}

func initLogging() {
	logLevel := viper.GetString("seer.logLevel")
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
	slog.Info("Setting log level", "level", logLevel)
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))
}
