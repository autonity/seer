package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version   string
	BuildTime string
)

var versionCommand = &cobra.Command{
	Use:   "version",
	Short: "seer version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("version:", Version)
		fmt.Println("build time:", BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCommand)
}
