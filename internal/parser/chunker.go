package parser

import (
	"fmt"
	"strings"
	"time"
)

// ChunkDocument splits a document into semantically bounded chunks.
func ChunkDocument(doc *Document, opts ChunkerOptions) ([]*Chunk, error) {
	if opts.TargetTokens <= 0 {
		opts = DefaultChunkerOptions()
	}

	if doc.FileType == "markdown" {
		return chunkMarkdown(doc, opts)
	}
	return chunkPlainText(doc, opts)
}

// headerNode tracks markdown heading hierarchy.
type headerNode struct {
	level int
	title string
}

func formatBreadcrumbs(hierarchy []headerNode) string {
	var titles []string
	for _, h := range hierarchy {
		titles = append(titles, h.title)
	}
	return strings.Join(titles, " > ")
}

// chunkMarkdown performs heading-aware chunking on markdown text.
func chunkMarkdown(doc *Document, opts ChunkerOptions) ([]*Chunk, error) {
	lines := strings.Split(doc.Content, "\n")
	var chunks []*Chunk

	var currentHierarchy []headerNode
	var currentSectionLines []string

	flushSection := func() {
		if len(currentSectionLines) == 0 {
			return
		}
		sectionText := strings.TrimSpace(strings.Join(currentSectionLines, "\n"))
		currentSectionLines = nil
		if sectionText == "" {
			return
		}

		breadcrumbs := formatBreadcrumbs(currentHierarchy)
		subChunks := splitTextIntoSlices(sectionText, breadcrumbs, doc.ID, doc.Title, len(chunks), opts)
		chunks = append(chunks, subChunks...)
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			// Heading line
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level <= 6 && level < len(trimmed) && trimmed[level] == ' ' {
				flushSection()
				headingText := strings.TrimSpace(trimmed[level:])

				// Update hierarchy
				for len(currentHierarchy) > 0 && currentHierarchy[len(currentHierarchy)-1].level >= level {
					currentHierarchy = currentHierarchy[:len(currentHierarchy)-1]
				}
				currentHierarchy = append(currentHierarchy, headerNode{level: level, title: headingText})
				continue
			}
		}

		currentSectionLines = append(currentSectionLines, line)
	}

	flushSection()

	// If no chunks were produced (e.g. empty or only headers)
	if len(chunks) == 0 && strings.TrimSpace(doc.Content) != "" {
		return chunkPlainText(doc, opts)
	}

	return chunks, nil
}

// chunkPlainText performs paragraph and sliding window chunking on raw text.
func chunkPlainText(doc *Document, opts ChunkerOptions) ([]*Chunk, error) {
	return splitTextIntoSlices(doc.Content, "", doc.ID, doc.Title, 0, opts), nil
}

// splitTextIntoSlices breaks text into paragraphs, merging until target token count is reached.
func splitTextIntoSlices(text string, headerContext string, docID string, docTitle string, startIndex int, opts ChunkerOptions) []*Chunk {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []*Chunk

	var currentParagraphs []string
	currentTokens := 0

	flush := func() {
		if len(currentParagraphs) == 0 {
			return
		}
		content := strings.TrimSpace(strings.Join(currentParagraphs, "\n\n"))
		if content == "" {
			currentParagraphs = nil
			currentTokens = 0
			return
		}

		chunkIdx := startIndex + len(chunks)
		chunkID := fmt.Sprintf("chunk_%s_%04d", docID, chunkIdx)

		chunks = append(chunks, &Chunk{
			ID:            chunkID,
			DocumentID:    docID,
			DocumentTitle: docTitle,
			ChunkIndex:    chunkIdx,
			Content:       content,
			TokenCount:    ApproximateTokens(content),
			HeaderContext: headerContext,
			CreatedAt:     time.Now().UTC(),
		})

		// Overlap retention: keep the last paragraph if overlap is configured
		if opts.OverlapTokens > 0 && len(currentParagraphs) > 1 {
			lastPara := currentParagraphs[len(currentParagraphs)-1]
			lastParaTokens := ApproximateTokens(lastPara)
			if lastParaTokens <= opts.OverlapTokens {
				currentParagraphs = []string{lastPara}
				currentTokens = lastParaTokens
				return
			}
		}

		currentParagraphs = nil
		currentTokens = 0
	}

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}

		pTokens := ApproximateTokens(trimmed)
		// If a single paragraph is larger than 1.5x target tokens, split by sentences
		if pTokens > int(float64(opts.TargetTokens)*1.5) {
			flush()
			sentenceSlices := splitLongParagraph(trimmed, headerContext, docID, docTitle, startIndex+len(chunks), opts)
			chunks = append(chunks, sentenceSlices...)
			continue
		}

		if currentTokens+pTokens > opts.TargetTokens && currentTokens >= opts.MinTokens {
			flush()
		}

		currentParagraphs = append(currentParagraphs, trimmed)
		currentTokens += pTokens
	}

	flush()
	return chunks
}

// splitLongParagraph breaks oversized paragraphs across sentence boundaries.
func splitLongParagraph(paragraph string, headerContext string, docID string, docTitle string, startIndex int, opts ChunkerOptions) []*Chunk {
	sentences := splitSentences(paragraph)
	var chunks []*Chunk

	var currentSentences []string
	currentTokens := 0

	flush := func() {
		if len(currentSentences) == 0 {
			return
		}
		content := strings.TrimSpace(strings.Join(currentSentences, " "))
		if content == "" {
			currentSentences = nil
			currentTokens = 0
			return
		}

		chunkIdx := startIndex + len(chunks)
		chunkID := fmt.Sprintf("chunk_%s_%04d", docID, chunkIdx)

		chunks = append(chunks, &Chunk{
			ID:            chunkID,
			DocumentID:    docID,
			DocumentTitle: docTitle,
			ChunkIndex:    chunkIdx,
			Content:       content,
			TokenCount:    ApproximateTokens(content),
			HeaderContext: headerContext,
			CreatedAt:     time.Now().UTC(),
		})

		currentSentences = nil
		currentTokens = 0
	}

	for _, s := range sentences {
		sTokens := ApproximateTokens(s)
		if currentTokens+sTokens > opts.TargetTokens && currentTokens >= opts.MinTokens {
			flush()
		}
		currentSentences = append(currentSentences, s)
		currentTokens += sTokens
	}

	flush()
	return chunks
}

// splitSentences isolates sentences by looking for period, exclamation, question mark delimiters.
func splitSentences(text string) []string {
	var sentences []string
	var cur strings.Builder

	runes := []rune(text)
	n := len(runes)

	for i := 0; i < n; i++ {
		cur.WriteRune(runes[i])
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			if i+1 < n && (runes[i+1] == ' ' || runes[i+1] == '\n' || runes[i+1] == '\t') {
				s := strings.TrimSpace(cur.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				cur.Reset()
				i++ // skip whitespace
			}
		}
	}

	if cur.Len() > 0 {
		s := strings.TrimSpace(cur.String())
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	return sentences
}
