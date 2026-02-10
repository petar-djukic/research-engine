package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of research-engine",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("research-engine %s\n", version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
