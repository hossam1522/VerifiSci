package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of VerifiSci",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("verifisci %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
