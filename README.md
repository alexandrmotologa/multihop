# MultiHop

Agentic Multi-Hop GraphRAG Engine with Hybrid Vector-Graph Retrieval and SQLite Property Graph.

MultiHop is a single-binary retrieval engine designed to solve relational reasoning queries where standard vector search fails. It automatically extracts entities and directed relations into an embedded SQLite property graph, plans multi-step graph traversals, and combines graph evidence paths with dense vector and BM25 search.

## Why MultiHop

Standard vector search evaluates documents by cosine distance in embedding space. This approach succeeds for direct factual questions, but fails when an answer requires connecting indirect links across multiple documents:

```
[Query] Did the company acquire patents from subsidiaries of Vendor X before 2023?
[Vector Search] Returns generic 2023 merger chunks or Vendor X overviews.
[MultiHop Traversal] Follows: Vendor X -> owns -> Subsidiary Y -> holds -> Patent Z -> transferred -> Company.
```

MultiHop addresses this without requiring external graph infrastructure:

1. Embedded SQLite Storage: Both graph structure (nodes, edges, aliases) and vector embeddings are stored inside a single SQLite database using WAL mode.
2. Memory Target Under 30MB RAM: Compact binary representations, stream processing, and packed float32 BLOB vector indexes maintain low memory consumption.
3. Multi-Hop Agentic Planner: Queries are decomposed into sub-goals and traversed using bounded beam search and breadth-first search.
4. Hybrid Reciprocal Rank Fusion: Merges vector similarity, BM25 full-text rank, and graph traversal evidence paths to eliminate hallucinations.
5. React 19 Interactive Studio: An embedded web interface displays force-directed 2D/3D knowledge graphs and animates active reasoning paths in real time.

## Architecture

```
                    +---------------------------------------+
                    |           Ingested Corpus             |
                    |    (Markdown, Text, JSON, PDF)        |
                    +---------------------------------------+
                                        |
                                        v
                    +---------------------------------------+
                    |         Semantic Chunker              |
                    |   Heading-aware sliding windows       |
                    +---------------------------------------+
                                        |
                   +--------------------+--------------------+
                   |                                         |
                   v                                         v
   +-------------------------------+         +-------------------------------+
   |      Embedded SQLite Graph    |         |    Vector and BM25 Indices    |
   | - Nodes (entities, types)     |         | - Packed float32 BLOB vectors |
   | - Edges (directed relations)  |         | - SQLite FTS5 BM25 engine     |
   | - Aliases and communities     |         | - Exact SIMD cosine distance  |
   +-------------------------------+         +-------------------------------+
                   |                                         |
                   +--------------------+--------------------+
                                        |
                                        v
                    +---------------------------------------+
                    |       Agentic Multi-Hop Planner       |
                    | 1. Query decomposition                |
                    | 2. Seed entity resolution             |
                    | 3. Bounded beam search traversal      |
                    | 4. Reciprocal Rank Fusion (RRF)       |
                    +---------------------------------------+
                                        |
                   +--------------------+--------------------+
                   |                                         |
                   v                                         v
   +-------------------------------+         +-------------------------------+
   |      Grounded Synthesis       |         |     React 19 Graph Studio     |
   | - Triples and chunk evidence  |         | - Force-directed 2D/3D canvas |
   | - Mandatory footnote cites    |         | - Animated reasoning path     |
   +-------------------------------+         +-------------------------------+
```

## Quick Start

### Installation

Clone the repository and build the binary:

```bash
git clone https://github.com/alexandrmotologa/multihop.git
cd multihop
go build -o multihop ./cmd/multihop
```

### Basic Usage

1. Initialize a knowledge base:

```bash
./multihop init --db ./knowledge.db
```

2. Index a directory of documents:

```bash
./multihop index --dir ./examples/legal_contracts --db ./knowledge.db
```

3. Run a multi-hop query:

```bash
./multihop query "What liability caps apply to European subsidiaries of Vendor Alpha?" --db ./knowledge.db
```

4. Find the shortest reasoning path between two entities:

```bash
./multihop path --from "Vendor Alpha" --to "Liability Cap" --db ./knowledge.db
```

5. Launch the Web Studio:

```bash
./multihop serve --port 8080 --db ./knowledge.db
```

Open `http://localhost:8080` in your browser.

## CLI Commands

| Command | Description |
| :--- | :--- |
| `multihop init` | Initializes a new SQLite database with schema and indices |
| `multihop index` | Parses documents, extracts entities/relations, computes vectors, and indexes |
| `multihop query` | Runs an agentic multi-hop query with ASCII reasoning tree and citations |
| `multihop path` | Computes the shortest reasoning path between two entities using Dijkstra |
| `multihop inspect` | Inspects an entity, its neighbors, aliases, and degree centrality |
| `multihop cypher` | Executes a Cypher-Lite graph pattern query against SQLite |
| `multihop export` | Exports graph data to GraphML or Cytoscape JSON |
| `multihop benchmark` | Runs comparative evaluation between Naive Vector RAG and MultiHop |
| `multihop serve` | Starts the HTTP API and embedded React 19 Web Studio |

## Configuration

MultiHop supports both local offline execution and cloud LLM providers:

```bash
# Offline mode (default): uses built-in deterministic embedder and rule extractor
./multihop query "..."

# OpenAI mode
export OPENAI_API_KEY="sk-..."
./multihop query "..." --provider openai --model gpt-4o-mini

# Ollama mode (local LLM)
./multihop query "..." --provider ollama --model llama3 --ollama-url http://localhost:11434
```

## Development and Testing

Run unit and integration tests:

```bash
go test -v ./...
```

Run benchmarks:

```bash
go test -bench=. ./internal/vector ./internal/graph
```

Build frontend assets:

```bash
cd web
npm install
npm run build
cd ..
go build -o multihop ./cmd/multihop
```

## License

MIT License. See [LICENSE](LICENSE) for details.
