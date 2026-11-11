package graph

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"
)

// TraversalOptions configures the graph traversal limits and scoring.
type TraversalOptions struct {
	MaxHops   int     // Maximum depth (default 3)
	BeamWidth int     // Max paths kept at each frontier step (default 5)
	MinScore  float64 // Minimum path score threshold
}

// DefaultTraversalOptions provides balanced traversal limits.
func DefaultTraversalOptions() TraversalOptions {
	return TraversalOptions{
		MaxHops:   3,
		BeamWidth: 5,
		MinScore:  0.05,
	}
}

// BoundedBeamSearch executes a beam search starting from seed nodes.
// Prunes unpromising paths at each step to maintain strict memory bounds.
func (s *Store) BoundedBeamSearch(seedIDs []string, query string, opts TraversalOptions) ([]ReasoningPath, error) {
	if opts.MaxHops <= 0 {
		opts.MaxHops = 3
	}
	if opts.BeamWidth <= 0 {
		opts.BeamWidth = 5
	}

	type beamPath struct {
		currentID string
		steps     []PathStep
		score     float64
		visited   map[string]bool
	}

	var currentBeam []beamPath

	for _, seedID := range seedIDs {
		node, err := s.GetNode(seedID)
		if err != nil {
			continue
		}
		visited := make(map[string]bool)
		visited[node.ID] = true
		currentBeam = append(currentBeam, beamPath{
			currentID: node.ID,
			steps:     nil,
			score:     1.0,
			visited:   visited,
		})
	}

	if len(currentBeam) == 0 {
		return nil, nil
	}

	var allCompletedPaths []ReasoningPath

	for hop := 1; hop <= opts.MaxHops; hop++ {
		var candidates []beamPath

		for _, p := range currentBeam {
			neighbors, err := s.GetNeighbors(p.currentID)
			if err != nil {
				continue
			}

			for _, n := range neighbors {
				// Avoid cycles
				if p.visited[n.Neighbor.ID] {
					continue
				}

				// Copy visited set
				newVisited := make(map[string]bool, len(p.visited)+1)
				for k, v := range p.visited {
					newVisited[k] = v
				}
				newVisited[n.Neighbor.ID] = true

				// Retrieve evidence snippet if present
				evidenceText := ""
				if n.Edge.EvidenceChunkID != "" {
					if c, err := s.GetChunk(n.Edge.EvidenceChunkID); err == nil && c != nil {
						evidenceText = c.Content
					}
				}

				// Calculate step score
				// Depth penalty: 1 / (1 + ln(hop))
				depthPenalty := 1.0 / (1.0 + math.Log(float64(hop)))
				stepScore := n.Edge.Weight * depthPenalty

				// Relevance boost if node name or evidence appears in query
				qLower := strings.ToLower(query)
				if strings.Contains(qLower, strings.ToLower(n.Neighbor.Name)) {
					stepScore *= 1.5
				}

				cumScore := p.score * stepScore
				if cumScore < opts.MinScore {
					continue
				}

				currNode, _ := s.GetNode(p.currentID)
				step := PathStep{
					SourceNode:   currNode,
					Edge:         n.Edge,
					TargetNode:   n.Neighbor,
					EvidenceText: evidenceText,
					Score:        stepScore,
				}

				newSteps := make([]PathStep, len(p.steps)+1)
				copy(newSteps, p.steps)
				newSteps[len(p.steps)] = step

				candidates = append(candidates, beamPath{
					currentID: n.Neighbor.ID,
					steps:     newSteps,
					score:     cumScore,
					visited:   newVisited,
				})
			}
		}

		if len(candidates) == 0 {
			break
		}

		// Sort candidates by score descending and prune to BeamWidth
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})

		if len(candidates) > opts.BeamWidth {
			candidates = candidates[:opts.BeamWidth]
		}

		currentBeam = candidates

		// Save valid paths
		for _, c := range currentBeam {
			if len(c.steps) > 0 {
				var totalWeight float64
				for _, st := range c.steps {
					totalWeight += st.Edge.Weight
				}
				allCompletedPaths = append(allCompletedPaths, ReasoningPath{
					Steps:       c.steps,
					TotalWeight: totalWeight,
					Score:       c.score,
				})
			}
		}
	}

	sort.Slice(allCompletedPaths, func(i, j int) bool {
		return allCompletedPaths[i].Score > allCompletedPaths[j].Score
	})

	if len(allCompletedPaths) > opts.BeamWidth*2 {
		allCompletedPaths = allCompletedPaths[:opts.BeamWidth*2]
	}

	return allCompletedPaths, nil
}

type dijkstraItem struct {
	nodeID string
	cost   float64
	index  int
}

// DijkstraShortestPath finds the minimal cost reasoning chain connecting sourceID and targetID.
func (s *Store) DijkstraShortestPath(sourceID, targetID string) (*ReasoningPath, error) {
	if sourceID == targetID {
		node, err := s.GetNode(sourceID)
		if err != nil {
			return nil, err
		}
		return &ReasoningPath{
			Steps: []PathStep{{
				SourceNode: node,
				TargetNode: node,
				Score:      1.0,
			}},
			Score: 1.0,
		}, nil
	}

	dist := make(map[string]float64)
	prev := make(map[string]*NeighborEdge)
	visited := make(map[string]bool)

	dist[sourceID] = 0.0

	// Min-heap priority queue
	pq := &dijkstraPQ{}
	heap.Init(pq)
	heap.Push(pq, &dijkstraItem{nodeID: sourceID, cost: 0.0})

	for pq.Len() > 0 {
		curr := heap.Pop(pq).(*dijkstraItem)
		currID := curr.nodeID

		if visited[currID] {
			continue
		}
		visited[currID] = true

		if currID == targetID {
			break
		}

		neighbors, err := s.GetNeighbors(currID)
		if err != nil {
			continue
		}

		for i := range neighbors {
			n := neighbors[i]
			neighborID := n.Neighbor.ID
			if visited[neighborID] {
				continue
			}

			// Edge weight is confidence: edge cost = 1.0 / weight
			weight := n.Edge.Weight
			if weight <= 0 {
				weight = 0.01
			}
			edgeCost := 1.0 / weight

			newDist := dist[currID] + edgeCost
			oldDist, exists := dist[neighborID]
			if !exists || newDist < oldDist {
				dist[neighborID] = newDist
				prev[neighborID] = &n
				heap.Push(pq, &dijkstraItem{nodeID: neighborID, cost: newDist})
			}
		}
	}

	if _, found := dist[targetID]; !found {
		return nil, fmt.Errorf("no path found between '%s' and '%s'", sourceID, targetID)
	}

	// Reconstruct path backward
	var steps []PathStep
	currID := targetID

	for currID != sourceID {
		incomingEdge := prev[currID]
		if incomingEdge == nil {
			break
		}

		var srcNode, tgtNode *Node
		if incomingEdge.IsOutgoing {
			srcNode, _ = s.GetNode(incomingEdge.Edge.SourceNodeID)
			tgtNode, _ = s.GetNode(incomingEdge.Edge.TargetNodeID)
		} else {
			srcNode, _ = s.GetNode(incomingEdge.Edge.TargetNodeID)
			tgtNode, _ = s.GetNode(incomingEdge.Edge.SourceNodeID)
		}

		evidenceText := ""
		if incomingEdge.Edge.EvidenceChunkID != "" {
			if c, err := s.GetChunk(incomingEdge.Edge.EvidenceChunkID); err == nil && c != nil {
				evidenceText = c.Content
			}
		}

		step := PathStep{
			SourceNode:   srcNode,
			Edge:         incomingEdge.Edge,
			TargetNode:   tgtNode,
			EvidenceText: evidenceText,
			Score:        incomingEdge.Edge.Weight,
		}
		steps = append([]PathStep{step}, steps...)

		if incomingEdge.IsOutgoing {
			currID = incomingEdge.Edge.SourceNodeID
		} else {
			currID = incomingEdge.Edge.TargetNodeID
		}
	}

	totalWeight := 0.0
	for _, st := range steps {
		totalWeight += st.Edge.Weight
	}

	return &ReasoningPath{
		Steps:       steps,
		TotalWeight: totalWeight,
		Score:       1.0 / (1.0 + dist[targetID]),
	}, nil
}

type dijkstraPQ []*dijkstraItem

func (pq dijkstraPQ) Len() int           { return len(pq) }
func (pq dijkstraPQ) Less(i, j int) bool { return pq[i].cost < pq[j].cost }
func (pq dijkstraPQ) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }
func (pq *dijkstraPQ) Push(x interface{}) {
	item := x.(*dijkstraItem)
	*pq = append(*pq, item)
}
func (pq *dijkstraPQ) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// DetectCommunitiesLabelPropagation groups nodes into topical clusters using Label Propagation Algorithm.
func (s *Store) DetectCommunitiesLabelPropagation(maxIterations int) (map[int][]*Node, error) {
	if maxIterations <= 0 {
		maxIterations = 10
	}

	nodes, err := s.ListNodes(5000, "")
	if err != nil {
		return nil, err
	}

	labels := make(map[string]int)
	for i, n := range nodes {
		labels[n.ID] = i + 1
	}

	for iter := 0; iter < maxIterations; iter++ {
		changed := 0

		for _, n := range nodes {
			neighbors, err := s.GetNeighbors(n.ID)
			if err != nil || len(neighbors) == 0 {
				continue
			}

			// Frequency of labels among neighbors
			labelWeights := make(map[int]float64)
			for _, neigh := range neighbors {
				neighLabel := labels[neigh.Neighbor.ID]
				labelWeights[neighLabel] += neigh.Edge.Weight
			}

			bestLabel := labels[n.ID]
			maxW := 0.0
			for l, w := range labelWeights {
				if w > maxW {
					maxW = w
					bestLabel = l
				}
			}

			if bestLabel != labels[n.ID] {
				labels[n.ID] = bestLabel
				changed++
			}
		}

		if changed == 0 {
			break
		}
	}

	// Update community IDs in SQLite
	s.mu.Lock()
	tx, _ := s.db.Begin()
	stmt, _ := tx.Prepare(`UPDATE nodes SET community_id = ? WHERE id = ?`)
	for nodeID, commID := range labels {
		stmt.Exec(commID, nodeID)
	}
	stmt.Close()
	tx.Commit()
	s.mu.Unlock()

	communities := make(map[int][]*Node)
	for _, n := range nodes {
		cID := labels[n.ID]
		n.CommunityID = cID
		communities[cID] = append(communities[cID], n)
	}

	return communities, nil
}
