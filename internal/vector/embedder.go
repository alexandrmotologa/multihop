package vector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strings"
	"time"
)

// Embedder represents a text vector embedding generator.
type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
	Name() string
}

// BuiltinEmbedder provides a deterministic, zero-dependency 384-dimensional embedder.
// Uses feature hashing (char n-grams + word tokens) with sign hashing and L2 normalization.
type BuiltinEmbedder struct {
	dim int
}

// NewBuiltinEmbedder creates a deterministic local embedder with the specified dimension (default 384).
func NewBuiltinEmbedder(dim int) *BuiltinEmbedder {
	if dim <= 0 {
		dim = 384
	}
	return &BuiltinEmbedder{dim: dim}
}

func (b *BuiltinEmbedder) Dimension() int {
	return b.dim
}

func (b *BuiltinEmbedder) Name() string {
	return "builtin-deterministic-384"
}

func (b *BuiltinEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	vec := make([]float32, b.dim)
	clean := strings.ToLower(strings.TrimSpace(text))
	if clean == "" {
		return vec, nil
	}

	words := strings.Fields(clean)
	for i, w := range words {
		// Clean punctuation
		wClean := strings.Trim(w, ",.!?:;\"'()[]{}<>-")
		if wClean == "" {
			continue
		}

		// Positional weight: early words in sentences carry slightly more weight
		posWeight := 1.0 + (1.0 / float32(i+1))

		// Word level hash
		hWord := hashFNV(wClean)
		idx := int(hWord % uint64(b.dim))
		sign := float32(1.0)
		if (hWord >> 32)&1 == 1 {
			sign = -1.0
		}
		vec[idx] += sign * posWeight * 2.0

		// Character n-grams (3-grams) for morphological similarity
		runes := []rune(wClean)
		if len(runes) >= 3 {
			for j := 0; j <= len(runes)-3; j++ {
				ngram := string(runes[j : j+3])
				hNgram := hashFNV(ngram)
				nIdx := int(hNgram % uint64(b.dim))
				nSign := float32(1.0)
				if (hNgram >> 32)&1 == 1 {
					nSign = -1.0
				}
				vec[nIdx] += nSign * 0.5
			}
		}
	}

	NormalizeL2(vec)
	return vec, nil
}

func (b *BuiltinEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v, err := b.EmbedText(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func hashFNV(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// OpenAIEmbedder calls the OpenAI embeddings API.
type OpenAIEmbedder struct {
	apiKey   string
	model    string
	endpoint string
	dim      int
	client   *http.Client
}

// NewOpenAIEmbedder creates an OpenAI-compatible embedding client.
func NewOpenAIEmbedder(apiKey string, model string, endpoint string) *OpenAIEmbedder {
	if model == "" {
		model = "text-embedding-3-small"
	}
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1/embeddings"
	}
	dim := 1536
	if model == "text-embedding-3-large" {
		dim = 3072
	}
	return &OpenAIEmbedder{
		apiKey:   apiKey,
		model:    model,
		endpoint: endpoint,
		dim:      dim,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OpenAIEmbedder) Dimension() int { return o.dim }
func (o *OpenAIEmbedder) Name() string   { return fmt.Sprintf("openai-%s", o.model) }

func (o *OpenAIEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	batch, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(batch) == 0 {
		return nil, errors.New("empty response from embedding API")
	}
	return batch[0], nil
}

type openAIEmbedRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openAIEmbedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(openAIEmbedRequest{
		Input: texts,
		Model: o.model,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embedding request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai embedding API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res openAIEmbedResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("decode embedding json: %w", err)
	}

	if res.Error != nil && res.Error.Message != "" {
		return nil, fmt.Errorf("openai embedding error: %s", res.Error.Message)
	}

	results := make([][]float32, len(texts))
	for _, item := range res.Data {
		if item.Index < len(results) {
			results[item.Index] = item.Embedding
		}
	}

	return results, nil
}

// OllamaEmbedder connects to a local Ollama instance for embeddings.
type OllamaEmbedder struct {
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOllamaEmbedder creates an Ollama embedding client.
func NewOllamaEmbedder(baseURL string, model string) *OllamaEmbedder {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbedder{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		dim:     768,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (ol *OllamaEmbedder) Dimension() int { return ol.dim }
func (ol *OllamaEmbedder) Name() string   { return fmt.Sprintf("ollama-%s", ol.model) }

type ollamaEmbedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (ol *OllamaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	reqBody, _ := json.Marshal(ollamaEmbedRequest{
		Model: ol.model,
		Input: text,
	})
	url := fmt.Sprintf("%s/api/embeddings", ol.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ol.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var res ollamaEmbedResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("unmarshal ollama response: %w", err)
	}
	return res.Embedding, nil
}

func (ol *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := ol.EmbedText(ctx, text)
		if err != nil {
			return nil, err
		}
		out[i] = vec
	}
	return out, nil
}

// Config creates an Embedder from configuration parameters.
func CreateEmbedder(provider string, model string, apiKey string, endpoint string) (Embedder, error) {
	switch strings.ToLower(provider) {
	case "openai":
		if apiKey == "" {
			return nil, errors.New("openai apiKey is required")
		}
		return NewOpenAIEmbedder(apiKey, model, endpoint), nil
	case "ollama":
		return NewOllamaEmbedder(endpoint, model), nil
	case "builtin", "local", "":
		return NewBuiltinEmbedder(384), nil
	default:
		return nil, fmt.Errorf("unsupported embedder provider: %s", provider)
	}
}
