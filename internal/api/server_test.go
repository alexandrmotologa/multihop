package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexandrmotologa/multihop/internal/agent"
	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
)

func setupTestServer(t *testing.T) *Server {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "api_test.db")
	store, err := graph.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("open store error: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})

	embedder := vector.NewBuiltinEmbedder(384)
	synth := agent.NewSynthesizer("builtin", "", "", "")
	coord := agent.NewCoordinator(store, embedder, synth)

	return NewServer(store, coord, 0)
}

func TestHealthEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if res["status"] != "ok" {
		t.Errorf("expected status ok, got %v", res["status"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestQueryEndpoint(t *testing.T) {
	srv := setupTestServer(t)

	// Ingest sample node
	srv.store.UpsertNode(&graph.Node{
		ID:         "apple_inc",
		Name:       "Apple Inc",
		EntityType: "ORGANIZATION",
		CreatedAt:  time.Now().UTC(),
	})

	queryBody, _ := json.Marshal(map[string]interface{}{
		"query": "Who is Apple Inc?",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewReader(queryBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}
}

func TestDocumentCreateAndList(t *testing.T) {
	srv := setupTestServer(t)

	body, _ := json.Marshal(map[string]string{
		"title":   "Vendor Guidelines",
		"content": "All vendors must comply with Section 12 on data protection.",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/documents", bytes.NewReader(body))
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/documents", nil)
	wList := httptest.NewRecorder()
	srv.mux.ServeHTTP(wList, reqList)

	if wList.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", wList.Code)
	}

	var docs []*parser.Document
	json.Unmarshal(wList.Body.Bytes(), &docs)
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
}
