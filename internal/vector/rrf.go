package vector

import (
	"sort"
)

// ChannelWeight configures relative priority for each retrieval channel.
type RRFWeights struct {
	VectorWeight float64
	BM25Weight   float64
	GraphWeight  float64
	ConstantK    float64
}

// DefaultRRFWeights provides standard balanced weights for MultiHop.
func DefaultRRFWeights() RRFWeights {
	return RRFWeights{
		VectorWeight: 1.0,
		BM25Weight:   0.8,
		GraphWeight:  1.2,
		ConstantK:    60.0,
	}
}

// RankedItem represents an entity or chunk with its channel ranking.
type RankedItem struct {
	ID        string
	RRFScore  float64
	VectorPos int
	BM25Pos   int
	GraphPos  int
}

// ReciprocalRankFusion merges three ranked item lists into a single consolidated ranking.
// Items appearing near the top across multiple channels receive the highest composite score.
func ReciprocalRankFusion(vectorList []string, bm25List []string, graphList []string, weights RRFWeights) []RankedItem {
	if weights.ConstantK <= 0 {
		weights.ConstantK = 60.0
	}

	scoreMap := make(map[string]*RankedItem)

	getItem := func(id string) *RankedItem {
		if item, exists := scoreMap[id]; exists {
			return item
		}
		item := &RankedItem{
			ID:        id,
			VectorPos: -1,
			BM25Pos:   -1,
			GraphPos:  -1,
		}
		scoreMap[id] = item
		return item
	}

	// 1. Vector Channel
	for rank, id := range vectorList {
		item := getItem(id)
		item.VectorPos = rank + 1
		item.RRFScore += weights.VectorWeight / (weights.ConstantK + float64(rank+1))
	}

	// 2. BM25 Channel
	for rank, id := range bm25List {
		item := getItem(id)
		item.BM25Pos = rank + 1
		item.RRFScore += weights.BM25Weight / (weights.ConstantK + float64(rank+1))
	}

	// 3. Graph Evidence Channel
	for rank, id := range graphList {
		item := getItem(id)
		item.GraphPos = rank + 1
		item.RRFScore += weights.GraphWeight / (weights.ConstantK + float64(rank+1))
	}

	results := make([]RankedItem, 0, len(scoreMap))
	for _, item := range scoreMap {
		results = append(results, *item)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].RRFScore > results[j].RRFScore
	})

	return results
}
