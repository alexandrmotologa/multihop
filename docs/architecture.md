# MultiHop Technical Architecture

## Overview

MultiHop is engineered for low memory consumption and relational multi-hop reasoning. The system runs as a single Go binary that operates directly on an embedded SQLite database using Write-Ahead Logging (WAL).

## Storage Engine

SQLite was chosen over external graph engines (such as Neo4j) and vector databases (such as Qdrant or Milvus) for five primary reasons:
1. Operational Simplicity: No background daemons, zero external network sockets, and single-file portability.
2. Low Memory Baseline: Under typical query loads, SQLite with a 2000-page cache operates in under 15MB of RAM.
3. Hybrid Tabular, Relational, and Full-Text Model: Relational tables store entities and edges, FTS5 provides BM25 full-text indexing, and BLOB columns store packed float32 vector embeddings.
4. Transactional Consistency: Document ingestion, chunking, entity extraction, and edge creation occur within unified transactions.
5. In-Memory Recursive CTEs: Path traversal and Cypher-Lite queries execute using SQLite common table expressions.

## Vector Search Implementation

Vectors are stored as packed little-endian 32-bit floating point arrays in standard SQLite BLOB columns.

For a 384-dimensional embedding:
- Storage size: 384 * 4 = 1,536 bytes per chunk.
- Indexing approach: Chunks are pre-filtered using metadata and FTS5 before cosine distance evaluation, keeping total cosine computations minimal.
- SIMD and Loop-Unrolled Cosine Distance: The cosine similarity function calculates dot products and norms in single-pass linear scans without heap allocations.

$$\text{Cosine Similarity}(u, v) = \frac{\sum_{i=1}^n u_i v_i}{\sqrt{\sum_{i=1}^n u_i^2} \sqrt{\sum_{i=1}^n v_i^2}}$$

## Property Graph Schema

The property graph consists of three core tables:
- `nodes`: Represents entities. Identified by a normalized lowercase slug. Contains `name`, `entity_type` (e.g. ORGANIZATION, PRODUCT, PERSON, CLAUSE), `description`, `mention_count`, and `community_id`.
- `edges`: Represents directed relationships. Contains `source_node_id`, `target_node_id`, `relation_type` (e.g. OWNS, TRANSFERS, LIMITS, LOCATED_IN), `weight`, and `evidence_chunk_id`.
- `node_aliases`: Maps colloquial names, acronyms, and variations to the canonical node ID (e.g. "Apple Corp" -> "apple_inc").

## Traversal Algorithms

### Bounded Beam Search

When a user submits a query, MultiHop resolves seed entities and initiates a bounded beam search:
1. Beam State: A priority queue holding candidate paths.
2. Expansion: At step $k$, candidate edges outgoing from and incoming to the frontier node are evaluated.
3. Cycle Avoidance: Paths tracking visited node sets discard cyclic traversals.
4. Scoring Function:

$$\text{Score}(P_k) = \text{Score}(P_{k-1}) \cdot W(e_k) \cdot \text{sim}(Q, \text{Chunk}(e_k)) \cdot \frac{1}{1 + \ln(k)}$$

5. Pruning: Only the top $W$ paths (default $W=5$, max depth $D=3$) are kept for the next expansion step.

### Dijkstra Shortest Reasoning Path

For queries inquiring about connections between two specific entities:
- Finds the path with minimum total cost where $\text{cost}(e) = \frac{1}{\text{weight}(e)}$.
- Returns the exact chain of relations and the corresponding evidence chunks.

## Reciprocal Rank Fusion (RRF)

To balance lexical precision, semantic similarity, and relational context, candidate chunks are ranked by three channels:
1. Vector Similarity: Top $K$ chunks closest in embedding space to query $Q$.
2. BM25 Search: Top $K$ chunks matching query terms via SQLite FTS5.
3. Graph Evidence: Chunks attached to the top-scoring paths discovered during beam search.

The final rank score for chunk $c$:

$$\text{RRF}(c) = \frac{w_v}{60 + \text{rank}_v(c)} + \frac{w_b}{60 + \text{rank}_b(c)} + \frac{w_g}{60 + \text{rank}_g(c)}$$

Default weights: $w_v = 1.0$, $w_b = 0.8$, $w_g = 1.2$.

## Anti-Hallucination Guard

Before returning an answer, MultiHop verifies assertions:
1. Every claim in the generated answer must refer to a citation index `[^id]`.
2. The synthesizer prompt receives explicit factual triples extracted from the graph alongside source chunks.
3. If an answer mentions relationships absent from the retrieved triples or chunks, the engine flags ungrounded assertions.
