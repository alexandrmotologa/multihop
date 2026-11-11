package extractor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/alexandrmotologa/multihop/internal/parser"
)

// RuleExtractor provides high-precision rule-based extraction for zero-dependency local execution.
type RuleExtractor struct{}

func NewRuleExtractor() *RuleExtractor {
	return &RuleExtractor{}
}

func (r *RuleExtractor) Name() string {
	return "builtin-rule-extractor"
}

var (
	// Regex for detecting capitalized entities (e.g. "Acme Corp", "Vendor Alpha", "Subsidiary Beta")
	entityRegex = regexp.MustCompile(`\b([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*(?:\s+(?:Inc|Corp|LLC|Ltd|GmbH|Co|Corporation|Company))?)\b`)
	patentRegex = regexp.MustCompile(`\b(Patent\s+(?:No\.\s*)?[0-9A-Z-]+)\b`)
	clauseRegex = regexp.MustCompile(`\b(Section\s+[0-9.]+|Clause\s+[0-9.]+|Article\s+[0-9.]+)\b`)
	moneyRegex  = regexp.MustCompile(`(\$\s*[0-9]+(?:,[0-9]{3})*(?:\.[0-9]+)?(?:\s*(?:million|billion|M|B))?)`)
)

func (r *RuleExtractor) Extract(ctx context.Context, chunk *parser.Chunk) (*ExtractionResult, error) {
	text := chunk.Content
	entityMap := make(map[string]ExtractedEntity)

	// Helper to register entity
	addEntity := func(name, eType, desc string) {
		name = strings.TrimSpace(name)
		name = strings.Trim(name, ",.;:\"'()")
		if len(name) < 2 {
			return
		}
		if _, exists := entityMap[name]; !exists {
			entityMap[name] = ExtractedEntity{
				Name:        name,
				Type:        eType,
				Description: desc,
			}
		}
	}

	// 1. Patents
	for _, m := range patentRegex.FindAllString(text, -1) {
		addEntity(m, "PATENT", "Intellectual property patent")
	}

	// 2. Clauses
	for _, m := range clauseRegex.FindAllString(text, -1) {
		addEntity(m, "CLAUSE", "Contractual clause or section")
	}

	// 3. Money / Liabilities
	for _, m := range moneyRegex.FindAllString(text, -1) {
		addEntity(m, "MONEY", "Monetary figure or liability amount")
	}

	// 4. Capitalized organizations and proper nouns
	stopWords := map[string]bool{
		"The": true, "This": true, "That": true, "In": true, "On": true, "At": true, "For": true,
		"Under": true, "Whereas": true, "Therefore": true, "Each": true, "Both": true, "All": true,
		"Party": true, "Section": true, "Clause": true, "Article": true,
	}

	for _, m := range entityRegex.FindAllString(text, -1) {
		trimmed := strings.TrimSpace(m)
		if stopWords[trimmed] {
			continue
		}

		eType := "ORGANIZATION"
		if strings.Contains(trimmed, "Corp") || strings.Contains(trimmed, "Inc") || strings.Contains(trimmed, "GmbH") || strings.Contains(trimmed, "LLC") {
			eType = "ORGANIZATION"
		} else if strings.HasPrefix(trimmed, "Patent") {
			eType = "PATENT"
		}
		addEntity(trimmed, eType, "Extracted entity from document text")
	}

	// 5. Relational Pattern Matching
	var relations []ExtractedRelation
	sentences := splitSentencesForExtractor(text)

	type relPattern struct {
		re      *regexp.Regexp
		relType string
	}

	patterns := []relPattern{
		{regexp.MustCompile(`(?i)(.+?)\s+(?:acquired|purchased|bought)\s+(.+?)(?:\s+in|\s+for|\s+on|\.|$)`), "ACQUIRED"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:owns|possesses)\s+(.+?)(?:\s+in|\s+for|\s+on|\.|$)`), "OWNS"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:is a subsidiary of|wholly owned by)\s+(.+?)(?:\.|$|\s+in)`), "SUBSIDIARY_OF"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:holds|has rights to)\s+(.+?)(?:\.|$|\s+under)`), "HOLDS"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:transferred|assigned)\s+(.+?)\s+to\s+(.+?)(?:\.|$|\s+in)`), "TRANSFERRED_TO"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:limits liability to|liability cap of)\s+(.+?)(?:\.|$|\s+for)`), "LIMITS_LIABILITY"},
		{regexp.MustCompile(`(?i)(.+?)\s+(?:located in|headquartered in)\s+(.+?)(?:\.|$|\s+since)`), "LOCATED_IN"},
	}

	for _, s := range sentences {
		sClean := strings.TrimSpace(s)
		for _, p := range patterns {
			matches := p.re.FindStringSubmatch(sClean)
			if len(matches) >= 3 {
				srcCandidate := strings.TrimSpace(matches[1])
				tgtCandidate := strings.TrimSpace(matches[2])

				// Match candidates with known entities
				src := findMatchingEntity(srcCandidate, entityMap)
				tgt := findMatchingEntity(tgtCandidate, entityMap)

				if src != "" && tgt != "" && src != tgt {
					relType := p.relType
					// Handle inverse relation for SUBSIDIARY_OF
					if relType == "SUBSIDIARY_OF" {
						relations = append(relations, ExtractedRelation{
							Source:   tgt,
							Target:   src,
							Type:     "OWNS",
							Weight:   1.0,
							Evidence: sClean,
						})
					} else {
						relations = append(relations, ExtractedRelation{
							Source:   src,
							Target:   tgt,
							Type:     relType,
							Weight:   1.0,
							Evidence: sClean,
						})
					}
				}
			}
		}
	}

	entities := make([]ExtractedEntity, 0, len(entityMap))
	for _, e := range entityMap {
		entities = append(entities, e)
	}

	return &ExtractionResult{
		Entities:  entities,
		Relations: relations,
	}, nil
}

func findMatchingEntity(text string, entityMap map[string]ExtractedEntity) string {
	for name := range entityMap {
		if strings.Contains(strings.ToLower(text), strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

func splitSentencesForExtractor(text string) []string {
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

// LLMExtractor uses an external language model API to perform structured triple extraction.
type LLMExtractor struct {
	provider   string
	model      string
	apiKey     string
	endpoint   string
	httpClient *http.Client
	fallback   *RuleExtractor
}

// NewLLMExtractor creates an LLM-powered extractor with automatic fallback to RuleExtractor.
func NewLLMExtractor(provider, model, apiKey, endpoint string) *LLMExtractor {
	if endpoint == "" {
		if provider == "openai" {
			endpoint = "https://api.openai.com/v1/chat/completions"
		} else if provider == "ollama" {
			endpoint = "http://localhost:11434/api/chat"
		}
	}
	return &LLMExtractor{
		provider:   provider,
		model:      model,
		apiKey:     apiKey,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 45 * time.Second},
		fallback:   NewRuleExtractor(),
	}
}

func (l *LLMExtractor) Name() string {
	return fmt.Sprintf("llm-%s-%s", l.provider, l.model)
}

func (l *LLMExtractor) Extract(ctx context.Context, chunk *parser.Chunk) (*ExtractionResult, error) {
	// If provider is builtin or local without keys, run rule extractor immediately
	if l.provider == "builtin" || l.provider == "local" || l.provider == "" {
		return l.fallback.Extract(ctx, chunk)
	}

	// Attempt LLM API call
	res, err := l.callAPI(ctx, chunk.Content)
	if err != nil {
		// Log and fallback to rule extractor so operations never stall
		return l.fallback.Extract(ctx, chunk)
	}

	// If LLM returned zero entities, merge with rule extractor findings
	if len(res.Entities) == 0 {
		ruleRes, _ := l.fallback.Extract(ctx, chunk)
		return ruleRes, nil
	}

	return res, nil
}

func (l *LLMExtractor) callAPI(ctx context.Context, text string) (*ExtractionResult, error) {
	if l.provider == "openai" {
		reqBody, _ := json.Marshal(map[string]interface{}{
			"model": l.model,
			"messages": []map[string]string{
				{"role": "system", "content": SystemExtractionPrompt},
				{"role": "user", "content": text},
			},
			"temperature": 0.0,
			"response_format": map[string]string{
				"type": "json_object",
			},
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.endpoint, bytes.NewReader(reqBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+l.apiKey)

		resp, err := l.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var openAIResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &openAIResp); err != nil || len(openAIResp.Choices) == 0 {
			return nil, fmt.Errorf("failed to parse openai response: %s", string(body))
		}

		var result ExtractionResult
		if err := json.Unmarshal([]byte(openAIResp.Choices[0].Message.Content), &result); err != nil {
			return nil, err
		}
		return &result, nil
	}

	return nil, fmt.Errorf("unsupported LLM provider: %s", l.provider)
}
