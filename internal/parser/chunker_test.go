package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFileMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "contract.md")
	content := "# Master Service Agreement\n\nThis agreement is made between Party A and Party B.\n\n## Section 1: Liability\n\nParty A shall not be liable for incidental damages.\n"

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	doc, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if doc.Title != "Master Service Agreement" {
		t.Errorf("expected title 'Master Service Agreement', got '%s'", doc.Title)
	}

	if doc.FileType != "markdown" {
		t.Errorf("expected fileType 'markdown', got '%s'", doc.FileType)
	}

	chunks, err := ChunkDocument(doc, DefaultChunkerOptions())
	if err != nil {
		t.Fatalf("ChunkDocument returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatalf("expected at least 1 chunk, got 0")
	}

	// Verify header breadcrumbs exist
	foundHeader := false
	for _, c := range chunks {
		if c.HeaderContext != "" {
			foundHeader = true
			break
		}
	}
	if !foundHeader {
		t.Errorf("expected at least one chunk to have header context")
	}
}

func TestParseFileJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "data.json")
	content := `{"title": "Vendor Profile", "vendor": "Acme Corp", "subsidiaries": ["SubA", "SubB"]}`

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	doc, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}

	if doc.Title != "Vendor Profile" {
		t.Errorf("expected title 'Vendor Profile', got '%s'", doc.Title)
	}

	chunks, err := ChunkDocument(doc, DefaultChunkerOptions())
	if err != nil {
		t.Fatalf("ChunkDocument returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatalf("expected at least 1 chunk from JSON, got 0")
	}
}
