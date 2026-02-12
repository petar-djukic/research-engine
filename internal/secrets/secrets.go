// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

// Package secrets loads API keys and credentials from a directory of plain-text files.
// Each file in the directory represents one secret: the filename is the key name and the
// file contents (trimmed) are the value.
//
// Supported key files: patentsview-api-key, semantic-scholar-api-key, anthropic-api-key, openalex-email.
package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Load reads all files in dir and returns a map of filename to trimmed contents.
// A missing directory or missing files are not errors; Load returns an empty map.
// Unreadable files produce a warning on stderr but do not abort.
func Load(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("reading secrets directory %s: %w", dir, err)
	}

	secrets := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not read secret %s: %v\n", name, err)
			continue
		}

		value := strings.TrimSpace(string(data))
		if value != "" {
			secrets[name] = value
		}
	}

	return secrets, nil
}
