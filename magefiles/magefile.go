// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package main contains Mage build targets for research-engine developer tooling.
// Implements: docs/ARCHITECTURE § Developer Tooling, § Technology Choices (Pandoc).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pdiddy/research-engine/internal/draft"
)

// projectDirs lists the working directories the pipeline expects.
var projectDirs = []string{
	"papers/raw",
	"papers/markdown",
	"papers/metadata",
	"knowledge/extracted",
	"knowledge/index",
	"output/papers",
}

// Init creates the project directory structure for the pipeline.
func Init() error {
	for _, dir := range projectDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Println("  ", dir)
	}
	fmt.Println("Project directories initialized.")
	return nil
}

const (
	binDir  = "bin"
	binName = "research-engine"
	cmdPkg  = "./cmd/research-engine"
)

// Build compiles the CLI binary into bin/.
func Build() error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", binDir, err)
	}
	out := filepath.Join(binDir, binName)
	cmd := exec.Command("go", "build", "-tags", "sqlite_fts5", "-o", out, cmdPkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	fmt.Printf("Built %s\n", out)
	return nil
}

// Stats prints project metrics: Go production/test LOC and documentation word count.
func Stats() error {
	prodLines, err := countGoLines(".", false)
	if err != nil {
		return err
	}
	testLines, err := countGoLines(".", true)
	if err != nil {
		return err
	}
	docWords, err := countDocWords("docs")
	if err != nil {
		return err
	}

	fmt.Printf("Lines of code (Go, production): %d\n", prodLines)
	fmt.Printf("Lines of code (Go, tests):      %d\n", testLines)
	fmt.Printf("Words (documentation):           %d\n", docWords)
	return nil
}

// countGoLines walks the directory tree and counts non-blank lines in Go files.
// If testOnly is true, count only _test.go files; otherwise count non-test .go files.
func countGoLines(root string, testOnly bool) (int, error) {
	total := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		isTest := filepath.Ext(path) == ".go" && len(path) > 8 && path[len(path)-8:] == "_test.go"
		isGo := filepath.Ext(path) == ".go"
		if !isGo {
			return nil
		}
		if testOnly && !isTest {
			return nil
		}
		if !testOnly && isTest {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		for _, line := range splitLines(data) {
			if len(line) > 0 {
				total++
			}
		}
		return nil
	})
	return total, err
}

// countDocWords walks the docs directory and counts words in .md and .yaml files.
func countDocWords(root string) (int, error) {
	total := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".md" && ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		total += countWords(data)
		return nil
	})
	return total, err
}

// splitLines splits data by newline, returning each line as a trimmed string.
func splitLines(data []byte) []string {
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := trimSpace(data[start:i])
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(data) {
		line := trimSpace(data[start:])
		lines = append(lines, line)
	}
	return lines
}

// trimSpace returns a string with leading and trailing whitespace removed.
func trimSpace(b []byte) string {
	start, end := 0, len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\r') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\r') {
		end--
	}
	return string(b[start:end])
}

// countWords counts whitespace-separated tokens in data.
func countWords(data []byte) int {
	count := 0
	inWord := false
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			inWord = false
		} else if !inWord {
			inWord = true
			count++
		}
	}
	return count
}

const binPandoc = "pandoc"

// Compile produces a PDF from a paper project directory using pandoc.
// The project directory must contain numbered Markdown section files and
// optionally a references.yaml for citation support.
// Implements: prd007-paper-writing R6.4.
//
// Usage: mage compile output/papers/my-survey
func Compile(projectDir string) error {
	if projectDir == "" {
		return fmt.Errorf("project directory required: mage compile output/papers/my-survey")
	}

	// Collect numbered section files in order.
	inputPaths, err := draft.SectionFiles(projectDir)
	if err != nil {
		return err
	}
	if len(inputPaths) == 0 {
		return fmt.Errorf("no numbered section files (NN-*.md) found in %s", projectDir)
	}

	// Determine output PDF path.
	slug := filepath.Base(projectDir)
	outPDF := filepath.Join(projectDir, slug+".pdf")

	// Build pandoc arguments.
	args := []string{
		"--from=markdown",
		"--to=pdf",
		"-o", outPDF,
	}

	// Generate BibTeX from references.yaml if it exists.
	bibPath := filepath.Join(projectDir, slug+".bib")
	refs, err := draft.LoadReferences(projectDir)
	if err == nil && len(refs.Papers) > 0 {
		bibContent := draft.GenerateBibTeX(refs)
		if err := os.WriteFile(bibPath, []byte(bibContent), 0o644); err != nil {
			return fmt.Errorf("writing BibTeX: %w", err)
		}
		args = append(args, "--citeproc", "--bibliography="+bibPath)
		fmt.Printf("Generated %s from references.yaml\n", bibPath)
	}

	args = append(args, inputPaths...)

	cmd := exec.Command(binPandoc, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pandoc: %w", err)
	}

	fmt.Printf("Compiled %s\n", outPDF)
	return nil
}
