package extractor

import (
	"context"

	"github.com/alexandrmotologa/multihop/internal/parser"
)

// ExtractedEntity represents an entity recognized in text.
type ExtractedEntity struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// ExtractedRelation represents a directed relationship recognized between two entities.
type ExtractedRelation struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	Type     string  `json:"type"`
	Weight   float64 `json:"weight"`
	Evidence string  `json:"evidence,omitempty"`
}

// ExtractionResult aggregates entities and relations extracted from a chunk.
type ExtractionResult struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// Extractor defines the interface for entity and relation extraction from text chunks.
type Extractor interface {
	Extract(ctx context.Context, chunk *parser.Chunk) (*ExtractionResult, error)
	Name() string
}
