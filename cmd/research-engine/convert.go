package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pdiddy/research-engine/internal/container"
	"github.com/pdiddy/research-engine/internal/convert"
)

var convertCmd = &cobra.Command{
	Use:   "convert [papers...]",
	Short: "Convert PDF files to structured Markdown",
	Long: `Convert transforms PDF files into structured Markdown that preserves
section hierarchy, paragraphs, and reference lists. Supports GROBID,
pdftotext, and markitdown (container-based) backends.`,
	RunE: runConvert,
}

func init() {
	convertCmd.Flags().String("backend", "markitdown", "conversion backend: grobid, pdftotext, or markitdown")
	convertCmd.Flags().String("papers-dir", "papers", "base directory for papers")
	convertCmd.Flags().Bool("batch", false, "process all unconverted papers in papers-dir")

	rootCmd.AddCommand(convertCmd)
}

func runConvert(cmd *cobra.Command, args []string) error {
	backend, _ := cmd.Flags().GetString("backend")
	papersDir, _ := cmd.Flags().GetString("papers-dir")
	batch, _ := cmd.Flags().GetBool("batch")

	converter, err := newConverter(backend)
	if err != nil {
		return err
	}

	var pdfPaths []string
	if batch {
		rawDir := filepath.Join(papersDir, "raw")
		entries, err := os.ReadDir(rawDir)
		if err != nil {
			return fmt.Errorf("reading %s: %w", rawDir, err)
		}
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".pdf" {
				pdfPaths = append(pdfPaths, filepath.Join(rawDir, e.Name()))
			}
		}
		if len(pdfPaths) == 0 {
			fmt.Fprintln(os.Stdout, "No PDF files found in", rawDir)
			return nil
		}
	} else {
		if len(args) == 0 {
			return fmt.Errorf("provide PDF paths as arguments or use --batch")
		}
		pdfPaths = args
	}

	result := convert.ConvertPaths(converter, pdfPaths, papersDir, os.Stdout)
	if result.HasFailures() {
		return fmt.Errorf("%d paper(s) failed conversion", result.Failed)
	}
	return nil
}

func newConverter(backend string) (convert.Converter, error) {
	switch backend {
	case "markitdown":
		rt, err := container.DetectRuntime()
		if err != nil {
			return nil, fmt.Errorf("markitdown backend requires a container runtime: %w", err)
		}
		return convert.NewMarkitdownConverter(rt)
	default:
		return nil, fmt.Errorf("unsupported backend: %s (available: markitdown)", backend)
	}
}
