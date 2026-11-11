package parser

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Document represents an ingested source document file.
type Document struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	SourcePath string    `json:"source_path"`
	FileType   string    `json:"file_type"`
	ByteSize   int64     `json:"byte_size"`
	TokenCount int       `json:"token_count"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// Chunk represents a segmented slice of a document.
type Chunk struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	DocumentTitle string    `json:"document_title,omitempty"`
	ChunkIndex    int       `json:"chunk_index"`
	Content       string    `json:"content"`
	TokenCount    int       `json:"token_count"`
	HeaderContext string    `json:"header_context,omitempty"`
	Embedding     []float32 `json:"embedding,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ChunkerOptions specifies parameters for text segmentation.
type ChunkerOptions struct {
	TargetTokens  int
	OverlapTokens int
	MinTokens     int
}

// DefaultChunkerOptions provides standard defaults for semantic chunking.
func DefaultChunkerOptions() ChunkerOptions {
	return ChunkerOptions{
		TargetTokens:  200,
		OverlapTokens: 40,
		MinTokens:     20,
	}
}

// ComputeHash generates a deterministic SHA-256 hash for document content.
func ComputeHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// ApproximateTokens estimates token count from text using word and punctuation heuristics.
func ApproximateTokens(text string) int {
	words := len(text) / 4
	if words < 1 && len(text) > 0 {
		return 1
	}
	return words
}
