// Copyright Mesh Intelligence Inc., 2026. All rights reserved.

package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
)

// extractionPromptTmpl is the prompt template sent to the Claude API for each
// section of Markdown. It instructs the model to extract typed knowledge items
// with provenance. Per prd003-extraction R5.2.
var extractionPromptTmpl = template.Must(template.New("extraction").Parse(`You are a research knowledge extraction system. Analyze the following section of an academic paper and extract typed knowledge items.

For each item, identify:
- type: one of "claim", "method", "definition", "result"
  - claim: a factual assertion or finding
  - method: a technique, algorithm, or procedure
  - definition: a term or concept being defined
  - result: a quantitative outcome, metric, or comparison
- content: the original text from the paper (preserve exact language, do not paraphrase)
- section: the section heading where the item appears
- page: the page number if available (0 if unknown)
- confidence: a float between 0.0 and 1.0 indicating how certain you are about the type classification and item boundaries
- tags: one or more lowercase, hyphenated topic labels drawn from the paper's vocabulary (e.g. "transformer", "attention-mechanism", "benchmark")

Respond with a JSON object containing an "items" array. Each element must have all fields listed above. Do not include any text outside the JSON object.

Example response:
{"items": [{"type": "claim", "content": "Attention mechanisms improve translation quality by 2 BLEU points.", "section": "Results", "page": 5, "confidence": 0.92, "tags": ["attention-mechanism", "machine-translation", "bleu"]}]}

Paper section:
{{.Section}}
`))

// claudeAPIURL is the Claude API endpoint. Package-level var for test substitution.
var claudeAPIURL = "https://api.anthropic.com/v1/messages"

// ClaudeBackend calls the Claude API to extract knowledge items from a section
// of Markdown. Per prd003-extraction R5.2.
type ClaudeBackend struct {
	APIKey string
	Model  string
	Client *http.Client
}

// claudeRequest is the request body for the Claude Messages API.
type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

// claudeMessage is a single message in the Claude API conversation.
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse is the response body from the Claude Messages API.
type claudeResponse struct {
	Content []claudeContent `json:"content"`
}

// claudeContent is a content block in the Claude API response.
type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Extract calls the Claude API with the extraction prompt for one section (R5.2).
func (c *ClaudeBackend) Extract(ctx context.Context, section string) (AIResponse, error) {
	prompt, err := renderPrompt(section)
	if err != nil {
		return AIResponse{}, fmt.Errorf("rendering prompt: %w", err)
	}

	reqBody := claudeRequest{
		Model:     c.Model,
		MaxTokens: 4096,
		Messages: []claudeMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return AIResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, claudeAPIURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return AIResponse{}, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return AIResponse{}, fmt.Errorf("calling Claude API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return AIResponse{}, fmt.Errorf("Claude API returned %d: %s", resp.StatusCode, string(body))
	}

	var cResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&cResp); err != nil {
		return AIResponse{}, fmt.Errorf("decoding Claude response: %w", err)
	}

	if len(cResp.Content) == 0 {
		return AIResponse{}, fmt.Errorf("Claude API returned empty content")
	}

	var aiResp AIResponse
	for _, block := range cResp.Content {
		if block.Type != "text" {
			continue
		}
		if err := json.Unmarshal([]byte(block.Text), &aiResp); err != nil {
			return AIResponse{}, fmt.Errorf("parsing AI response JSON: %w", err)
		}
		return aiResp, nil
	}

	return AIResponse{}, fmt.Errorf("no text content in Claude API response")
}

// renderPrompt executes the extraction prompt template with the given section.
func renderPrompt(section string) (string, error) {
	var buf bytes.Buffer
	if err := extractionPromptTmpl.Execute(&buf, struct{ Section string }{Section: section}); err != nil {
		return "", err
	}
	return buf.String(), nil
}
