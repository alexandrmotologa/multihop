package graph

import (
	"time"
)

// Node represents an entity in the property graph.
type Node struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	EntityType   string    `json:"entity_type"`
	Description  string    `json:"description,omitempty"`
	MentionCount int       `json:"mention_count"`
	CommunityID  int       `json:"community_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	ID              string    `json:"id"`
	SourceNodeID    string    `json:"source"`
	TargetNodeID    string    `json:"target"`
	RelationType    string    `json:"relation_type"`
	Weight          float64   `json:"weight"`
	EvidenceChunkID string    `json:"evidence_chunk_id,omitempty"`
	ValidFrom       string    `json:"valid_from,omitempty"`
	ValidTo         string    `json:"valid_to,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Community represents an entity cluster with a high-level summary.
type Community struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	NodeCount int       `json:"node_count"`
	CreatedAt time.Time `json:"created_at"`
}

// DuplicateCandidate represents two nodes that may represent the same entity.
type DuplicateCandidate struct {
	NodeA      *Node `json:"node_a"`
	NodeB      *Node `json:"node_b"`
	Similarity float64 `json:"similarity"`
	Reason     string  `json:"reason"`
}

// NodeAlias maps colloquial variations to the canonical node.
type NodeAlias struct {
	Alias           string `json:"alias"`
	CanonicalNodeID string `json:"canonical_node_id"`
}

// PathStep represents one hop in a graph reasoning path.
type PathStep struct {
	SourceNode   *Node   `json:"source_node"`
	Edge         *Edge   `json:"edge"`
	TargetNode   *Node   `json:"target_node"`
	EvidenceText string  `json:"evidence_text,omitempty"`
	Score        float64 `json:"score"`
}

// ReasoningPath represents an end-to-end multi-hop traversal trajectory.
type ReasoningPath struct {
	Steps       []PathStep `json:"steps"`
	TotalWeight float64    `json:"total_weight"`
	Score       float64    `json:"score"`
}

// GraphSnapshot holds a subgraph structure suitable for visualization.
type GraphSnapshot struct {
	Nodes []*Node `json:"nodes"`
	Links []*Edge `json:"links"`
}

// NeighborEdge pairs an edge with its connected neighbor node.
type NeighborEdge struct {
	Edge       *Edge
	Neighbor   *Node
	IsOutgoing bool
}
