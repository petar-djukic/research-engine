// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package convert

import (
	"bytes"
	"fmt"
	"os"

	"github.com/pdiddy/research-engine/internal/container"
)

const imageMarkitdown = "markitdown:latest"

// MarkitdownConverter converts PDFs by piping them through the markitdown
// container image. It depends on a container.Runtime (docker or podman)
// injected at construction time.
type MarkitdownConverter struct {
	runtime container.Runtime
}

// NewMarkitdownConverter creates a converter that uses the given container
// runtime to run the markitdown image. It verifies that the markitdown image
// exists locally before returning.
func NewMarkitdownConverter(rt container.Runtime) (*MarkitdownConverter, error) {
	if err := rt.ImageExists(imageMarkitdown); err != nil {
		return nil, fmt.Errorf("markitdown image not available in %s: %w", rt.Name(), err)
	}
	return &MarkitdownConverter{runtime: rt}, nil
}

// Convert reads the PDF at pdfPath, pipes it through the markitdown container,
// and returns the resulting Markdown text.
func (m *MarkitdownConverter) Convert(pdfPath string) (string, error) {
	f, err := os.Open(pdfPath)
	if err != nil {
		return "", fmt.Errorf("opening PDF %s: %w", pdfPath, err)
	}
	defer f.Close()

	var out bytes.Buffer
	if err := m.runtime.Run(imageMarkitdown, f, &out); err != nil {
		return "", fmt.Errorf("converting %s with markitdown: %w", pdfPath, err)
	}

	if out.Len() == 0 {
		return "", fmt.Errorf("markitdown produced empty output for %s", pdfPath)
	}

	return out.String(), nil
}
