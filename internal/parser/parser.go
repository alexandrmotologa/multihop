package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ParseFile reads and parses a document from the filesystem.
func ParseFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}

	content := string(data)
	ext := strings.ToLower(filepath.Ext(path))
	baseName := filepath.Base(path)
	title := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	fileType := "text"
	switch ext {
	case ".md", ".markdown":
		fileType = "markdown"
		if mdTitle := extractMarkdownTitle(content); mdTitle != "" {
			title = mdTitle
		}
	case ".json":
		fileType = "json"
		if jsonTitle, text, err := extractJSONContent(data); err == nil && text != "" {
			content = text
			if jsonTitle != "" {
				title = jsonTitle
			}
		}
	case ".txt":
		fileType = "text"
	default:
		fileType = "unknown"
	}

	docID := "doc_" + ComputeHash(content + path)

	return &Document{
		ID:         docID,
		Title:      title,
		SourcePath: path,
		FileType:   fileType,
		ByteSize:   int64(len(data)),
		TokenCount: ApproximateTokens(content),
		Content:    content,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

// extractMarkdownTitle looks for the first # Header 1 in markdown text.
func extractMarkdownTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// extractJSONContent tries to parse structured JSON into linear text.
func extractJSONContent(data []byte) (string, string, error) {
	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err == nil {
		title := ""
		if t, ok := generic["title"].(string); ok {
			title = t
		}
		var builder strings.Builder
		for k, v := range generic {
			builder.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		return title, builder.String(), nil
	}

	var list []interface{}
	if err := json.Unmarshal(data, &list); err == nil {
		var builder strings.Builder
		for i, item := range list {
			builder.WriteString(fmt.Sprintf("[%d] %v\n", i, item))
		}
		return "", builder.String(), nil
	}

	return "", string(data), nil
}
