<p align="center">
  <img src="docs/images/logo.png?raw=true" alt="MultiHop Logo" width="130" style="border-radius: 24px;" />
</p>

<h1 align="center">MultiHop</h1>

<p align="center">
  <a href="https://github.com/alexandrmotologa/multihop/actions"><img src="https://github.com/alexandrmotologa/multihop/actions/workflows/ci.yml/badge.svg" alt="CI Status" /></a>
  <img src="https://img.shields.io/badge/go-1.24-00ADD8?logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/SQLite-WAL%20%2B%20FTS5-003B57?logo=sqlite" alt="SQLite" />
  <img src="https://img.shields.io/badge/RAM-%3C%2030MB-10B981" alt="RAM Target" />
  <img src="https://img.shields.io/badge/frontend-React%2019-61DAFB?logo=react" alt="React 19" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" /></a>
</p>

<p align="center">
  <strong>Agentic Multi-Hop GraphRAG Engine with Hybrid Vector-Graph Retrieval and SQLite Property Graph</strong>
</p>

<p align="center">
  <img src="docs/images/multihop_demo.gif?raw=true" alt="MultiHop Demo Walkthrough" width="100%" style="border-radius: 12px; border: 1px solid #1e293b;" />
</p>

---

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

## Visual Studio and Interfaces

### Multi-Hop Studio with Reasoning Player
Execute multi-hop queries, inspect decomposed sub-goals, step through relational hops with the interactive player, and verify strict citation footnotes with real-time RAG Triad diagnostic metrics.

<p align="center">
  <img src="docs/images/multihop_studio.png?raw=true" alt="Multi-Hop Studio" width="100%" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Interactive 2D/3D Knowledge Graph Explorer
Inspect entity nodes, directed relationship edges, and community clusters with force-directed physics layout, active hop laser pulse animations, and detailed entity sidebars.

<p align="center">
  <img src="docs/images/multihop_graph.png?raw=true" alt="Graph Explorer" width="100%" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Cypher-Lite Micro Query Console
Execute open Cypher pattern matching queries directly against SQLite recursive common table expressions (CTEs) with instant JSON record inspection.

<p align="center">
  <img src="docs/images/multihop_cypher.png?raw=true" alt="Cypher Console" width="100%" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Empirical Architecture Benchmark
Compare Naive Vector RAG against MultiHop GraphRAG side-by-side on real relational multi-hop queries measuring latency, path recovery, and grounding accuracy.

<p align="center">
  <img src="docs/images/multihop_benchmark.png?raw=true" alt="Architecture Benchmark" width="100%" style="border-radius: 10px; border: 1px solid #1e293b;" />
</p>

### Responsive Multi-Device Adaptability
The Studio and Graph interfaces adapt dynamically across mobile, tablet, and widescreen viewports with slide-out navigation drawers and touch-friendly controls.

<p align="center">
  <img src="docs/images/multihop_mobile.png?raw=true" alt="Mobile View" width="380" style="border-radius: 16px; border: 1px solid #1e293b;" />
</p>

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
./multihop path --from "Vendor Alpha" --to "Patent 9821" --db ./knowledge.db
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
| `multihop query` | Runs an agentic multi-hop query with ASCII reasoning tree and citations (`--mode`, `--as-of`) |
| `multihop path` | Computes the shortest reasoning path between two entities using Dijkstra |
| `multihop inspect` | Inspects an entity, its neighbors, aliases, and degree centrality |
| `multihop cypher` | Executes a Cypher-Lite graph pattern query against SQLite |
| `multihop export` | Exports graph data to GraphML or Cytoscape JSON |
| `multihop duplicates` | Detects candidate duplicate entities for disambiguation |
| `multihop merge` | Merges duplicate entities into a canonical node, remapping edges |
| `multihop communities` | Generates and lists Label Propagation Algorithm (LPA) communities and summaries |
| `multihop benchmark` | Runs comparative evaluation between Naive Vector RAG and MultiHop |
| `multihop serve` | Starts the HTTP API and embedded React 19 Web Studio |

## Key Capabilities

### 1. Dual-Mode Search
- Local Multi-Hop: Traverses fine-grained entity chains up to N hops using bounded beam search.
- Global Community Search: Summarizes entire graph clusters generated via the Label Propagation Algorithm (LPA).
- Hybrid Auto-Routing: Classifies incoming queries and selects the optimal traversal strategy.

### 2. Temporal 4D Graph Traversal
Edges maintain `valid_from` and `valid_to` bounds. Queries support `--as-of YYYY-MM-DD` to evaluate corporate and legal relationships at any historical point in time.

### 3. Entity Disambiguation and Resolution
Normalized edit distance and acronym matching identify duplicate nodes across documents. The `merge` engine transactionally rewrites inbound and outbound edges while registering alias tables.

### 4. Interactive Reasoning Path Player
The React 19 studio includes a step-by-step player with auto-panning camera, active hop highlighting, and real-time RAG Triad scores (Context Relevance, Groundedness, Answer Relevance).

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
