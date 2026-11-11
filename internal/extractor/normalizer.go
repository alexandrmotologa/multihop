package extractor

import (
	"strings"
	"unicode"
)

// NormalizeEntity creates a canonical slug and extracts alias variations.
func NormalizeEntity(rawName string) (canonicalID string, aliases []string) {
	clean := strings.TrimSpace(rawName)
	clean = strings.Trim(clean, ".,;:\"'()[]{}<>-")
	if clean == "" {
		return "unknown", nil
	}

	lower := strings.ToLower(clean)

	// Strip common legal entity suffixes for canonical ID derivation
	stripped := lower
	suffixes := []string{
		" inc.", " inc", " corp.", " corp", " corporation", " llc", " ltd.", " ltd",
		" gmbh", " s.a.", " sa", " co.", " co", " company",
	}
	changed := true
	for changed {
		changed = false
		for _, sfx := range suffixes {
			if strings.HasSuffix(stripped, sfx) {
				stripped = strings.TrimSpace(strings.TrimSuffix(stripped, sfx))
				stripped = strings.Trim(stripped, ".,;:\"'()[]{}<>-")
				changed = true
				break
			}
		}
	}

	// Build slug: replace non-alphanumeric with underscores
	var slugBuilder strings.Builder
	for _, r := range stripped {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			slugBuilder.WriteRune(r)
		} else if slugBuilder.Len() > 0 && !strings.HasSuffix(slugBuilder.String(), "_") {
			slugBuilder.WriteRune('_')
		}
	}
	slug := strings.Trim(slugBuilder.String(), "_")
	if slug == "" {
		slug = "entity"
	}

	var aliasList []string
	if lower != slug {
		aliasList = append(aliasList, clean)
	}
	if stripped != lower && stripped != slug {
		aliasList = append(aliasList, stripped)
	}

	return slug, aliasList
}

// LevenshteinDistance calculates the edit distance between two strings.
func LevenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			dp[i][j] = min(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[n][m]
}

func min(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}
