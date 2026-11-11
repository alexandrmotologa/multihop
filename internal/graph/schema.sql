-- SQLite Property Graph Schema for MultiHop
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

-- Documents Registry
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    source_path TEXT NOT NULL,
    file_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    token_count INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Text Chunks
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    header_context TEXT,
    embedding BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- FTS5 Full-Text Index on Chunks
CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
    id UNINDEXED,
    content,
    tokenize='porter unicode61'
);

-- Graph Nodes (Entities)
CREATE TABLE IF NOT EXISTS nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    description TEXT,
    mention_count INTEGER DEFAULT 1,
    community_id INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Entity Aliases for Disambiguation
CREATE TABLE IF NOT EXISTS node_aliases (
    alias TEXT PRIMARY KEY,
    canonical_node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE
);

-- Graph Edges (Directed Relationships)
CREATE TABLE IF NOT EXISTS edges (
    id TEXT PRIMARY KEY,
    source_node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    target_node_id TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    weight REAL DEFAULT 1.0,
    evidence_chunk_id TEXT REFERENCES chunks(id) ON DELETE SET NULL,
    valid_from TEXT,
    valid_to TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Community Clusters and Summaries
CREATE TABLE IF NOT EXISTS communities (
    id INTEGER PRIMARY KEY,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    node_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indices for high-speed bi-directional graph traversal and searches
CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_node_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_node_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(entity_type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_chunks_doc ON chunks(document_id);
