package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
)

// Coordinator orchestrates multi-hop reasoning, hybrid retrieval, and grounded synthesis.
type Coordinator struct {
	store       *graph.Store
	embedder    vector.Embedder
	planner     *Planner
	synthesizer *Synthesizer
}

// NewCoordinator initializes the agent coordinator.
func NewCoordinator(store *graph.Store, embedder vector.Embedder, synthesizer *Synthesizer) *Coordinator {
	return &Coordinator{
		store:       store,
		embedder:    embedder,
		planner:     NewPlanner(store),
		synthesizer: synthesizer,
	}
}

// ExecuteQuery runs an agentic multi-hop query with step callbacks.
func (c *Coordinator) ExecuteQuery(ctx context.Context, req QueryRequest, onStep StepHandler) (*QueryResponse, error) {
	start := time.Now()

	emit := func(step, details string, data interface{}) {
		if onStep != nil {
			onStep(QueryStepEvent{
				Step:      step,
				Details:   details,
				Data:      data,
				Timestamp: time.Now().UTC(),
			})
		}
	}

	maxHops := req.MaxHops
	if maxHops <= 0 {
		maxHops = 3
	}
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}
	beamWidth := req.BeamWidth
	if beamWidth <= 0 {
		beamWidth = 5
	}

	// 1. Decompose query into sub-goals and seed entities
	emit("decomposing_query", "Analyzing query structure and candidate entities", nil)
	plan, err := c.planner.Decompose(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("decompose query: %w", err)
	}
	emit("seed_resolution", fmt.Sprintf("Resolved %d seed entities in knowledge graph", len(plan.SeedEntities)), plan.SeedEntities)

	// 2. Multi-Hop Graph Traversal via Bounded Beam Search
	var seedIDs []string
	for _, s := range plan.SeedEntities {
		seedIDs = append(seedIDs, s.ID)
	}

	emit("graph_traversal", fmt.Sprintf("Traversing graph up to %d hops (beam width %d)", maxHops, beamWidth), nil)
	traversalOpts := graph.TraversalOptions{
		MaxHops:   maxHops,
		BeamWidth: beamWidth,
		MinScore:  0.02,
	}
	paths, err := c.store.BoundedBeamSearch(seedIDs, req.Query, traversalOpts)
	if err != nil {
		return nil, fmt.Errorf("beam search traversal: %w", err)
	}
	emit("paths_discovered", fmt.Sprintf("Discovered %d multi-hop reasoning paths", len(paths)), paths)

	// Collect graph evidence chunks
	var graphChunkIDs []string
	seenGraphChunks := make(map[string]bool)
	for _, p := range paths {
		for _, st := range p.Steps {
			if st.Edge.EvidenceChunkID != "" && !seenGraphChunks[st.Edge.EvidenceChunkID] {
				seenGraphChunks[st.Edge.EvidenceChunkID] = true
				graphChunkIDs = append(graphChunkIDs, st.Edge.EvidenceChunkID)
			}
		}
	}

	// 3. Dense Vector Similarity Search
	emit("vector_search", "Computing dense vector query embedding and cosine similarity", nil)
	queryVec, err := c.embedder.EmbedText(ctx, req.Query)
	var vectorChunkIDs []string
	if err == nil && len(queryVec) > 0 {
		scoredItems, err := c.store.SearchChunksVector(queryVec, topK*2)
		if err == nil {
			for _, item := range scoredItems {
				vectorChunkIDs = append(vectorChunkIDs, item.ID)
			}
		}
	}

	// 4. BM25 Full-Text Search
	emit("bm25_search", "Executing BM25 keyword matching via SQLite FTS5", nil)
	bm25ChunkIDs, _ := c.store.SearchChunksBM25(req.Query, topK*2)

	// 5. Hybrid Reciprocal Rank Fusion (RRF)
	emit("rank_fusion", "Merging vector, BM25, and graph evidence via Reciprocal Rank Fusion", nil)
	rrfResults := vector.ReciprocalRankFusion(vectorChunkIDs, bm25ChunkIDs, graphChunkIDs, vector.DefaultRRFWeights())

	// Select Top K chunks
	var retrievedChunks []*parser.Chunk
	var citations []CitationRef
	chunkLimit := topK
	if len(rrfResults) < chunkLimit {
		chunkLimit = len(rrfResults)
	}

	for i := 0; i < chunkLimit; i++ {
		rrfItem := rrfResults[i]
		cObj, err := c.store.GetChunk(rrfItem.ID)
		if err == nil && cObj != nil {
			retrievedChunks = append(retrievedChunks, cObj)
			citations = append(citations, CitationRef{
				Index:         i + 1,
				ChunkID:       cObj.ID,
				DocumentID:    cObj.DocumentID,
				DocumentTitle: cObj.DocumentTitle,
				HeaderContext: cObj.HeaderContext,
				Content:       cObj.Content,
				Relevance:     rrfItem.RRFScore,
			})
		}
	}

	// 6. Context Synthesis & Anti-Hallucination Guard
	emit("synthesis", "Generating grounded answer with citation provenance", nil)
	sCtx := &SynthesisContext{
		Query:  req.Query,
		Paths:  paths,
		Chunks: retrievedChunks,
	}

	answer, risk, err := c.synthesizer.SynthesizeAnswer(ctx, sCtx)
	if err != nil {
		return nil, fmt.Errorf("synthesize answer: %w", err)
	}

	elapsed := time.Since(start).Milliseconds()
	emit("complete", fmt.Sprintf("Query completed in %d ms with hallucination risk %.2f", elapsed, risk), nil)

	return &QueryResponse{
		Query:             req.Query,
		Answer:            answer,
		SubGoals:          plan.SubGoals,
		SeedEntities:      plan.SeedEntities,
		ReasoningPaths:    paths,
		Citations:         citations,
		ExecutionTimeMs:   elapsed,
		HallucinationRisk: risk,
	}, nil
}
