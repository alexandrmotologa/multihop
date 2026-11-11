package graph

import (
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store manages persistence of documents, chunks, nodes, and edges in SQLite.
type Store struct {
	db   *sql.DB
	path string
	mu   sync.RWMutex
}

// OpenStore opens or initializes a SQLite property graph database.
func OpenStore(dbPath string) (*Store, error) {
	// Enable WAL mode, cache size, foreign keys via DSN
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	// Optimize connection pool for embedded SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &Store{
		db:   db,
		path: dbPath,
	}

	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return store, nil
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queries := strings.Split(schemaSQL, ";")
	for _, q := range queries {
		trimmed := strings.TrimSpace(q)
		if trimmed == "" {
			continue
		}
		if _, err := s.db.Exec(trimmed); err != nil {
			return fmt.Errorf("exec migration (%s): %w", trimmed, err)
		}
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// InsertDocument persists a document in the database.
func (s *Store) InsertDocument(doc *parser.Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO documents (id, title, source_path, file_type, byte_size, token_count, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT(id) DO UPDATE SET
                title = excluded.title,
                byte_size = excluded.byte_size,
                token_count = excluded.token_count`

	_, err := s.db.Exec(query, doc.ID, doc.Title, doc.SourcePath, doc.FileType, doc.ByteSize, doc.TokenCount, doc.CreatedAt)
	return err
}

// GetDocument retrieves a document by ID.
func (s *Store) GetDocument(id string) (*parser.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`SELECT id, title, source_path, file_type, byte_size, token_count, created_at FROM documents WHERE id = ?`, id)
	var doc parser.Document
	var createdStr string
	if err := row.Scan(&doc.ID, &doc.Title, &doc.SourcePath, &doc.FileType, &doc.ByteSize, &doc.TokenCount, &createdStr); err != nil {
		return nil, err
	}
	doc.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	return &doc, nil
}

// ListDocuments returns all registered documents.
func (s *Store) ListDocuments() ([]*parser.Document, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, title, source_path, file_type, byte_size, token_count, created_at FROM documents ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []*parser.Document
	for rows.Next() {
		var doc parser.Document
		var createdStr string
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.SourcePath, &doc.FileType, &doc.ByteSize, &doc.TokenCount, &createdStr); err != nil {
			return nil, err
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

// InsertChunk writes a single chunk and updates the FTS5 index.
func (s *Store) InsertChunk(chunk *parser.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var embBytes []byte
	if len(chunk.Embedding) > 0 {
		embBytes = vector.EncodeVector(chunk.Embedding)
	}

	q1 := `INSERT INTO chunks (id, document_id, chunk_index, content, token_count, header_context, embedding, created_at)
           VALUES (?, ?, ?, ?, ?, ?, ?, ?)
           ON CONFLICT(id) DO UPDATE SET
             content = excluded.content,
             header_context = excluded.header_context,
             embedding = excluded.embedding`
	if _, err := tx.Exec(q1, chunk.ID, chunk.DocumentID, chunk.ChunkIndex, chunk.Content, chunk.TokenCount, chunk.HeaderContext, embBytes, chunk.CreatedAt); err != nil {
		return err
	}

	// Update FTS5
	tx.Exec(`DELETE FROM chunks_fts WHERE id = ?`, chunk.ID)
	q2 := `INSERT INTO chunks_fts (id, content) VALUES (?, ?)`
	if _, err := tx.Exec(q2, chunk.ID, chunk.Content); err != nil {
		return err
	}

	return tx.Commit()
}

// InsertChunks writes multiple chunks in a single transaction.
func (s *Store) InsertChunks(chunks []*parser.Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtChunk, err := tx.Prepare(`INSERT INTO chunks (id, document_id, chunk_index, content, token_count, header_context, embedding, created_at)
                                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)
                                  ON CONFLICT(id) DO UPDATE SET
                                    content = excluded.content,
                                    header_context = excluded.header_context,
                                    embedding = excluded.embedding`)
	if err != nil {
		return err
	}
	defer stmtChunk.Close()

	stmtFTSDel, _ := tx.Prepare(`DELETE FROM chunks_fts WHERE id = ?`)
	defer stmtFTSDel.Close()

	stmtFTS, err := tx.Prepare(`INSERT INTO chunks_fts (id, content) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmtFTS.Close()

	for _, c := range chunks {
		var embBytes []byte
		if len(c.Embedding) > 0 {
			embBytes = vector.EncodeVector(c.Embedding)
		}
		if _, err := stmtChunk.Exec(c.ID, c.DocumentID, c.ChunkIndex, c.Content, c.TokenCount, c.HeaderContext, embBytes, c.CreatedAt); err != nil {
			return err
		}

		stmtFTSDel.Exec(c.ID)
		if _, err := stmtFTS.Exec(c.ID, c.Content); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetChunk returns a chunk by ID.
func (s *Store) GetChunk(id string) (*parser.Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`SELECT c.id, c.document_id, d.title, c.chunk_index, c.content, c.token_count, c.header_context, c.embedding, c.created_at
                          FROM chunks c
                          LEFT JOIN documents d ON c.document_id = d.id
                          WHERE c.id = ?`, id)

	var c parser.Chunk
	var embBytes []byte
	var headerCtx sql.NullString
	var docTitle sql.NullString
	var createdStr string

	if err := row.Scan(&c.ID, &c.DocumentID, &docTitle, &c.ChunkIndex, &c.Content, &c.TokenCount, &headerCtx, &embBytes, &createdStr); err != nil {
		return nil, err
	}

	if headerCtx.Valid {
		c.HeaderContext = headerCtx.String
	}
	if docTitle.Valid {
		c.DocumentTitle = docTitle.String
	}
	if len(embBytes) > 0 {
		c.Embedding, _ = vector.DecodeVector(embBytes)
	}

	return &c, nil
}

// ListChunksByDocument lists all chunks belonging to a document.
func (s *Store) ListChunksByDocument(docID string) ([]*parser.Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, document_id, chunk_index, content, token_count, header_context, created_at
                             FROM chunks WHERE document_id = ? ORDER BY chunk_index ASC`, docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*parser.Chunk
	for rows.Next() {
		var c parser.Chunk
		var headerCtx sql.NullString
		var createdStr string
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkIndex, &c.Content, &c.TokenCount, &headerCtx, &createdStr); err != nil {
			return nil, err
		}
		if headerCtx.Valid {
			c.HeaderContext = headerCtx.String
		}
		chunks = append(chunks, &c)
	}
	return chunks, nil
}

// SearchChunksBM25 executes a full-text search over chunks using SQLite FTS5.
func (s *Store) SearchChunksBM25(query string, limit int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cleanQuery := strings.Map(func(r rune) rune {
		if strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 ", r) {
			return r
		}
		return ' '
	}, query)

	words := strings.Fields(cleanQuery)
	if len(words) == 0 {
		return nil, nil
	}

	matchClause := strings.Join(words, " OR ")
	rows, err := s.db.Query(`SELECT id FROM chunks_fts WHERE content MATCH ? ORDER BY rank LIMIT ?`, matchClause, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// SearchChunksVector performs linear exact cosine distance search over chunk embeddings.
func (s *Store) SearchChunksVector(queryVec []float32, limit int) ([]vector.ScoredItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, embedding FROM chunks WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topk := vector.NewTopK(limit)
	for rows.Next() {
		var id string
		var embBytes []byte
		if err := rows.Scan(&id, &embBytes); err != nil {
			continue
		}

		vec, err := vector.DecodeVector(embBytes)
		if err != nil {
			continue
		}

		sim := vector.CosineSimilarity(queryVec, vec)
		topk.Add(id, sim)
	}

	return topk.Results(), nil
}

// UpsertNode writes or updates an entity node in the graph.
func (s *Store) UpsertNode(node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO nodes (id, name, entity_type, description, mention_count, community_id, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT(id) DO UPDATE SET
                mention_count = nodes.mention_count + 1,
                description = COALESCE(excluded.description, nodes.description)`

	_, err := s.db.Exec(query, node.ID, node.Name, node.EntityType, node.Description, node.MentionCount, node.CommunityID, node.CreatedAt)
	return err
}

// GetNode retrieves a node by its ID.
func (s *Store) GetNode(id string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	row := s.db.QueryRow(`SELECT id, name, entity_type, description, mention_count, community_id, created_at FROM nodes WHERE id = ?`, id)
	var n Node
	var desc sql.NullString
	var createdStr string
	if err := row.Scan(&n.ID, &n.Name, &n.EntityType, &desc, &n.MentionCount, &n.CommunityID, &createdStr); err != nil {
		return nil, err
	}
	if desc.Valid {
		n.Description = desc.String
	}
	return &n, nil
}

// FindNodeByNameOrAlias searches for a node matching a name or known alias.
func (s *Store) FindNodeByNameOrAlias(name string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(name))

	// 1. Direct ID or Name match
	var nodeID string
	err := s.db.QueryRow(`SELECT id FROM nodes WHERE id = ? OR LOWER(name) = ? LIMIT 1`, norm, norm).Scan(&nodeID)
	if err == nil {
		return s.GetNode(nodeID)
	}

	// 2. Alias match
	err = s.db.QueryRow(`SELECT canonical_node_id FROM node_aliases WHERE LOWER(alias) = ? LIMIT 1`, norm).Scan(&nodeID)
	if err == nil {
		return s.GetNode(nodeID)
	}

	// 3. Substring / LIKE match
	err = s.db.QueryRow(`SELECT id FROM nodes WHERE LOWER(name) LIKE ? LIMIT 1`, "%"+norm+"%").Scan(&nodeID)
	if err == nil {
		return s.GetNode(nodeID)
	}

	return nil, sql.ErrNoRows
}

// ListNodes returns nodes optionally filtered by entity type.
func (s *Store) ListNodes(limit int, entityType string) ([]*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows *sql.Rows
	var err error

	if entityType != "" {
		rows, err = s.db.Query(`SELECT id, name, entity_type, description, mention_count, community_id, created_at
                                FROM nodes WHERE entity_type = ? ORDER BY mention_count DESC LIMIT ?`, entityType, limit)
	} else {
		rows, err = s.db.Query(`SELECT id, name, entity_type, description, mention_count, community_id, created_at
                                FROM nodes ORDER BY mention_count DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []*Node
	for rows.Next() {
		var n Node
		var desc sql.NullString
		var createdStr string
		if err := rows.Scan(&n.ID, &n.Name, &n.EntityType, &desc, &n.MentionCount, &n.CommunityID, &createdStr); err != nil {
			continue
		}
		if desc.Valid {
			n.Description = desc.String
		}
		nodes = append(nodes, &n)
	}
	return nodes, nil
}

// UpsertEdge records a directed relation between two nodes.
func (s *Store) UpsertEdge(edge *Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `INSERT INTO edges (id, source_node_id, target_node_id, relation_type, weight, evidence_chunk_id, created_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)
              ON CONFLICT(id) DO UPDATE SET
                weight = excluded.weight,
                evidence_chunk_id = COALESCE(excluded.evidence_chunk_id, edges.evidence_chunk_id)`

	var evidence sql.NullString
	if edge.EvidenceChunkID != "" {
		evidence.String = edge.EvidenceChunkID
		evidence.Valid = true
	}

	_, err := s.db.Exec(query, edge.ID, edge.SourceNodeID, edge.TargetNodeID, edge.RelationType, edge.Weight, evidence, edge.CreatedAt)
	return err
}

// GetNeighbors retrieves all outgoing and incoming connected edges and nodes for a given node.
func (s *Store) GetNeighbors(nodeID string) ([]NeighborEdge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Outgoing
	queryOut := `SELECT e.id, e.source_node_id, e.target_node_id, e.relation_type, e.weight, e.evidence_chunk_id, e.created_at,
                        n.id, n.name, n.entity_type, n.description, n.mention_count, n.community_id, n.created_at
                 FROM edges e
                 JOIN nodes n ON e.target_node_id = n.id
                 WHERE e.source_node_id = ?`

	rowsOut, err := s.db.Query(queryOut, nodeID)
	if err != nil {
		return nil, err
	}
	defer rowsOut.Close()

	var neighbors []NeighborEdge
	for rowsOut.Next() {
		var e Edge
		var n Node
		var evChunk sql.NullString
		var eCreated, nCreated string
		var nDesc sql.NullString

		if err := rowsOut.Scan(&e.ID, &e.SourceNodeID, &e.TargetNodeID, &e.RelationType, &e.Weight, &evChunk, &eCreated,
			&n.ID, &n.Name, &n.EntityType, &nDesc, &n.MentionCount, &n.CommunityID, &nCreated); err != nil {
			continue
		}
		if evChunk.Valid {
			e.EvidenceChunkID = evChunk.String
		}
		if nDesc.Valid {
			n.Description = nDesc.String
		}
		neighbors = append(neighbors, NeighborEdge{
			Edge:       &e,
			Neighbor:   &n,
			IsOutgoing: true,
		})
	}

	// Incoming
	queryIn := `SELECT e.id, e.source_node_id, e.target_node_id, e.relation_type, e.weight, e.evidence_chunk_id, e.created_at,
                       n.id, n.name, n.entity_type, n.description, n.mention_count, n.community_id, n.created_at
                FROM edges e
                JOIN nodes n ON e.source_node_id = n.id
                WHERE e.target_node_id = ?`

	rowsIn, err := s.db.Query(queryIn, nodeID)
	if err != nil {
		return nil, err
	}
	defer rowsIn.Close()

	for rowsIn.Next() {
		var e Edge
		var n Node
		var evChunk sql.NullString
		var eCreated, nCreated string
		var nDesc sql.NullString

		if err := rowsIn.Scan(&e.ID, &e.SourceNodeID, &e.TargetNodeID, &e.RelationType, &e.Weight, &evChunk, &eCreated,
			&n.ID, &n.Name, &n.EntityType, &nDesc, &n.MentionCount, &n.CommunityID, &nCreated); err != nil {
			continue
		}
		if evChunk.Valid {
			e.EvidenceChunkID = evChunk.String
		}
		if nDesc.Valid {
			n.Description = nDesc.String
		}
		neighbors = append(neighbors, NeighborEdge{
			Edge:       &e,
			Neighbor:   &n,
			IsOutgoing: false,
		})
	}

	return neighbors, nil
}

// InsertAlias registers a name alias pointing to a canonical node ID.
func (s *Store) InsertAlias(alias string, canonicalID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO node_aliases (alias, canonical_node_id) VALUES (?, ?) ON CONFLICT(alias) DO NOTHING`,
		strings.ToLower(strings.TrimSpace(alias)), canonicalID)
	return err
}

// GetGraphSnapshot returns nodes and edges suitable for web visualization.
func (s *Store) GetGraphSnapshot(limit int) (*GraphSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 500
	}

	rowsN, err := s.db.Query(`SELECT id, name, entity_type, description, mention_count, community_id, created_at FROM nodes ORDER BY mention_count DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rowsN.Close()

	nodes := make([]*Node, 0)
	nodeMap := make(map[string]bool)
	for rowsN.Next() {
		var n Node
		var desc sql.NullString
		var createdStr string
		if err := rowsN.Scan(&n.ID, &n.Name, &n.EntityType, &desc, &n.MentionCount, &n.CommunityID, &createdStr); err == nil {
			if desc.Valid {
				n.Description = desc.String
			}
			nodes = append(nodes, &n)
			nodeMap[n.ID] = true
		}
	}

	rowsE, err := s.db.Query(`SELECT id, source_node_id, target_node_id, relation_type, weight, evidence_chunk_id, created_at FROM edges LIMIT ?`, limit*3)
	if err != nil {
		return nil, err
	}
	defer rowsE.Close()

	links := make([]*Edge, 0)
	for rowsE.Next() {
		var e Edge
		var evChunk sql.NullString
		var createdStr string
		if err := rowsE.Scan(&e.ID, &e.SourceNodeID, &e.TargetNodeID, &e.RelationType, &e.Weight, &evChunk, &createdStr); err == nil {
			if evChunk.Valid {
				e.EvidenceChunkID = evChunk.String
			}
			// Only include link if both nodes are present in the snapshot
			if nodeMap[e.SourceNodeID] && nodeMap[e.TargetNodeID] {
				links = append(links, &e)
			}
		}
	}

	return &GraphSnapshot{
		Nodes: nodes,
		Links: links,
	}, nil
}

// GetStats returns summary counts of all entities in the database.
func (s *Store) GetStats() (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]int64)

	var docCount, chunkCount, nodeCount, edgeCount int64
	s.db.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunkCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&nodeCount)
	s.db.QueryRow(`SELECT COUNT(*) FROM edges`).Scan(&edgeCount)

	stats["documents"] = docCount
	stats["chunks"] = chunkCount
	stats["nodes"] = nodeCount
	stats["edges"] = edgeCount

	return stats, nil
}
