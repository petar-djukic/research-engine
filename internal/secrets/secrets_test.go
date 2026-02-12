// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package secrets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(t *testing.T) string
		want   map[string]string
		errMsg string
	}{
		{
			name: "reads key files and trims whitespace",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				writeFile(t, dir, "patentsview-api-key", "  pk_abc123  \n")
				writeFile(t, dir, "semantic-scholar-api-key", "sk_xyz789")
				writeFile(t, dir, "openalex-email", "user@example.com\n")
				return dir
			},
			want: map[string]string{
				"patentsview-api-key":       "pk_abc123",
				"semantic-scholar-api-key":  "sk_xyz789",
				"openalex-email":            "user@example.com",
			},
		},
		{
			name: "returns empty map for nonexistent directory",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			want: map[string]string{},
		},
		{
			name: "skips empty files",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				writeFile(t, dir, "anthropic-api-key", "valid-key")
				writeFile(t, dir, "empty-key", "")
				writeFile(t, dir, "whitespace-only", "   \n\t  ")
				return dir
			},
			want: map[string]string{
				"anthropic-api-key": "valid-key",
			},
		},
		{
			name: "skips dotfiles",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				writeFile(t, dir, ".gitkeep", "")
				writeFile(t, dir, ".hidden-key", "secret")
				writeFile(t, dir, "patentsview-api-key", "pk_real")
				return dir
			},
			want: map[string]string{
				"patentsview-api-key": "pk_real",
			},
		},
		{
			name: "skips subdirectories",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				writeFile(t, dir, "anthropic-api-key", "ak_123")
				require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))
				return dir
			},
			want: map[string]string{
				"anthropic-api-key": "ak_123",
			},
		},
		{
			name: "returns empty map for empty directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			got, err := Load(dir)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "good-key", "value123")

	// Create a file then remove read permission.
	badPath := filepath.Join(dir, "bad-key")
	require.NoError(t, os.WriteFile(badPath, []byte("secret"), 0o000))
	t.Cleanup(func() { os.Chmod(badPath, 0o644) })

	got, err := Load(dir)
	require.NoError(t, err)
	// The good file should still be returned; the bad file is skipped with a warning.
	assert.Equal(t, "value123", got["good-key"])
	_, hasBad := got["bad-key"]
	assert.False(t, hasBad, "unreadable file should not appear in result")
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
}
