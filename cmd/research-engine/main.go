// Package main is the entry point for the research-engine CLI.
// Implements: prd006-search, prd001-acquisition, prd002-conversion,
//             prd003-extraction, prd004-knowledge-base, prd005-generation (CLI surface).
// See docs/ARCHITECTURE § Pipeline Interface, § Project Structure.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// version is set at build time via ldflags.
var version = "dev"

// rootCmd is the base command for the research-engine CLI.
var rootCmd = &cobra.Command{
	Use:   "research-engine",
	Short: "A six-stage pipeline for academic paper research",
	Long: `research-engine transforms academic papers into a structured knowledge base
and uses that knowledge to generate new documents.

Each pipeline stage is a subcommand: search, acquire, convert, extract,
store, and generate. Run any stage independently or compose multi-stage
workflows through shell scripts or prompts.`,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().String("config", "", "config file (default: ./research-engine.yaml or ~/.config/research-engine/config.yaml)")
}

func initConfig() {
	cfgFile, _ := rootCmd.PersistentFlags().GetString("config")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("research-engine")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")

		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".config", "research-engine"))
		}
	}

	viper.SetEnvPrefix("RESEARCH_ENGINE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
