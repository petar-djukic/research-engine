// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package main is the entry point for the research-engine CLI.
// Implements: prd006-search, prd001-acquisition, prd002-conversion,
//             prd003-extraction, prd004-knowledge-base (CLI surface).
// See docs/ARCHITECTURE § Pipeline Interface, § Project Structure.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pdiddy/research-engine/internal/secrets"
)

// version is set at build time via ldflags.
var version = "dev"

// loadedSecrets holds API keys loaded from .secrets/ at startup.
var loadedSecrets map[string]string

// secretDefault returns the secret value for key if it exists, or fallback otherwise.
func secretDefault(key, fallback string) string {
	if fallback != "" {
		return fallback
	}
	if v, ok := loadedSecrets[key]; ok {
		return v
	}
	return ""
}

// rootCmd is the base command for the research-engine CLI.
var rootCmd = &cobra.Command{
	Use:   "research-engine",
	Short: "Infrastructure for Claude-driven academic research",
	Long: `research-engine provides infrastructure for academic paper research. Claude
drives the research workflow through skills; the CLI handles search, acquisition,
conversion, extraction, and knowledge base operations.

Each infrastructure stage is a subcommand: search, acquire, convert, extract,
and knowledge. Claude composes these into research workflows through
.claude/commands/ skills.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		s, err := secrets.Load(".secrets/")
		if err != nil {
			return err
		}
		loadedSecrets = s
		if len(s) > 0 {
			keys := make([]string, 0, len(s))
			for k := range s {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			fmt.Fprintf(os.Stderr, "Loaded secrets: %v\n", keys)
		}
		return nil
	},
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
