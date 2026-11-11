package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alexandrmotologa/multihop/internal/agent"
	"github.com/alexandrmotologa/multihop/internal/extractor"
	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
	"github.com/alexandrmotologa/multihop/web"
)

// Server handles HTTP REST, SSE streaming, and frontend static asset delivery.
type Server struct {
	store       *graph.Store
	coordinator *agent.Coordinator
	port        int
	mux         *http.ServeMux
}

// NewServer initializes the API server.
func NewServer(store *graph.Store, coordinator *agent.Coordinator, port int) *Server {
	if port <= 0 {
		port = 8080
	}
	s := &Server{
		store:       store,
		coordinator: coordinator,
		port:        port,
		mux:         http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	// Health & Metrics
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// Graph Inspection
	s.mux.HandleFunc("GET /api/graph", s.handleGraph)
	s.mux.HandleFunc("GET /api/path", s.handlePath)
	s.mux.HandleFunc("POST /api/cypher", s.handleCypher)

	// Multi-Hop Query & SSE Streaming
	s.mux.HandleFunc("POST /api/query", s.handleQuery)
	s.mux.HandleFunc("GET /api/query/stream", s.handleQueryStream)

	// Documents & Chunks
	s.mux.HandleFunc("GET /api/documents", s.handleListDocuments)
	s.mux.HandleFunc("POST /api/documents", s.handleCreateDocument)
	s.mux.HandleFunc("POST /api/documents/upload", s.handleUploadDocument)
	s.mux.HandleFunc("GET /api/documents/{id}/chunks", s.handleListChunks)

	// Entity Deduplication & Communities
	s.mux.HandleFunc("GET /api/nodes/duplicates", s.handleGetDuplicates)
	s.mux.HandleFunc("POST /api/nodes/merge", s.handleMergeNodes)
	s.mux.HandleFunc("GET /api/communities", s.handleListCommunities)
	s.mux.HandleFunc("POST /api/communities/generate", s.handleGenerateCommunities)

	// Embedded Frontend SPA
	distFS := web.GetDistFS()
	fileServer := http.FileServer(http.FS(distFS))

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// If requesting an API route that wasn't matched, return 404 JSON
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Check if file exists in embedded FS
		if f, err := distFS.Open(path); err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		indexData, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexData)
	})
}

// Start launches the HTTP listener.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("[MultiHop] HTTP & SSE server listening on http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s.enableCORS(s.mux))
}

func (s *Server) enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	res := map[string]interface{}{
		"status":          "ok",
		"version":         "1.0.0",
		"memory_alloc_mb": float64(m.Alloc) / 1024 / 1024,
		"memory_sys_mb":   float64(m.Sys) / 1024 / 1024,
		"goroutines":      runtime.NumGoroutine(),
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 500
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	snapshot, err := s.store.GetGraphSnapshot(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "parameters 'from' and 'to' are required")
		return
	}

	// Resolve names to IDs if needed
	srcNode, err := s.store.FindNodeByNameOrAlias(from)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("source entity '%s' not found", from))
		return
	}

	tgtNode, err := s.store.FindNodeByNameOrAlias(to)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("target entity '%s' not found", to))
		return
	}

	path, err := s.store.DijkstraShortestPath(srcNode.ID, tgtNode.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, path)
}

func (s *Server) handleCypher(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query == "" {
		writeError(w, http.StatusBadRequest, "valid 'query' string required")
		return
	}

	rows, err := s.store.ExecuteCypherLite(body.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"query":   body.Query,
		"count":   len(rows),
		"results": rows,
	})
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req agent.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Query == "" {
		writeError(w, http.StatusBadRequest, "valid 'query' required")
		return
	}

	resp, err := s.coordinator.ExecuteQuery(r.Context(), req, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleQueryStream(w http.ResponseWriter, r *http.Request) {
	queryText := r.URL.Query().Get("q")
	if queryText == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported by client")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	req := agent.QueryRequest{
		Query: queryText,
	}

	stepHandler := func(ev agent.QueryStepEvent) {
		dataBytes, _ := json.Marshal(ev)
		fmt.Fprintf(w, "event: step\ndata: %s\n\n", string(dataBytes))
		flusher.Flush()
	}

	resp, err := s.coordinator.ExecuteQuery(r.Context(), req, stepHandler)
	if err != nil {
		errBytes, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", string(errBytes))
		flusher.Flush()
		return
	}

	completeBytes, _ := json.Marshal(resp)
	fmt.Fprintf(w, "event: complete\ndata: %s\n\n", string(completeBytes))
	flusher.Flush()
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	docs, err := s.store.ListDocuments()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (s *Server) handleListChunks(w http.ResponseWriter, r *http.Request) {
	docID := r.PathValue("id")
	chunks, err := s.store.ListChunksByDocument(docID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chunks)
}

func (s *Server) handleCreateDocument(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Content == "" {
		writeError(w, http.StatusBadRequest, "valid document 'content' required")
		return
	}

	title := body.Title
	if title == "" {
		title = "Untitled Document"
	}

	docID := "doc_" + parser.ComputeHash(body.Content)
	doc := &parser.Document{
		ID:         docID,
		Title:      title,
		SourcePath: "api-upload",
		FileType:   "text",
		ByteSize:   int64(len(body.Content)),
		TokenCount: parser.ApproximateTokens(body.Content),
		Content:    body.Content,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.store.InsertDocument(doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	chunks, err := parser.ChunkDocument(doc, parser.DefaultChunkerOptions())
	if err == nil {
		s.store.InsertChunks(chunks)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           doc.ID,
		"title":        doc.Title,
		"chunks_count": len(chunks),
	})
}

func (s *Server) handleGetDuplicates(w http.ResponseWriter, r *http.Request) {
	candidates, err := s.store.FindDuplicateCandidates(2)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, candidates)
}

func (s *Server) handleMergeNodes(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CanonicalID  string   `json:"canonical_id"`
		DuplicateIDs []string `json:"duplicate_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CanonicalID == "" || len(body.DuplicateIDs) == 0 {
		writeError(w, http.StatusBadRequest, "valid 'canonical_id' and non-empty 'duplicate_ids' array required")
		return
	}

	if err := s.store.MergeNodes(body.CanonicalID, body.DuplicateIDs); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"canonical_id": body.CanonicalID,
		"merged_count": len(body.DuplicateIDs),
	})
}

func (s *Server) handleListCommunities(w http.ResponseWriter, r *http.Request) {
	comms, err := s.store.ListCommunities()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comms)
}

func (s *Server) handleGenerateCommunities(w http.ResponseWriter, r *http.Request) {
	clusters, err := s.store.DetectCommunitiesLabelPropagation(10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	comms, _ := s.store.ListCommunities()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "ok",
		"communities_count": len(clusters),
		"communities":       comms,
	})
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' in upload form")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read uploaded file: "+err.Error())
		return
	}

	content := string(data)
	filename := header.Filename
	title := strings.TrimSuffix(filename, strings.ToLower(filepath.Ext(filename)))

	fileType := "text"
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".md" || ext == ".markdown" {
		fileType = "markdown"
	} else if ext == ".json" {
		fileType = "json"
	}

	docID := "doc_" + parser.ComputeHash(content+filename)
	doc := &parser.Document{
		ID:         docID,
		Title:      title,
		SourcePath: filename,
		FileType:   fileType,
		ByteSize:   int64(len(data)),
		TokenCount: parser.ApproximateTokens(content),
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.store.InsertDocument(doc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	chunks, err := parser.ChunkDocument(doc, parser.DefaultChunkerOptions())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	embedder := vector.NewBuiltinEmbedder(384)
	extRule := extractor.NewRuleExtractor()
	ctx := context.Background()

	// Embed chunks and commit
	for _, c := range chunks {
		vec, _ := embedder.EmbedText(ctx, c.Content)
		c.Embedding = vec
	}
	s.store.InsertChunks(chunks)

	entitiesCount := 0
	relationsCount := 0

	for _, c := range chunks {
		exRes, _ := extRule.Extract(ctx, c)
		if exRes != nil {
			for _, e := range exRes.Entities {
				slug, aliases := extractor.NormalizeEntity(e.Name)
				s.store.UpsertNode(&graph.Node{
					ID:          slug,
					Name:        e.Name,
					EntityType:  e.Type,
					Description: e.Description,
					CreatedAt:   time.Now().UTC(),
				})
				for _, a := range aliases {
					s.store.InsertAlias(a, slug)
				}
				entitiesCount++
			}
			for _, rel := range exRes.Relations {
				srcSlug, _ := extractor.NormalizeEntity(rel.Source)
				tgtSlug, _ := extractor.NormalizeEntity(rel.Target)
				s.store.UpsertNode(&graph.Node{ID: srcSlug, Name: rel.Source, EntityType: "ORGANIZATION", CreatedAt: time.Now().UTC()})
				s.store.UpsertNode(&graph.Node{ID: tgtSlug, Name: rel.Target, EntityType: "ORGANIZATION", CreatedAt: time.Now().UTC()})

				edgeID := fmt.Sprintf("e_%s_%s_%s", srcSlug, strings.ToLower(rel.Type), tgtSlug)
				s.store.UpsertEdge(&graph.Edge{
					ID:              edgeID,
					SourceNodeID:    srcSlug,
					TargetNodeID:    tgtSlug,
					RelationType:    rel.Type,
					Weight:          rel.Weight,
					EvidenceChunkID: c.ID,
					CreatedAt:       time.Now().UTC(),
				})
				relationsCount++
			}
		}
	}

	// Trigger LPA
	s.store.DetectCommunitiesLabelPropagation(10)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":          "ok",
		"document_id":     doc.ID,
		"title":           doc.Title,
		"chunks_count":    len(chunks),
		"entities_count":  entitiesCount,
		"relations_count": relationsCount,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
