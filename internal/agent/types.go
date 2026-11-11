package agent

import (
	"time"

	"github.com/alexandrmotologa/multihop/internal/graph"
)

// QueryStepEvent represents an execution milestone emitted during reasoning.
type QueryStepEvent struct {
	Step      string      `json:"step"`
	Details   string      `json:"details,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// CitationRef links a footnote in the answer to an exact source document and chunk.
type CitationRef struct {
	Index         int     `json:"index"`
	ChunkID       string  `json:"chunk_id"`
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	HeaderContext string  `json:"header_context,omitempty"`
	Content       string  `json:"content"`
	Relevance     float64 `json:"relevance"`
}

// QueryRequest configures the multi-hop reasoning search.
type QueryRequest struct {
	Query     string `json:"query"`
	MaxHops   int    `json:"max_hops,omitempty"`
	TopK      int    `json:"top_k,omitempty"`
	BeamWidth int    `json:"beam_width,omitempty"`
}

// QueryResponse delivers the complete grounded answer and multi-hop reasoning trace.
type QueryResponse struct {
	Query           string                `json:"query"`
	Answer          string                `json:"answer"`
	SubGoals        []string              `json:"sub_goals"`
	SeedEntities    []*graph.Node         `json:"seed_entities"`
	ReasoningPaths  []graph.ReasoningPath `json:"reasoning_paths"`
	Citations       []CitationRef         `json:"citations"`
	ExecutionTimeMs int64                 `json:"execution_time_ms"`
	HallucinationRisk float64             `json:"hallucination_risk"`
}

// StepHandler receives real-time progress callbacks during query execution.
type StepHandler func(event QueryStepEvent)
