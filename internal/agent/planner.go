package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/alexandrmotologa/multihop/internal/extractor"
	"github.com/alexandrmotologa/multihop/internal/graph"
)

// Planner decomposes user queries into seed entities and navigation subgoals.
type Planner struct {
	store *graph.Store
}

func NewPlanner(store *graph.Store) *Planner {
	return &Planner{store: store}
}

var (
	capitalizedPhraseRegex = regexp.MustCompile(`\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)\b`)
	quotedPhraseRegex      = regexp.MustCompile(`["']([^"']+)["']`)
)

// PlanDecomposition holds the analyzed subgoals and candidate seed entities.
type PlanDecomposition struct {
	SubGoals     []string
	SeedEntities []*graph.Node
}

// Decompose analyzes the query to identify seed nodes and traversal intentions.
func (p *Planner) Decompose(ctx context.Context, query string) (*PlanDecomposition, error) {
	var candidateNames []string

	// 1. Check for quoted terms
	for _, m := range quotedPhraseRegex.FindAllStringSubmatch(query, -1) {
		if len(m) > 1 {
			candidateNames = append(candidateNames, m[1])
		}
	}

	// 2. Check for capitalized noun phrases
	for _, m := range capitalizedPhraseRegex.FindAllString(query, -1) {
		clean := strings.TrimSpace(m)
		// Ignore common question starters
		if clean == "What" || clean == "Who" || clean == "Where" || clean == "Did" || clean == "How" || clean == "Why" || clean == "Is" || clean == "Are" {
			continue
		}
		candidateNames = append(candidateNames, clean)
	}

	// 3. Fallback: individual words in the query
	words := strings.Fields(query)
	for _, w := range words {
		wClean := strings.Trim(w, ",.?!:;\"'")
		if len(wClean) > 3 {
			candidateNames = append(candidateNames, wClean)
		}
	}

	// Resolve candidate names against SQLite store
	seen := make(map[string]bool)
	var seeds []*graph.Node

	for _, name := range candidateNames {
		node, err := p.store.FindNodeByNameOrAlias(name)
		if err == nil && node != nil {
			if !seen[node.ID] {
				seen[node.ID] = true
				seeds = append(seeds, node)
			}
		}
	}

	// If no exact seeds were found in graph, attempt fuzzy search
	if len(seeds) == 0 {
		allNodes, _ := p.store.ListNodes(200, "")
		for _, n := range allNodes {
			for _, c := range candidateNames {
				if extractor.LevenshteinDistance(strings.ToLower(n.Name), strings.ToLower(c)) <= 2 {
					if !seen[n.ID] {
						seen[n.ID] = true
						seeds = append(seeds, n)
					}
				}
			}
		}
	}

	// Generate sub-goals
	var subgoals []string
	if len(seeds) > 0 {
		subgoals = append(subgoals, "Identify relationships connected to "+seeds[0].Name)
	}
	if len(seeds) > 1 {
		subgoals = append(subgoals, "Find relational path connecting "+seeds[0].Name+" to "+seeds[1].Name)
	}
	subgoals = append(subgoals, "Retrieve supporting document chunks for factual verification")

	return &PlanDecomposition{
		SubGoals:     subgoals,
		SeedEntities: seeds,
	}, nil
}
