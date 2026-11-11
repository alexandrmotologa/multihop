# MultiHop API Reference

The MultiHop server exposes a RESTful API and a Server-Sent Events (SSE) streaming endpoint on port 8080 by default.

## Endpoints

### 1. Health and Status

#### `GET /api/health`
Returns the status and memory metrics of the running engine.

Response:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "memory_alloc_mb": 14.82,
  "memory_sys_mb": 26.15,
  "goroutines": 8
}
```

#### `GET /api/stats`
Returns counts for documents, chunks, nodes, edges, and communities.

Response:
```json
{
  "documents_count": 12,
  "chunks_count": 84,
  "nodes_count": 142,
  "edges_count": 310,
  "communities_count": 7
}
```

---

### 2. Multi-Hop Query and Streaming

#### `POST /api/query`
Executes an agentic multi-hop query synchronously.

Request:
```json
{
  "query": "Did the company acquire patents from Vendor X subsidiaries?",
  "max_hops": 3,
  "top_k": 5
}
```

Response:
```json
{
  "query": "Did the company acquire patents from Vendor X subsidiaries?",
  "answer": "Yes. Vendor Alpha owns Subsidiary Beta, which held Patent 9821, transferred to the parent company in 2022 [^c_8a1f].",
  "reasoning_paths": [
    {
      "nodes": ["vendor_alpha", "subsidiary_beta", "patent_9821", "acme_corp"],
      "relations": ["OWNS", "HOLDS", "TRANSFERRED_TO"],
      "score": 0.842
    }
  ],
  "citations": [
    {
      "chunk_id": "c_8a1f",
      "document_id": "doc_01",
      "document_title": "2022 IP Transfer Agreement",
      "content": "Subsidiary Beta hereby transfers all rights to Patent 9821 to Acme Corp...",
      "relevance_score": 0.912
    }
  ],
  "execution_time_ms": 142
}
```

#### `GET /api/query/stream?q={query}`
Streams step-by-step reasoning progress via Server-Sent Events (SSE).

Event Stream:
```
event: step
data: {"step": "decomposing_query", "subgoals": ["Identify subsidiaries of Vendor X", "Check patent transfers"]}

event: step
data: {"step": "resolving_seeds", "seeds": [{"id": "vendor_x", "name": "Vendor X"}]}

event: step
data: {"step": "graph_traversal", "hop": 1, "explored_nodes": 4, "current_paths": 2}

event: step
data: {"step": "hybrid_fusion", "chunks_retrieved": 6}

event: token
data: {"token": "Vendor"}

event: token
data: {"token": " Alpha"}

event: complete
data: {"answer": "...", "citations": [...], "latency_ms": 138}
```

---

### 3. Knowledge Graph Inspection

#### `GET /api/graph`
Returns the complete or filtered knowledge graph formatted for 2D/3D visualizers.

Query Parameters:
- `limit`: Max nodes (default 300)
- `community`: Filter by community ID
- `type`: Filter by entity type

Response:
```json
{
  "nodes": [
    {
      "id": "vendor_alpha",
      "name": "Vendor Alpha",
      "entity_type": "ORGANIZATION",
      "mention_count": 6,
      "community_id": 1
    }
  ],
  "links": [
    {
      "id": "e_102",
      "source": "vendor_alpha",
      "target": "subsidiary_beta",
      "relation_type": "OWNS",
      "weight": 1.0
    }
  ]
}
```

#### `GET /api/path?from={id1}&to={id2}`
Computes the shortest reasoning path between two entities.

Response:
```json
{
  "source": "vendor_alpha",
  "target": "patent_9821",
  "found": true,
  "hops": 2,
  "path": [
    {
      "node": "vendor_alpha",
      "relation": "OWNS",
      "next_node": "subsidiary_beta"
    },
    {
      "node": "subsidiary_beta",
      "relation": "HOLDS",
      "next_node": "patent_9821"
    }
  ]
}
```

---

### 4. Document Management

#### `GET /api/documents`
Lists all indexed documents.

#### `GET /api/documents/:id/chunks`
Lists chunks belonging to a document.

#### `POST /api/documents`
Uploads and indexes a new document file.
