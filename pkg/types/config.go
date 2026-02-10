package types

import "time"

// HTTPConfig holds shared HTTP settings used by stages that make network requests.
type HTTPConfig struct {
	// Timeout is the HTTP request timeout.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// UserAgent is the User-Agent header sent with HTTP requests
	// (e.g. "research-engine/0.1"). Per prd001-acquisition R5.2, prd006-search R5.4.
	UserAgent string `json:"user_agent" yaml:"user_agent"`
}

// SearchConfig holds settings for the search stage.
// Per prd006-search R1.4, R2.3, R5.1-R5.6.
type SearchConfig struct {
	HTTPConfig `yaml:",inline"`

	// MaxResults is the maximum number of results to return (default 20).
	MaxResults int `json:"max_results" yaml:"max_results"`

	// EnableArxiv controls whether the arXiv backend is used.
	EnableArxiv bool `json:"enable_arxiv" yaml:"enable_arxiv"`

	// EnableSemanticScholar controls whether the Semantic Scholar backend is used.
	EnableSemanticScholar bool `json:"enable_semantic_scholar" yaml:"enable_semantic_scholar"`

	// SemanticScholarAPIKey is an optional API key for higher rate limits.
	SemanticScholarAPIKey string `json:"semantic_scholar_api_key,omitempty" yaml:"semantic_scholar_api_key,omitempty"`

	// InterBackendDelay is the delay between API calls to different backends (default 1s).
	InterBackendDelay time.Duration `json:"inter_backend_delay" yaml:"inter_backend_delay"`

	// RecencyBiasWindow is the time window for boosting recent papers (default 2 years).
	RecencyBiasWindow time.Duration `json:"recency_bias_window" yaml:"recency_bias_window"`
}

// AcquisitionConfig holds settings for the acquisition stage.
// Per prd001-acquisition R2.6, R5.1-R5.2.
type AcquisitionConfig struct {
	HTTPConfig `yaml:",inline"`

	// DownloadDelay is the delay between consecutive downloads (default 1s).
	DownloadDelay time.Duration `json:"download_delay" yaml:"download_delay"`

	// PapersDir is the base directory for papers (contains raw/, metadata/, markdown/).
	PapersDir string `json:"papers_dir" yaml:"papers_dir"`
}

// ConversionBackend identifies the PDF conversion tool.
// Per prd002-conversion R5.1.
type ConversionBackend string

const (
	BackendGROBID     ConversionBackend = "grobid"
	BackendPdftotext  ConversionBackend = "pdftotext"
	BackendMarkitdown ConversionBackend = "markitdown"
)

// ConversionConfig holds settings for the conversion stage.
// Per prd002-conversion R5.1-R5.2.
type ConversionConfig struct {
	// Backend selects the conversion tool: grobid, pdftotext, or markitdown.
	Backend ConversionBackend `json:"backend" yaml:"backend"`

	// PapersDir is the base directory for papers (contains raw/, metadata/, markdown/).
	PapersDir string `json:"papers_dir" yaml:"papers_dir"`
}

// AIConfig holds shared settings for stages that call a Generative AI API.
type AIConfig struct {
	// Model is the AI model identifier (e.g. "claude-sonnet-4-5-20250929").
	Model string `json:"model" yaml:"model"`

	// APIKey is the authentication key for the AI API.
	APIKey string `json:"api_key,omitempty" yaml:"api_key,omitempty"`

	// MaxRetries is the number of retry attempts for failed API calls (default 3).
	MaxRetries int `json:"max_retries" yaml:"max_retries"`
}

// ExtractionConfig holds settings for the extraction stage.
// Per prd003-extraction R5.2-R5.5.
type ExtractionConfig struct {
	AIConfig `yaml:",inline"`

	// PapersDir is the base directory for papers (contains markdown/).
	PapersDir string `json:"papers_dir" yaml:"papers_dir"`

	// KnowledgeDir is the base directory for knowledge output (contains extracted/).
	KnowledgeDir string `json:"knowledge_dir" yaml:"knowledge_dir"`
}

// KnowledgeBaseConfig holds settings for the knowledge base stage.
// Per prd004-knowledge-base R1.2, R2.3.
type KnowledgeBaseConfig struct {
	// KnowledgeDir is the base directory for knowledge (contains extracted/, index/).
	KnowledgeDir string `json:"knowledge_dir" yaml:"knowledge_dir"`

	// MaxResults is the default maximum number of query results (default 20).
	MaxResults int `json:"max_results" yaml:"max_results"`
}

// OutputFormat selects the generation output format.
// Per prd005-generation R6.1-R6.3.
type OutputFormat string

const (
	OutputMarkdown OutputFormat = "markdown"
	OutputLaTeX    OutputFormat = "latex"
)

// GenerationConfig holds settings for the generation stage.
// Per prd005-generation R3.1, R6.1-R6.3.
type GenerationConfig struct {
	AIConfig `yaml:",inline"`

	// OutputDir is the directory for generated drafts (e.g. "output/drafts/").
	OutputDir string `json:"output_dir" yaml:"output_dir"`

	// NotesDir is the directory for brainstorming notes (e.g. "output/notes/").
	NotesDir string `json:"notes_dir" yaml:"notes_dir"`

	// Format selects the output format: markdown or latex.
	Format OutputFormat `json:"format" yaml:"format"`
}

// PipelineConfig groups all stage configurations for the pipeline.
type PipelineConfig struct {
	Search       SearchConfig        `json:"search" yaml:"search"`
	Acquisition  AcquisitionConfig   `json:"acquisition" yaml:"acquisition"`
	Conversion   ConversionConfig    `json:"conversion" yaml:"conversion"`
	Extraction   ExtractionConfig    `json:"extraction" yaml:"extraction"`
	KnowledgeBase KnowledgeBaseConfig `json:"knowledge_base" yaml:"knowledge_base"`
	Generation   GenerationConfig    `json:"generation" yaml:"generation"`
}
