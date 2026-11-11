package extractor

import (
	"context"
	"testing"

	"github.com/alexandrmotologa/multihop/internal/parser"
)

func TestNormalizer(t *testing.T) {
	slug, aliases := NormalizeEntity("Acme Corporation, Inc.")
	if slug != "acme" {
		t.Errorf("expected slug 'acme', got '%s'", slug)
	}
	if len(aliases) == 0 {
		t.Errorf("expected aliases for Acme Corporation, got 0")
	}

	dist := LevenshteinDistance("kitten", "sitting")
	if dist != 3 {
		t.Errorf("expected Levenshtein distance 3, got %d", dist)
	}
}

func TestRuleExtractor(t *testing.T) {
	chunk := &parser.Chunk{
		ID: "c1",
		Content: "Vendor Alpha acquired Subsidiary Beta in 2021. " +
			"Subsidiary Beta holds Patent 9821. " +
			"Section 4.2 limits liability to $5,000,000.",
	}

	ext := NewRuleExtractor()
	res, err := ext.Extract(context.Background(), chunk)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}

	if len(res.Entities) == 0 {
		t.Fatalf("expected extracted entities, got 0")
	}

	foundPatent := false
	foundMoney := false
	for _, e := range res.Entities {
		if e.Type == "PATENT" {
			foundPatent = true
		}
		if e.Type == "MONEY" {
			foundMoney = true
		}
	}

	if !foundPatent {
		t.Errorf("expected PATENT entity to be recognized")
	}
	if !foundMoney {
		t.Errorf("expected MONEY entity to be recognized")
	}

	if len(res.Relations) == 0 {
		t.Fatalf("expected extracted relations, got 0")
	}

	foundAcquired := false
	for _, r := range res.Relations {
		if r.Type == "ACQUIRED" {
			foundAcquired = true
			if r.Source != "Vendor Alpha" || r.Target != "Subsidiary Beta" {
				t.Errorf("unexpected ACQUIRED relation: %+v", r)
			}
		}
	}

	if !foundAcquired {
		t.Errorf("expected ACQUIRED relation to be extracted")
	}
}
