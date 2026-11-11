package graph

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/multihop/internal/parser"
)

func setupTestStore(t *testing.T) *Store {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_graph.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestStoreCRUDAndNeighbors(t *testing.T) {
	store := setupTestStore(t)

	// 1. Insert Document and Chunk
	doc := &parser.Document{
		ID:         "doc_01",
		Title:      "Merger Agreement",
		SourcePath: "/tmp/merger.md",
		FileType:   "markdown",
		ByteSize:   512,
		TokenCount: 120,
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.InsertDocument(doc); err != nil {
		t.Fatalf("insert doc error: %v", err)
	}

	chunk := &parser.Chunk{
		ID:         "chunk_01",
		DocumentID: "doc_01",
		ChunkIndex: 0,
		Content:    "Vendor Alpha acquired Subsidiary Beta in January 2022.",
		TokenCount: 8,
		Embedding:  []float32{0.5, 0.5, 0.5},
		CreatedAt:  time.Now().UTC(),
	}
	if err := store.InsertChunk(chunk); err != nil {
		t.Fatalf("insert chunk error: %v", err)
	}

	// 2. Insert Nodes
	n1 := &Node{
		ID:          "vendor_alpha",
		Name:        "Vendor Alpha",
		EntityType:  "ORGANIZATION",
		Description: "Primary technology contractor",
		CreatedAt:   time.Now().UTC(),
	}
	n2 := &Node{
		ID:          "subsidiary_beta",
		Name:        "Subsidiary Beta",
		EntityType:  "ORGANIZATION",
		Description: "European software division",
		CreatedAt:   time.Now().UTC(),
	}
	n3 := &Node{
		ID:          "patent_102",
		Name:        "Patent 102",
		EntityType:  "PATENT",
		Description: "Distributed consensus algorithm",
		CreatedAt:   time.Now().UTC(),
	}

	store.UpsertNode(n1)
	store.UpsertNode(n2)
	store.UpsertNode(n3)

	// 3. Insert Edges
	e1 := &Edge{
		ID:              "edge_1",
		SourceNodeID:    "vendor_alpha",
		TargetNodeID:    "subsidiary_beta",
		RelationType:    "OWNS",
		Weight:          1.0,
		EvidenceChunkID: "chunk_01",
		CreatedAt:       time.Now().UTC(),
	}
	e2 := &Edge{
		ID:              "edge_2",
		SourceNodeID:    "subsidiary_beta",
		TargetNodeID:    "patent_102",
		RelationType:    "HOLDS",
		Weight:          0.9,
		EvidenceChunkID: "chunk_01",
		CreatedAt:       time.Now().UTC(),
	}

	store.UpsertEdge(e1)
	store.UpsertEdge(e2)

	// 4. Test Neighbors
	neighbors, err := store.GetNeighbors("subsidiary_beta")
	if err != nil {
		t.Fatalf("get neighbors error: %v", err)
	}
	if len(neighbors) != 2 {
		t.Fatalf("expected 2 neighbors (1 incoming, 1 outgoing), got %d", len(neighbors))
	}

	// 5. Test Dijkstra Shortest Path
	path, err := store.DijkstraShortestPath("vendor_alpha", "patent_102")
	if err != nil {
		t.Fatalf("dijkstra path error: %v", err)
	}
	if len(path.Steps) != 2 {
		t.Fatalf("expected 2 hops from Vendor Alpha to Patent 102, got %d", len(path.Steps))
	}
	if path.Steps[0].TargetNode.ID != "subsidiary_beta" {
		t.Errorf("expected first hop target to be subsidiary_beta, got %s", path.Steps[0].TargetNode.ID)
	}

	// 6. Test Bounded Beam Search
	beamPaths, err := store.BoundedBeamSearch([]string{"vendor_alpha"}, "Patent 102 ownership", DefaultTraversalOptions())
	if err != nil {
		t.Fatalf("beam search error: %v", err)
	}
	if len(beamPaths) == 0 {
		t.Fatalf("expected beam paths, got 0")
	}

	// 7. Test Cypher-Lite
	cypherQuery := `MATCH (a:ORGANIZATION)-[r:OWNS]->(b:ORGANIZATION) WHERE a.name = 'Vendor Alpha' RETURN b`
	rows, err := store.ExecuteCypherLite(cypherQuery)
	if err != nil {
		t.Fatalf("cypher query error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 cypher match, got %d", len(rows))
	}
}

func TestDuplicateCandidatesAndMerge(t *testing.T) {
	store := setupTestStore(t)

	n1 := &Node{ID: "acme_corporation", Name: "Acme Corporation", EntityType: "ORGANIZATION", MentionCount: 3}
	n2 := &Node{ID: "acme_corp", Name: "Acme Corp", EntityType: "ORGANIZATION", MentionCount: 2}
	store.UpsertNode(n1)
	store.UpsertNode(n2)

	target := &Node{ID: "supplier_x", Name: "Supplier X", EntityType: "ORGANIZATION", MentionCount: 1}
	store.UpsertNode(target)

	store.UpsertEdge(&Edge{
		ID:           "e_old",
		SourceNodeID: "acme_corp",
		TargetNodeID: "supplier_x",
		RelationType: "CONTRACTS_WITH",
		Weight:       1.0,
	})

	candidates, err := store.FindDuplicateCandidates(2)
	if err != nil {
		t.Fatalf("find duplicates error: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatalf("expected duplicate candidate between Acme Corporation and Acme Corp")
	}

	// Merge acme_corp into acme_corporation
	if err := store.MergeNodes("acme_corporation", []string{"acme_corp"}); err != nil {
		t.Fatalf("merge nodes error: %v", err)
	}

	// Verify acme_corp deleted
	if _, err := store.GetNode("acme_corp"); err == nil {
		t.Errorf("expected acme_corp to be deleted after merge")
	}

	// Verify alias created
	mergedNode, err := store.FindNodeByNameOrAlias("Acme Corp")
	if err != nil || mergedNode.ID != "acme_corporation" {
		t.Errorf("expected alias lookup for 'Acme Corp' to resolve to 'acme_corporation'")
	}
}
