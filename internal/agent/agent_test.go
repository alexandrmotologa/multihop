package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
)

func TestCoordinatorMultiHopFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "agent_test.db")
	store, err := graph.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	defer store.Close()

	embedder := vector.NewBuiltinEmbedder(384)
	synthesizer := NewSynthesizer("builtin", "", "", "")
	coord := NewCoordinator(store, embedder, synthesizer)

	// Ingest sample data:
	// Vendor Alpha owns Subsidiary Beta.
	// Subsidiary Beta holds Patent 9821.
	doc := &parser.Document{
		ID:         "doc_tech",
		Title:      "IP Assignment Agreement",
		SourcePath: "/tmp/ip.md",
		FileType:   "markdown",
		ByteSize:   256,
		TokenCount: 50,
		CreatedAt:  time.Now().UTC(),
	}
	store.InsertDocument(doc)

	text1 := "Vendor Alpha is the corporate parent of Subsidiary Beta."
	emb1, _ := embedder.EmbedText(context.Background(), text1)
	store.InsertChunk(&parser.Chunk{
		ID:         "chunk_1",
		DocumentID: "doc_tech",
		ChunkIndex: 0,
		Content:    text1,
		TokenCount: 9,
		Embedding:  emb1,
		CreatedAt:  time.Now().UTC(),
	})

	text2 := "Subsidiary Beta holds Patent 9821 for distributed indexing."
	emb2, _ := embedder.EmbedText(context.Background(), text2)
	store.InsertChunk(&parser.Chunk{
		ID:         "chunk_2",
		DocumentID: "doc_tech",
		ChunkIndex: 1,
		Content:    text2,
		TokenCount: 8,
		Embedding:  emb2,
		CreatedAt:  time.Now().UTC(),
	})

	store.UpsertNode(&graph.Node{ID: "vendor_alpha", Name: "Vendor Alpha", EntityType: "ORGANIZATION"})
	store.UpsertNode(&graph.Node{ID: "subsidiary_beta", Name: "Subsidiary Beta", EntityType: "ORGANIZATION"})
	store.UpsertNode(&graph.Node{ID: "patent_9821", Name: "Patent 9821", EntityType: "PATENT"})

	store.UpsertEdge(&graph.Edge{
		ID:              "e1",
		SourceNodeID:    "vendor_alpha",
		TargetNodeID:    "subsidiary_beta",
		RelationType:    "OWNS",
		Weight:          1.0,
		EvidenceChunkID: "chunk_1",
	})
	store.UpsertEdge(&graph.Edge{
		ID:              "e2",
		SourceNodeID:    "subsidiary_beta",
		TargetNodeID:    "patent_9821",
		RelationType:    "HOLDS",
		Weight:          1.0,
		EvidenceChunkID: "chunk_2",
	})

	var emittedSteps []string
	stepHandler := func(ev QueryStepEvent) {
		emittedSteps = append(emittedSteps, ev.Step)
	}

	req := QueryRequest{
		Query:   "What patent is held by subsidiaries of Vendor Alpha?",
		MaxHops: 3,
		TopK:    3,
	}

	resp, err := coord.ExecuteQuery(context.Background(), req, stepHandler)
	if err != nil {
		t.Fatalf("execute query error: %v", err)
	}

	if len(resp.ReasoningPaths) == 0 {
		t.Fatalf("expected discovered reasoning paths, got 0")
	}

	if len(resp.Citations) == 0 {
		t.Fatalf("expected citations, got 0")
	}

	if resp.Answer == "" {
		t.Fatalf("expected non-empty answer")
	}

	if len(emittedSteps) < 5 {
		t.Errorf("expected at least 5 reasoning steps emitted, got %d", len(emittedSteps))
	}

	// Verify RAG Triad scores
	if resp.ContextRelevance <= 0 || resp.Groundedness <= 0 || resp.AnswerRelevance <= 0 {
		t.Errorf("expected positive RAG Triad scores, got Context: %f, Groundedness: %f, Answer: %f",
			resp.ContextRelevance, resp.Groundedness, resp.AnswerRelevance)
	}

	// Test Global Search
	globalReq := QueryRequest{
		Query: "Summarize all main entities and clusters across the entire corpus",
		Mode:  "global",
	}
	globalResp, err := coord.ExecuteQuery(context.Background(), globalReq, nil)
	if err != nil {
		t.Fatalf("global query error: %v", err)
	}
	if globalResp.Mode != "global" {
		t.Errorf("expected mode 'global', got '%s'", globalResp.Mode)
	}
	if len(globalResp.Communities) == 0 {
		t.Errorf("expected community summaries in global response")
	}
}
