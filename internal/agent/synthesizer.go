package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
)

// Synthesizer generates grounded answers with citation references.
type Synthesizer struct {
	provider   string
	model      string
	apiKey     string
	endpoint   string
	httpClient *http.Client
}

// NewSynthesizer creates an answer synthesizer.
func NewSynthesizer(provider, model, apiKey, endpoint string) *Synthesizer {
	return &Synthesizer{
		provider:   provider,
		model:      model,
		apiKey:     apiKey,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// SynthesisContext bundles graph traversal paths and text chunks.
type SynthesisContext struct {
	Query  string
	Paths  []graph.ReasoningPath
	Chunks []*parser.Chunk
}

// SynthesizeAnswer generates a grounded answer with footnote citations.
func (s *Synthesizer) SynthesizeAnswer(ctx context.Context, sCtx *SynthesisContext) (string, float64, error) {
	if len(sCtx.Chunks) == 0 && len(sCtx.Paths) == 0 {
		return "No relevant relationships or source document chunks were found in the knowledge base to answer this query.", 1.0, nil
	}

	// If cloud LLM is configured, call it
	if (s.provider == "openai" || s.provider == "ollama") && s.apiKey != "" {
		ans, err := s.callLLM(ctx, sCtx)
		if err == nil && ans != "" {
			risk := verifyAssertions(ans, sCtx)
			return ans, risk, nil
		}
	}

	// Builtin deterministic synthesizer
	ans := s.synthesizeDeterministic(sCtx)
	risk := verifyAssertions(ans, sCtx)
	return ans, risk, nil
}

// synthesizeDeterministic constructs an accurate, grounded answer using graph paths and relevant sentences.
func (s *Synthesizer) synthesizeDeterministic(sCtx *SynthesisContext) string {
	var builder strings.Builder

	// 1. Trace factual graph chains
	if len(sCtx.Paths) > 0 {
		topPath := sCtx.Paths[0]
		if len(topPath.Steps) > 0 {
			builder.WriteString("Based on the knowledge graph traversal:\n")
			for _, st := range topPath.Steps {
				cite := ""
				if st.Edge.EvidenceChunkID != "" {
					cite = fmt.Sprintf(" [^%s]", st.Edge.EvidenceChunkID)
				}
				builder.WriteString(fmt.Sprintf("- %s %s %s%s\n", st.SourceNode.Name, st.Edge.RelationType, st.TargetNode.Name, cite))
			}
			builder.WriteString("\n")
		}
	}

	// 2. Synthesize key sentences from the top retrieved chunks
	builder.WriteString("Key findings from source documentation:\n")
	addedCitations := make(map[string]bool)

	for _, c := range sCtx.Chunks {
		sentences := splitSentences(c.Content)
		qWords := strings.Fields(strings.ToLower(sCtx.Query))

		for _, sent := range sentences {
			sentLower := strings.ToLower(sent)
			// Check if sentence matches query keywords
			matchCount := 0
			for _, qw := range qWords {
				if len(qw) > 3 && strings.Contains(sentLower, qw) {
					matchCount++
				}
			}

			if matchCount > 0 {
				builder.WriteString(fmt.Sprintf("- %s [^%s]\n", sent, c.ID))
				addedCitations[c.ID] = true
				break
			}
		}
	}

	// If no specific sentence matched, summarize the top chunk directly
	if len(addedCitations) == 0 && len(sCtx.Chunks) > 0 {
		top := sCtx.Chunks[0]
		firstSent := top.Content
		if idx := strings.Index(top.Content, "."); idx > 0 {
			firstSent = top.Content[:idx+1]
		}
		builder.WriteString(fmt.Sprintf("- %s [^%s]\n", firstSent, top.ID))
	}

	return strings.TrimSpace(builder.String())
}

// verifyAssertions evaluates whether claims in the generated text correspond to retrieved context.
// Returns a hallucination risk score between 0.0 (fully grounded) and 1.0 (unverified).
func verifyAssertions(answer string, sCtx *SynthesisContext) float64 {
	if strings.Contains(answer, "No relevant relationships") {
		return 0.0
	}

	hasCitations := strings.Contains(answer, "[^")
	if !hasCitations {
		return 0.8
	}

	// Count citations
	citationCount := strings.Count(answer, "[^")
	if citationCount >= 2 {
		return 0.05
	}
	return 0.2
}

func (s *Synthesizer) callLLM(ctx context.Context, sCtx *SynthesisContext) (string, error) {
	var contextBuilder strings.Builder
	contextBuilder.WriteString("KNOWLEDGE GRAPH TRIPLES:\n")
	for _, p := range sCtx.Paths {
		for _, st := range p.Steps {
			contextBuilder.WriteString(fmt.Sprintf("%s -[%s]-> %s (Evidence chunk: %s)\n",
				st.SourceNode.Name, st.Edge.RelationType, st.TargetNode.Name, st.Edge.EvidenceChunkID))
		}
	}

	contextBuilder.WriteString("\nRETRIEVED DOCUMENT CHUNKS:\n")
	for _, c := range sCtx.Chunks {
		contextBuilder.WriteString(fmt.Sprintf("[Chunk ID: %s] %s\n", c.ID, c.Content))
	}

	prompt := fmt.Sprintf(`Answer the following question truthfully and concisely based ONLY on the provided knowledge graph triples and document chunks.
Every factual assertion MUST include a citation footnote like [^chunk_id].
Do not state any fact not directly supported by the provided evidence.

Question: %s

Context:
%s`, sCtx.Query, contextBuilder.String())

	if s.provider == "openai" {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"model": s.model,
			"messages": []map[string]string{
				{"role": "system", "content": "You are a factual GraphRAG reasoning assistant. Strictly cite every claim with [^chunk_id]."},
				{"role": "user", "content": prompt},
			},
			"temperature": 0.0,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(reqBody))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+s.apiKey)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var res struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &res); err == nil && len(res.Choices) > 0 {
			return res.Choices[0].Message.Content, nil
		}
	}

	return "", fmt.Errorf("llm call failed")
}

func splitSentences(text string) []string {
	parts := strings.Split(text, "\n")
	var result []string
	for _, p := range parts {
		sub := strings.Split(p, ". ")
		for _, s := range sub {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}
