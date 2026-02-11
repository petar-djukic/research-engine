package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/pdiddy/research-engine/internal/acquire"
	"github.com/pdiddy/research-engine/pkg/types"
)

const (
	defaultTimeout   = 60 * time.Second
	defaultDelay     = 1 * time.Second
	defaultUserAgent = "research-engine/0.1"
)

var acquireCmd = &cobra.Command{
	Use:   "acquire [identifiers...]",
	Short: "Download papers from URLs, DOIs, or arXiv IDs",
	Long: `Acquire resolves paper identifiers (arXiv IDs, DOIs, direct PDF URLs)
to PDF files, downloads them, and creates metadata records. Existing papers
are skipped.`,
	RunE: runAcquire,
}

func init() {
	acquireCmd.Flags().Duration("timeout", 0, "HTTP request timeout (default 60s)")
	acquireCmd.Flags().Duration("delay", 0, "delay between consecutive downloads (default 1s)")
	acquireCmd.Flags().String("papers-dir", "papers", "base directory for papers")

	rootCmd.AddCommand(acquireCmd)
}

func runAcquire(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("provide one or more paper identifiers (arXiv IDs, DOIs, or URLs)")
	}

	timeout, _ := cmd.Flags().GetDuration("timeout")
	if timeout == 0 {
		timeout = defaultTimeout
	}
	delay, _ := cmd.Flags().GetDuration("delay")
	if delay == 0 {
		delay = defaultDelay
	}
	papersDir, _ := cmd.Flags().GetString("papers-dir")

	cfg := types.AcquisitionConfig{
		HTTPConfig: types.HTTPConfig{
			Timeout:   timeout,
			UserAgent: defaultUserAgent,
		},
		DownloadDelay: delay,
		PapersDir:     papersDir,
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
	}

	result := acquire.AcquireBatch(client, args, cfg, os.Stdout)
	if result.HasFailures() {
		return fmt.Errorf("%d paper(s) failed acquisition", result.Failed)
	}
	return nil
}
