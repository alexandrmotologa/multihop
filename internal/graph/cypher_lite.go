package graph

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// CypherPattern represents a parsed pattern matching clause.
type CypherPattern struct {
	SourceVar   string
	SourceType  string
	RelVar      string
	RelType     string
	MinHops     int
	MaxHops     int
	TargetVar   string
	TargetType  string
	WhereKey    string
	WhereVal    string
	ReturnVars  []string
}

// CypherResultRow holds matches for matched variables.
type CypherResultRow struct {
	Values map[string]interface{} `json:"values"`
}

var (
	// Matches: MATCH (a:TYPE)-[r:REL*1..3]->(b:TYPE) WHERE ... RETURN ...
	cypherRegex = regexp.MustCompile(`(?i)MATCH\s*\(([a-zA-Z0-9_]*)(?::([a-zA-Z0-9_]+))?\)\s*-\[\s*([a-zA-Z0-9_]*)(?::([a-zA-Z0-9_]+))?(?:\*([0-9]+)\.\.([0-9]+))?\s*\]->\s*\(([a-zA-Z0-9_]*)(?::([a-zA-Z0-9_]+))?\)(?:\s+WHERE\s+([a-zA-Z0-9_.]+)\s*=\s*['"]([^'"]+)['"])?(?:\s+RETURN\s+([a-zA-Z0-9_,\s]+))?`)
)

// ParseCypherLite parses a micro-Cypher query string.
func ParseCypherLite(query string) (*CypherPattern, error) {
	clean := strings.TrimSpace(query)
	matches := cypherRegex.FindStringSubmatch(clean)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid Cypher-Lite syntax: '%s'. Expected format: MATCH (a:TYPE)-[r:REL*1..3]->(b) WHERE a.name = 'X' RETURN b", query)
	}

	pat := &CypherPattern{
		SourceVar:  matches[1],
		SourceType: matches[2],
		RelVar:     matches[3],
		RelType:    matches[4],
		MinHops:    1,
		MaxHops:    1,
		TargetVar:  matches[7],
		TargetType: matches[8],
		WhereKey:   matches[9],
		WhereVal:   matches[10],
	}

	if matches[5] != "" {
		pat.MinHops, _ = strconv.Atoi(matches[5])
	}
	if matches[6] != "" {
		pat.MaxHops, _ = strconv.Atoi(matches[6])
	}

	if matches[11] != "" {
		rawVars := strings.Split(matches[11], ",")
		for _, v := range rawVars {
			vClean := strings.TrimSpace(v)
			if vClean != "" {
				pat.ReturnVars = append(pat.ReturnVars, vClean)
			}
		}
	} else {
		pat.ReturnVars = []string{pat.TargetVar}
	}

	return pat, nil
}

// ExecuteCypherLite executes a parsed Cypher query via SQLite recursive CTEs.
func (s *Store) ExecuteCypherLite(query string) ([]CypherResultRow, error) {
	pat, err := ParseCypherLite(query)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// If 1-hop without variable length, use direct join
	if pat.MaxHops <= 1 {
		sqlQuery := `SELECT a.id, a.name, a.entity_type, e.id, e.relation_type, e.weight, b.id, b.name, b.entity_type
                     FROM nodes a
                     JOIN edges e ON a.id = e.source_node_id
                     JOIN nodes b ON e.target_node_id = b.id
                     WHERE 1=1`
		var args []interface{}

		if pat.SourceType != "" {
			sqlQuery += ` AND a.entity_type = ?`
			args = append(args, strings.ToUpper(pat.SourceType))
		}
		if pat.TargetType != "" {
			sqlQuery += ` AND b.entity_type = ?`
			args = append(args, strings.ToUpper(pat.TargetType))
		}
		if pat.RelType != "" {
			sqlQuery += ` AND e.relation_type = ?`
			args = append(args, strings.ToUpper(pat.RelType))
		}
		if pat.WhereKey != "" {
			col := "name"
			if strings.HasSuffix(strings.ToLower(pat.WhereKey), ".id") {
				col = "id"
			}
			sqlQuery += fmt.Sprintf(` AND a.%s = ?`, col)
			args = append(args, pat.WhereVal)
		}

		rows, err := s.db.Query(sqlQuery, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var results []CypherResultRow
		for rows.Next() {
			var aID, aName, aType, eID, eRel string
			var eWeight float64
			var bID, bName, bType string

			if err := rows.Scan(&aID, &aName, &aType, &eID, &eRel, &eWeight, &bID, &bName, &bType); err != nil {
				continue
			}

			rowMap := make(map[string]interface{})
			rowMap[pat.SourceVar] = map[string]interface{}{"id": aID, "name": aName, "type": aType}
			if pat.RelVar != "" {
				rowMap[pat.RelVar] = map[string]interface{}{"id": eID, "relation": eRel, "weight": eWeight}
			}
			rowMap[pat.TargetVar] = map[string]interface{}{"id": bID, "name": bName, "type": bType}

			results = append(results, CypherResultRow{Values: rowMap})
		}
		return results, nil
	}

	// Multi-hop recursive CTE
	cteSQL := fmt.Sprintf(`
    WITH RECURSIVE traversal(src_id, curr_id, depth, path) AS (
        SELECT a.id, a.id, 0, a.name
        FROM nodes a
        WHERE 1=1 %s
        UNION ALL
        SELECT t.src_id, e.target_node_id, t.depth + 1, t.path || ' -> ' || e.relation_type || ' -> ' || b.name
        FROM traversal t
        JOIN edges e ON t.curr_id = e.source_node_id
        JOIN nodes b ON e.target_node_id = b.id
        WHERE t.depth < ? %s
    )
    SELECT DISTINCT b.id, b.name, b.entity_type, t.depth, t.path
    FROM traversal t
    JOIN nodes b ON t.curr_id = b.id
    WHERE t.depth >= ?`,
		buildWhereClause(pat.WhereKey, pat.WhereVal, pat.SourceType),
		buildRelFilter(pat.RelType),
	)

	var args []interface{}
	if pat.WhereVal != "" {
		args = append(args, pat.WhereVal)
	}
	if pat.SourceType != "" {
		args = append(args, strings.ToUpper(pat.SourceType))
	}
	args = append(args, pat.MaxHops)
	if pat.RelType != "" {
		args = append(args, strings.ToUpper(pat.RelType))
	}
	args = append(args, pat.MinHops)

	rows, err := s.db.Query(cteSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CypherResultRow
	for rows.Next() {
		var bID, bName, bType, path string
		var depth int
		if err := rows.Scan(&bID, &bName, &bType, &depth, &path); err == nil {
			rowMap := map[string]interface{}{
				pat.TargetVar: map[string]interface{}{
					"id":    bID,
					"name":  bName,
					"type":  bType,
					"depth": depth,
					"path":  path,
				},
			}
			results = append(results, CypherResultRow{Values: rowMap})
		}
	}

	return results, nil
}

func buildWhereClause(key, val, srcType string) string {
	clause := ""
	if val != "" {
		col := "name"
		if strings.HasSuffix(strings.ToLower(key), ".id") {
			col = "id"
		}
		clause += fmt.Sprintf(` AND a.%s = ?`, col)
	}
	if srcType != "" {
		clause += ` AND a.entity_type = ?`
	}
	return clause
}

func buildRelFilter(relType string) string {
	if relType != "" {
		return ` AND e.relation_type = ?`
	}
	return ""
}
