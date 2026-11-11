package vector

import (
	"context"
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	v1 := []float32{1.0, 0.0, 0.0}
	v2 := []float32{1.0, 0.0, 0.0}
	sim := CosineSimilarity(v1, v2)
	if math.Abs(float64(sim-1.0)) > 1e-5 {
		t.Errorf("expected 1.0 for identical vectors, got %f", sim)
	}

	vOrthogonal := []float32{0.0, 1.0, 0.0}
	simOrth := CosineSimilarity(v1, vOrthogonal)
	if math.Abs(float64(simOrth)) > 1e-5 {
		t.Errorf("expected 0.0 for orthogonal vectors, got %f", simOrth)
	}

	vOpposite := []float32{-1.0, 0.0, 0.0}
	simOpp := CosineSimilarity(v1, vOpposite)
	if math.Abs(float64(simOpp - (-1.0))) > 1e-5 {
		t.Errorf("expected -1.0 for opposite vectors, got %f", simOpp)
	}
}

func TestEncodeDecodeVector(t *testing.T) {
	orig := []float32{0.123, -0.456, 0.789, 1.0}
	buf := EncodeVector(orig)
	if len(buf) != len(orig)*4 {
		t.Fatalf("expected buffer length %d, got %d", len(orig)*4, len(buf))
	}

	decoded, err := DecodeVector(buf)
	if err != nil {
		t.Fatalf("failed to decode vector: %v", err)
	}

	if len(decoded) != len(orig) {
		t.Fatalf("expected decoded length %d, got %d", len(orig), len(decoded))
	}

	for i := range orig {
		if orig[i] != decoded[i] {
			t.Errorf("at index %d: expected %f, got %f", i, orig[i], decoded[i])
		}
	}
}

func TestTopK(t *testing.T) {
	topk := NewTopK(3)
	topk.Add("doc1", 0.2)
	topk.Add("doc2", 0.9)
	topk.Add("doc3", 0.5)
	topk.Add("doc4", 0.8)
	topk.Add("doc5", 0.1)

	res := topk.Results()
	if len(res) != 3 {
		t.Fatalf("expected 3 results, got %d", len(res))
	}

	if res[0].ID != "doc2" || res[0].Score != 0.9 {
		t.Errorf("expected top doc to be doc2 with 0.9, got %s (%f)", res[0].ID, res[0].Score)
	}
	if res[1].ID != "doc4" || res[1].Score != 0.8 {
		t.Errorf("expected second doc to be doc4 with 0.8, got %s (%f)", res[1].ID, res[1].Score)
	}
	if res[2].ID != "doc3" || res[2].Score != 0.5 {
		t.Errorf("expected third doc to be doc3 with 0.5, got %s (%f)", res[2].ID, res[2].Score)
	}
}

func TestBuiltinEmbedder(t *testing.T) {
	ctx := context.Background()
	emb := NewBuiltinEmbedder(384)

	t1 := "Acme Corp acquires Patent 102 from subsidiary"
	t2 := "Acme Corp bought Patent 102 from its subsidiary"
	t3 := "The quick brown fox jumps over the lazy dog"

	v1, err := emb.EmbedText(ctx, t1)
	if err != nil {
		t.Fatalf("embed t1 error: %v", err)
	}
	v2, err := emb.EmbedText(ctx, t2)
	if err != nil {
		t.Fatalf("embed t2 error: %v", err)
	}
	v3, err := emb.EmbedText(ctx, t3)
	if err != nil {
		t.Fatalf("embed t3 error: %v", err)
	}

	simSemantic := CosineSimilarity(v1, v2)
	simUnrelated := CosineSimilarity(v1, v3)

	if simSemantic <= simUnrelated {
		t.Errorf("expected semantically related sentences to score higher (%f vs %f)", simSemantic, simUnrelated)
	}
}

func TestRRF(t *testing.T) {
	vecList := []string{"chunk_A", "chunk_B", "chunk_C"}
	bm25List := []string{"chunk_B", "chunk_D", "chunk_A"}
	graphList := []string{"chunk_A", "chunk_E", "chunk_B"}

	results := ReciprocalRankFusion(vecList, bm25List, graphList, DefaultRRFWeights())
	if len(results) == 0 {
		t.Fatalf("expected non-empty RRF results")
	}

	// chunk_A appears near top in all three channels, so it should rank first or second
	if results[0].ID != "chunk_A" && results[0].ID != "chunk_B" {
		t.Errorf("expected chunk_A or chunk_B as top ranked item, got %s", results[0].ID)
	}
}
