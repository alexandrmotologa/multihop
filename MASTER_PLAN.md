# MultiHop Architecture and Implementation Plan

Agentic Multi-Hop GraphRAG Engine with Hybrid Vector-Graph Retrieval
Target Memory Footprint: under 30MB RAM Single-Binary Go Engine + Embedded SQLite Property Graph + React 19 Visual Studio

---

## 1. Executive Summary and Problem Statement

### 1.1 The Failure Modes of Naive RAG
First-generation Retrieval-Augmented Generation (RAG) relied almost entirely on top-k cosine similarity over dense vector embeddings. While adequate for direct factual lookups, naive RAG fails on enterprise analytical queries requiring relational reasoning across multiple documents:
1. Multi-Hop Reasoning Blindspot: Consider a query such as:
   "Did the company acquire any patents from subsidiaries of Vendor X prior to the 2023 merger?"
   A dense vector search returns chunks discussing the 2023 merger or Vendor X in isolation. It cannot follow the relational chain:
   Vendor X -> owns -> Subsidiary Y -> holds -> Patent Z -> transferred -> Company.
2. Context Fragmentation: Pure vector retrieval surfaces isolated paragraphs without structural relationships, forcing the language model to extrapolate connections that may not exist.
3. Operational Cost of Microsoft GraphRAG: Traditional GraphRAG implementations require external graph databases such as Neo4j, high token budgets for community summaries, and multi-gigabyte memory footprints unsuitable for local or cost-sensitive deployments.

### 1.2 The MultiHop Solution
MultiHop is a lightweight, single-binary GraphRAG engine combining dense vector retrieval, BM25 full-text search, and an embedded SQLite property graph with an agentic multi-hop query planner:
- Zero External Infrastructure: Runs entirely inside an embedded SQLite database using WAL mode and pure Go drivers.
- Memory Target under 30MB RAM: Compact memory allocations, stream processing of documents, and vector storage directly inside SQLite BLOBs.
- Agentic Multi-Hop Planner: Decomposes complex user queries into directional graph traversal goals, navigating N-degree relationships with bounded beam search and BFS.
- Hybrid Reciprocal Rank Fusion (RRF): Combines dense vector similarity, BM25 lexical ranking, and graph traversal evidence paths.
- Interactive React 19 Studio: Embedded web interface with 2D/3D force-directed knowledge graph visualization, real-time reasoning path animation, and click-to-verify chunk attribution.

---

## 2. High-Level System Architecture

```
                              +---------------------------------------+
                              |         Enterprise Documents          |
                              |    (Markdown, Text, JSON, PDF)        |
                              +---------------------------------------+
                                                  |
                                                  v
                              +---------------------------------------+
                              |         Ingestion and Chunking        |
                              |  - Semantic Heading-Aware Chunker     |
                              |  - Token and Character Sliding Window |
                              +---------------------------------------+
                                                  |
                                                  v
                              +---------------------------------------+
                              |      Entity and Relation Extractor    |
                              |  - LLM Extraction or Builtin Parser   |
                              |  - Normalization and Aliasing Engine  |
                              +---------------------------------------+
                                                  |
                     +----------------------------+----------------------------+
                     |                                                         |
                     v                                                         v
     +-------------------------------+                         +-------------------------------+
     |     Embedded SQLite Graph     |                         |     Vector and BM25 Indices   |
     |  - Nodes (Entities and Types) |                         |  - Packed Float32 Vector BLOB |
     |  - Edges (Relations, Weights) |                         |  - SQLite FTS5 BM25 Engine    |
     |  - Aliases and Communities    |                         |  - Exact SIMD Cosine Distance |
     +-------------------------------+                         +-------------------------------+
                     |                                                         |
                     +----------------------------+----------------------------+
                                                  |
                                                  v
                              +---------------------------------------+
                              |       Complex Multi-Hop Query         |
                              |  "How does Policy A affect Vendor B?" |
                              +---------------------------------------+
                                                  |
                                                  v
                              +---------------------------------------+
                              |      Agentic Multi-Hop Planner        |
                              |  1. Sub-question Decomposition        |
                              |  2. Seed Entity Resolution            |
                              |  3. Bounded Beam Search Traversal     |
                              |  4. Hybrid RRF Retrieval              |
                              +---------------------------------------+
                                                  |
                                                  v
                              +---------------------------------------+
                              |     Context Synthesis and Rerank      |
                              |  - Traversal Path Triples Assembly    |
                              |  - Strict Citation Footnotes          |
                              |  - Anti-Hallucination Guard           |
                              +---------------------------------------+
                                                  |
                     +----------------------------+----------------------------+
                     |                                                         |
                     v                                                         v
     +-------------------------------+                         +-------------------------------+
     |       LLM Answer Output       |                         |    React 19 Graph Studio      |
     |  - Grounded Text Response     |                         |  - Force-Directed 2D/3D Graph |
     |  - Exact Source Provenance    |                         |  - Step-by-Step Path Lighting |
     +-------------------------------+                         +-------------------------------+
```

---

## 3. Core Database Schema

The database utilizes SQLite with WAL mode, foreign keys, and FTS5 full-text indexing.

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

-- Document Registry
CREATE TABLE IF NOT EXISTS documents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    source_path TEXT NOT NULL,
    file_type TEXT NOT NULL,
    byte_size INTEGER NOT NULL,
    token_count INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Document Chunks
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    document_id TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    token_count INTEGER NOT NULL,
    embedding BLOB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- FTS5 Full-Text Search on Chunks
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

-- Entity Aliases and Normalization
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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indices for high-speed bi-directional graph traversal
CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_node_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_node_id, relation_type);
CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(entity_type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
```

---

## 4. Multi-Hop Traversal and Retrieval Algorithms

### 4.1 Seed Entity Identification
Given query $Q$, seed entities $S = \{s_1, s_2, \dots\}$ are detected by matching against the `nodes` and `node_aliases` tables using exact matching, normalized token overlap, and Levenshtein edit distance.

### 4.2 Bounded Beam Search Traversal
From seed entities $S$, outgoing and incoming edges are traversed up to depth $D = 1 \dots 4$. At each hop $k$, candidates are scored using:
$$\text{Score}(path) = \prod_{i=1}^{k} \text{weight}(e_i) \times \text{sim}(Q, \text{chunk}_{evidence}) \times \frac{1}{1 + \ln(\text{depth})}$$
The beam width $W$ limits active paths to the top scoring trajectories, eliminating combinatorial explosion while guaranteeing deterministic memory bounds.

### 4.3 Reciprocal Rank Fusion (RRF)
To ensure complete factual grounding, retrieval combines three complementary signals:
1. Dense vector similarity search on chunk embeddings.
2. BM25 full-text keyword search over `chunks_fts`.
3. Graph traversal path evidence chunks.

For each candidate chunk $c$, the fusion score is calculated as:
$$\text{RRF}(c) = \sum_{m \in \{vec, bm25, graph\}} \frac{w_m}{k_{rrf} + \text{rank}_m(c)}$$
where $k_{rrf} = 60$, $w_{vec} = 1.0$, $w_{bm25} = 0.8$, and $w_{graph} = 1.2$.

---

## 5. Detailed Task and Subtask Breakdown

### Milestone 1: Project Setup, Repository and Infrastructure
- Task 1.1: Initialize Go module, directory hierarchy, and build tools.
  - Subtask 1.1.1: Initialize `go.mod` with pure Go SQLite driver (`modernc.org/sqlite`).
  - Subtask 1.1.2: Scaffold directory tree (`cmd/`, `internal/`, `web/`, `examples/`, `docs/`).
  - Subtask 1.1.3: Create Makefile and GitHub Actions CI workflow (`.github/workflows/ci.yml`).
- Task 1.2: Documentation and humanized guidelines.
  - Subtask 1.2.1: Write humanized README.md without AI clichés or em dashes.
  - Subtask 1.2.2: Add architecture documentation in `docs/architecture.md`.
  - Subtask 1.2.3: Add API specification in `docs/api.md`.

### Milestone 2: Document Ingestion, Chunking and Vector Engine
- Task 2.1: Parser and semantic chunker.
  - Subtask 2.1.1: Implement Markdown header-aware chunker with configurable token size and overlap.
  - Subtask 2.1.2: Implement Plaintext and JSON document ingestion.
  - Subtask 2.1.3: Implement document metadata registry and chunk hashing for deduplication.
- Task 2.2: Embedded vector engine.
  - Subtask 2.2.1: Define pluggable `Embedder` interface.
  - Subtask 2.2.2: Implement built-in fast deterministic semantic embedder (384-dimensional, zero external API required for local testing).
  - Subtask 2.2.3: Implement OpenAI, Gemini, and Ollama embedding client adapters.
  - Subtask 2.2.4: Implement packed float32 BLOB encoding/decoding and SIMD-optimized cosine similarity.

### Milestone 3: Embedded SQLite Graph Store and Extraction
- Task 3.1: SQLite property graph store.
  - Subtask 3.1.1: Implement database initialization, schema migration, and connection pooling with WAL mode.
  - Subtask 3.1.2: Implement CRUD operations for nodes, edges, aliases, and chunks.
  - Subtask 3.1.3: Implement bidirectional adjacency queries and neighbor lookups.
- Task 3.2: Entity and relation extraction.
  - Subtask 3.2.1: Implement structured LLM extraction prompt for entities and directed triples with chunk provenance.
  - Subtask 3.2.2: Implement deterministic rule-based / regex / grammatical fallback extractor for offline usage.
  - Subtask 3.2.3: Implement entity normalizer and alias resolver (case normalization, acronym resolution, fuzzy matching).

### Milestone 4: Multi-Hop Traversal and Graph Analytics
- Task 4.1: Graph traversal engine.
  - Subtask 4.1.1: Implement bounded Breadth-First Search (BFS) and Depth-First Search (DFS).
  - Subtask 4.1.2: Implement Beam Search with path scoring and cycle avoidance.
  - Subtask 4.1.3: Implement shortest reasoning path finder between two entities (Dijkstra algorithm).
- Task 4.2: Graph analytics and Cypher-Lite.
  - Subtask 4.2.1: Implement Label Propagation Algorithm (LPA) for entity community detection.
  - Subtask 4.2.2: Implement degree and betweenness centrality ranking.
  - Subtask 4.2.3: Implement Cypher-Lite micro query parser and executor for pattern matching (`MATCH (a)-[r]->(b)`).

### Milestone 5: Agentic Multi-Hop Planner and Hybrid Retrieval
- Task 5.1: Agentic query planner.
  - Subtask 5.1.1: Implement query analyzer that decomposes compound queries into sub-questions and target hops.
  - Subtask 5.1.2: Implement multi-step execution coordinator with intermediate state tracking.
- Task 5.2: Hybrid retrieval and context synthesis.
  - Subtask 5.2.1: Implement Reciprocal Rank Fusion (RRF) combining vector, BM25, and graph paths.
  - Subtask 5.2.2: Implement prompt context builder structuring factual triples and source text.
  - Subtask 5.2.3: Implement grounded answer synthesizer with mandatory footnote citations `[^chunk_id]`.
  - Subtask 5.2.4: Implement anti-hallucination verification checking generated claims against retrieved triples.

### Milestone 6: CLI Command Surface and Benchmarking
- Task 6.1: Command line interface.
  - Subtask 6.1.1: Implement `multihop init` and `multihop index`.
  - Subtask 6.1.2: Implement `multihop query` with ASCII reasoning path tree.
  - Subtask 6.1.3: Implement `multihop path`, `multihop inspect`, and `multihop cypher`.
  - Subtask 6.1.4: Implement `multihop export` (GraphML, Cytoscape JSON).
- Task 6.2: Evaluation and benchmarking suite.
  - Subtask 6.2.1: Implement automated evaluation dataset (multi-hop legal contracts and corporate relationships).
  - Subtask 6.2.2: Implement comparative benchmark comparing Naive Vector RAG vs MultiHop GraphRAG (accuracy, retrieval recall, latency, memory footprint).

### Milestone 7: HTTP Server and Real-Time SSE API
- Task 7.1: REST API.
  - Subtask 7.1.1: Implement endpoints for document management, graph inspection, and search.
  - Subtask 7.1.2: Implement Server-Sent Events (SSE) streaming endpoint `/api/query/stream` emitting step-by-step reasoning events.
  - Subtask 7.1.3: Implement metrics endpoint reporting active memory usage and graph statistics.

### Milestone 8: React 19 Interactive Visual Studio
- Task 8.1: Frontend architecture.
  - Subtask 8.1.1: Set up Vite, React 19, TypeScript, and Tailwind CSS in `web/`.
  - Subtask 8.1.2: Embed production frontend build into Go binary using `embed.FS`.
- Task 8.2: Interactive components.
  - Subtask 8.2.1: Build 2D/3D Force-Directed Graph canvas with interactive zoom, pan, and node exploration.
  - Subtask 8.2.2: Build real-time reasoning path animation lighting up traversed nodes and edges.
  - Subtask 8.2.3: Build streaming chat interface with interactive citation drawer displaying raw chunk text.
  - Subtask 8.2.4: Build live benchmark dashboard visualizing Naive RAG vs MultiHop performance.

### Milestone 9: Advanced GraphRAG Capabilities (Completed)
- Task 9.1: Dual-Mode Search (Local Multi-Hop vs Global Community Search).
  - Subtask 9.1.1: Implement automatic query routing detecting broad thematic vs entity-specific queries.
  - Subtask 9.1.2: Implement hierarchical community summary generation via Label Propagation Algorithm (LPA).
  - Subtask 9.1.3: Add CLI `--mode global` and API `mode` parameter.
- Task 9.2: Temporal 4D Knowledge Graph Traversal.
  - Subtask 9.2.1: Extend graph schema with `valid_from` and `valid_to` edge timestamp bounds.
  - Subtask 9.2.2: Implement point-in-time traversal filters with `--as-of` flag.
  - Subtask 9.2.3: Add temporal date picker in React 19 Studio.
- Task 9.3: Cypher-Lite Interactive Query Playground.
  - Subtask 9.3.1: Implement Cypher-Lite recursive CTE parser and executor in Go backend.
  - Subtask 9.3.2: Build dedicated `CypherConsole` component with query presets and JSON record viewer.
- Task 9.4: Entity Disambiguation and Deduplication.
  - Subtask 9.4.1: Implement candidate duplicate detector based on Levenshtein distance and alias expansion.
  - Subtask 9.4.2: Implement transactional node merger remapping edges and preserving historical aliases.
  - Subtask 9.4.3: Build `DeduplicationPanel` with one-click entity merging in web studio.
- Task 9.5: Reasoning Path Player & Visual Diagnostics.
  - Subtask 9.5.1: Build step-by-step playback controls (Play, Pause, Step Next, Step Prev, Reset).
  - Subtask 9.5.2: Implement glowing laser pulse animation and auto-panning camera on active hops in `GraphCanvas`.
  - Subtask 9.5.3: Compute and display real-time RAG Triad scores (Context Relevance, Groundedness, Answer Relevance).
- Task 9.6: Drag-and-Drop Document Ingestion in UI.
  - Subtask 9.6.1: Build `DocumentUploadModal` with drag-and-drop file upload and real-time extraction progress.
  - Subtask 9.6.2: Implement `/api/documents/upload` endpoint with on-the-fly chunking, embedding, and entity extraction.

---

## 6. Verification and Acceptance Criteria
1. Single Binary: Compiles cleanly with standard `go build` to a single binary with embedded web assets.
2. Memory Footprint: Engine operates under 30MB RAM under normal query workloads.
3. Zero Hardcoding: Ingestion, graph construction, traversal, and retrieval operate dynamically on arbitrary ingested text.
4. Offline Capable: All tests and core workflows execute with built-in embedder and rule extractor without mandatory cloud API keys.
5. 100% Functional Verification: Full automated test suite covering chunker, SQLite store, graph algorithms, multi-hop planner, RRF, and HTTP/SSE routes.
