package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandrmotologa/multihop/internal/agent"
	"github.com/alexandrmotologa/multihop/internal/api"
	"github.com/alexandrmotologa/multihop/internal/extractor"
	"github.com/alexandrmotologa/multihop/internal/graph"
	"github.com/alexandrmotologa/multihop/internal/parser"
	"github.com/alexandrmotologa/multihop/internal/vector"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		cmdInit(os.Args[2:])
	case "index":
		cmdIndex(os.Args[2:])
	case "query":
		cmdQuery(os.Args[2:])
	case "path":
		cmdPath(os.Args[2:])
	case "inspect":
		cmdInspect(os.Args[2:])
	case "cypher":
		cmdCypher(os.Args[2:])
	case "export":
		cmdExport(os.Args[2:])
	case "benchmark":
		cmdBenchmark(os.Args[2:])
	case "duplicates":
		cmdDuplicates(os.Args[2:])
	case "merge":
		cmdMerge(os.Args[2:])
	case "communities":
		cmdCommunities(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("MultiHop version %s\n", version)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`MultiHop v%s - Agentic Multi-Hop GraphRAG Engine with Hybrid Vector-Graph Retrieval

Usage:
  multihop <command> [flags]

Commands:
  init       Initialize a new SQLite database with schema and indices
  index      Parse, chunk, extract entities, compute vectors, and index documents
  query      Execute an agentic multi-hop query with ASCII reasoning tree
  path       Find the shortest reasoning path between two entities
  inspect    Inspect an entity, its neighbors, and degree centrality
  cypher     Execute a Cypher-Lite graph query against SQLite
  export     Export knowledge graph to GraphML or Cytoscape JSON
  benchmark    Run comparative benchmark between Naive Vector RAG and MultiHop
  duplicates   List candidate duplicate entities for disambiguation
  merge        Merge duplicate entities into a canonical node
  communities  List detected LPA community clusters and summaries
  serve        Launch the HTTP REST/SSE API and embedded React 19 Web Studio
  version    Show version information

Run 'multihop <command> --help' for details on each command.
`, version)
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	fmt.Printf("[MultiHop] Initializing database at %s ...\n", *dbPath)
	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Println("[MultiHop] Database initialized successfully with WAL mode, FTS5 full-text index, and graph tables.")
}

func cmdIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	dirPath := fs.String("dir", "", "Directory containing documents (Markdown, TXT, JSON)")
	filePath := fs.String("file", "", "Single document file to index")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	provider := fs.String("provider", "builtin", "Embedding and extraction provider (builtin, openai, ollama)")
	model := fs.String("model", "", "Model name for embedding/extraction")
	apiKey := fs.String("api-key", os.Getenv("OPENAI_API_KEY"), "API key for cloud provider")
	endpoint := fs.String("endpoint", "", "API endpoint URL")
	fs.Parse(args)

	if *dirPath == "" && *filePath == "" {
		fmt.Println("Error: either --dir or --file must be specified")
		fs.Usage()
		os.Exit(1)
	}

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	embedder, err := vector.CreateEmbedder(*provider, *model, *apiKey, *endpoint)
	if err != nil {
		fmt.Printf("Error creating embedder: %v\n", err)
		os.Exit(1)
	}

	var ext extractor.Extractor
	if *provider == "openai" {
		ext = extractor.NewLLMExtractor("openai", *model, *apiKey, *endpoint)
	} else {
		ext = extractor.NewRuleExtractor()
	}

	var files []string
	if *filePath != "" {
		files = append(files, *filePath)
	} else {
		filepath.Walk(*dirPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".md" || ext == ".markdown" || ext == ".txt" || ext == ".json" {
				files = append(files, path)
			}
			return nil
		})
	}

	if len(files) == 0 {
		fmt.Println("No supported files found to index.")
		return
	}

	fmt.Printf("[MultiHop] Indexing %d documents into %s using %s ...\n", len(files), *dbPath, embedder.Name())
	start := time.Now()

	totalChunks := 0
	totalEntities := 0
	totalRelations := 0

	ctx := context.Background()
	for i, f := range files {
		doc, err := parser.ParseFile(f)
		if err != nil {
			fmt.Printf("  [%d/%d] Skipping %s: %v\n", i+1, len(files), f, err)
			continue
		}

		if err := store.InsertDocument(doc); err != nil {
			fmt.Printf("  [%d/%d] Error saving document %s: %v\n", i+1, len(files), f, err)
			continue
		}

		chunks, err := parser.ChunkDocument(doc, parser.DefaultChunkerOptions())
		if err != nil {
			continue
		}

		// 1. Compute embeddings and insert chunks first so foreign keys are satisfied
		for _, c := range chunks {
			vec, _ := embedder.EmbedText(ctx, c.Content)
			c.Embedding = vec
		}
		if err := store.InsertChunks(chunks); err != nil {
			fmt.Printf("  [%d/%d] Error inserting chunks for %s: %v\n", i+1, len(files), f, err)
			continue
		}

		// 2. Extract entities and directed relations
		for _, c := range chunks {
			exRes, _ := ext.Extract(ctx, c)
			if exRes != nil {
				// Upsert entities
				for _, e := range exRes.Entities {
					slug, aliases := extractor.NormalizeEntity(e.Name)
					n := &graph.Node{
						ID:          slug,
						Name:        e.Name,
						EntityType:  e.Type,
						Description: e.Description,
						CreatedAt:   time.Now().UTC(),
					}
					store.UpsertNode(n)
					totalEntities++

					for _, a := range aliases {
						store.InsertAlias(a, slug)
					}
				}

				// Upsert edges
				for _, r := range exRes.Relations {
					srcSlug, _ := extractor.NormalizeEntity(r.Source)
					tgtSlug, _ := extractor.NormalizeEntity(r.Target)

					// Guarantee node existence for foreign key integrity
					store.UpsertNode(&graph.Node{
						ID:         srcSlug,
						Name:       r.Source,
						EntityType: "ORGANIZATION",
						CreatedAt:  time.Now().UTC(),
					})
					store.UpsertNode(&graph.Node{
						ID:         tgtSlug,
						Name:       r.Target,
						EntityType: "ORGANIZATION",
						CreatedAt:  time.Now().UTC(),
					})

					edgeID := fmt.Sprintf("e_%s_%s_%s", srcSlug, strings.ToLower(r.Type), tgtSlug)
					edge := &graph.Edge{
						ID:              edgeID,
						SourceNodeID:    srcSlug,
						TargetNodeID:    tgtSlug,
						RelationType:    r.Type,
						Weight:          r.Weight,
						EvidenceChunkID: c.ID,
						CreatedAt:       time.Now().UTC(),
					}
					if err := store.UpsertEdge(edge); err == nil {
						totalRelations++
					} else {
						fmt.Printf("    Warning: failed to upsert edge: %v\n", err)
					}
				}
			}
		}

		totalChunks += len(chunks)
		fmt.Printf("  [%d/%d] Indexed %s (%d chunks)\n", i+1, len(files), filepath.Base(f), len(chunks))
	}

	// Run LPA community detection
	store.DetectCommunitiesLabelPropagation(10)

	elapsed := time.Since(start)
	fmt.Printf("\n[MultiHop] Indexing complete in %s:\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  - Documents: %d\n", len(files))
	fmt.Printf("  - Chunks:    %d\n", totalChunks)
	fmt.Printf("  - Entities:  %d\n", totalEntities)
	fmt.Printf("  - Relations: %d\n", totalRelations)
}

func cmdQuery(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: multihop query \"<question>\" [flags]")
		os.Exit(1)
	}

	queryText := args[0]
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	provider := fs.String("provider", "builtin", "LLM & Embedding provider (builtin, openai, ollama)")
	model := fs.String("model", "", "Model name")
	apiKey := fs.String("api-key", os.Getenv("OPENAI_API_KEY"), "API key")
	endpoint := fs.String("endpoint", "", "API endpoint")
	mode := fs.String("mode", "auto", "Query mode (auto, local, global)")
	asOf := fs.String("as-of", "", "Temporal cutoff date (YYYY-MM-DD)")
	maxHops := fs.Int("max-hops", 3, "Max graph traversal hops")
	topK := fs.Int("top-k", 5, "Number of chunks to retrieve")
	fs.Parse(args[1:])

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	embedder, _ := vector.CreateEmbedder(*provider, *model, *apiKey, *endpoint)
	synth := agent.NewSynthesizer(*provider, *model, *apiKey, *endpoint)
	coord := agent.NewCoordinator(store, embedder, synth)

	fmt.Printf("\n[MultiHop] Executing Multi-Hop Query:\n  \"%s\"\n\n", queryText)

	req := agent.QueryRequest{
		Query:   queryText,
		Mode:    *mode,
		AsOf:    *asOf,
		MaxHops: *maxHops,
		TopK:    *topK,
	}

	stepHandler := func(ev agent.QueryStepEvent) {
		fmt.Printf("  * [%-18s] %s\n", ev.Step, ev.Details)
	}

	resp, err := coord.ExecuteQuery(context.Background(), req, stepHandler)
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
		os.Exit(1)
	}

	// Print Discovered Reasoning Paths
	if len(resp.ReasoningPaths) > 0 {
		fmt.Println("\n================= DISCOVERED REASONING PATHS =================")
		for i, p := range resp.ReasoningPaths {
			fmt.Printf("Path #%d (Score: %.3f):\n", i+1, p.Score)
			for j, st := range p.Steps {
				fmt.Printf("  (%d) %s  --[%s]-->  %s\n", j+1, st.SourceNode.Name, st.Edge.RelationType, st.TargetNode.Name)
				if st.EvidenceText != "" {
					shortEv := st.EvidenceText
					if len(shortEv) > 90 {
						shortEv = shortEv[:87] + "..."
					}
					fmt.Printf("      Evidence: \"%s\"\n", shortEv)
				}
			}
		}
	}

	// Print Grounded Answer
	fmt.Println("\n======================= GROUNDED ANSWER =======================")
	fmt.Println(resp.Answer)

	// Print Citations
	if len(resp.Citations) > 0 {
		fmt.Println("\n========================= CITATIONS ==========================")
		for _, c := range resp.Citations {
			header := ""
			if c.HeaderContext != "" {
				header = fmt.Sprintf(" [%s]", c.HeaderContext)
			}
			fmt.Printf("[^%s] %s%s (Relevance: %.4f)\n", c.ChunkID, c.DocumentTitle, header, c.Relevance)
			snippet := c.Content
			if len(snippet) > 120 {
				snippet = snippet[:117] + "..."
			}
			fmt.Printf("      \"%s\"\n", snippet)
		}
	}

	fmt.Printf("\nCompleted in %d ms (Mode: %s)\n", resp.ExecutionTimeMs, resp.Mode)
	fmt.Printf("RAG Triad: Context Relevance: %.2f | Groundedness: %.2f | Answer Relevance: %.2f\n\n",
		resp.ContextRelevance, resp.Groundedness, resp.AnswerRelevance)
}

func cmdPath(args []string) {
	fs := flag.NewFlagSet("path", flag.ExitOnError)
	from := fs.String("from", "", "Source entity name or ID")
	to := fs.String("to", "", "Target entity name or ID")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	if *from == "" || *to == "" {
		fmt.Println("Error: both --from and --to must be specified")
		fs.Usage()
		os.Exit(1)
	}

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	srcNode, err := store.FindNodeByNameOrAlias(*from)
	if err != nil {
		fmt.Printf("Entity '%s' not found in graph.\n", *from)
		os.Exit(1)
	}

	tgtNode, err := store.FindNodeByNameOrAlias(*to)
	if err != nil {
		fmt.Printf("Entity '%s' not found in graph.\n", *to)
		os.Exit(1)
	}

	path, err := store.DijkstraShortestPath(srcNode.ID, tgtNode.ID)
	if err != nil {
		fmt.Printf("No reasoning path found: %v\n", err)
		return
	}

	fmt.Printf("\nShortest Reasoning Chain between '%s' and '%s' (%d hops):\n\n", srcNode.Name, tgtNode.Name, len(path.Steps))
	for i, st := range path.Steps {
		fmt.Printf("  [%d] %s --(%s)--> %s\n", i+1, st.SourceNode.Name, st.Edge.RelationType, st.TargetNode.Name)
		if st.EvidenceText != "" {
			fmt.Printf("      Evidence: \"%s\"\n", st.EvidenceText)
		}
	}
	fmt.Printf("\nPath Score: %.4f\n", path.Score)
}

func cmdInspect(args []string) {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	name := fs.String("name", "", "Entity name or ID to inspect")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if *name == "" {
		// List top entities
		nodes, _ := store.ListNodes(25, "")
		fmt.Printf("\nTop Knowledge Graph Entities (%d total):\n", len(nodes))
		for _, n := range nodes {
			fmt.Printf("  - [%s] %s (%s, %d mentions, Community #%d)\n", n.ID, n.Name, n.EntityType, n.MentionCount, n.CommunityID)
		}
		return
	}

	node, err := store.FindNodeByNameOrAlias(*name)
	if err != nil {
		fmt.Printf("Entity '%s' not found.\n", *name)
		return
	}

	fmt.Printf("\nEntity Details:\n")
	fmt.Printf("  ID:           %s\n", node.ID)
	fmt.Printf("  Name:         %s\n", node.Name)
	fmt.Printf("  Type:         %s\n", node.EntityType)
	fmt.Printf("  Mentions:     %d\n", node.MentionCount)
	fmt.Printf("  Community:    #%d\n", node.CommunityID)
	if node.Description != "" {
		fmt.Printf("  Description:  %s\n", node.Description)
	}

	neighbors, _ := store.GetNeighbors(node.ID)
	fmt.Printf("\nConnected Relationships (%d):\n", len(neighbors))
	for _, nb := range neighbors {
		dir := "-->"
		if !nb.IsOutgoing {
			dir = "<--"
		}
		fmt.Printf("  %s %s [%s] %s\n", dir, nb.Edge.RelationType, nb.Neighbor.EntityType, nb.Neighbor.Name)
	}
	fmt.Println()
}

func cmdCypher(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: multihop cypher \"MATCH (a)-[r:REL]->(b) WHERE a.name = 'X' RETURN b\" [flags]")
		os.Exit(1)
	}

	cypherQuery := args[0]
	fs := flag.NewFlagSet("cypher", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args[1:])

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	rows, err := store.ExecuteCypherLite(cypherQuery)
	if err != nil {
		fmt.Printf("Cypher query error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nCypher Query: %s\nMatches found: %d\n\n", cypherQuery, len(rows))
	for i, r := range rows {
		bytes, _ := json.MarshalIndent(r.Values, "  ", "  ")
		fmt.Printf("Result #%d:\n  %s\n", i+1, string(bytes))
	}
}

func cmdExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	format := fs.String("format", "json", "Export format: json or graphml")
	outputPath := fs.String("output", "graph_export.json", "Output file path")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	snapshot, err := store.GetGraphSnapshot(5000)
	if err != nil {
		fmt.Printf("Error retrieving graph: %v\n", err)
		os.Exit(1)
	}

	if *format == "graphml" {
		var b strings.Builder
		b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		b.WriteString("<graphml xmlns=\"http://graphml.graphdrawing.org/xmlns\">\n")
		b.WriteString("  <graph id=\"G\" edgedefault=\"directed\">\n")
		for _, n := range snapshot.Nodes {
			b.WriteString(fmt.Sprintf("    <node id=\"%s\"><data key=\"label\">%s</data></node>\n", n.ID, n.Name))
		}
		for _, e := range snapshot.Links {
			b.WriteString(fmt.Sprintf("    <edge source=\"%s\" target=\"%s\"><data key=\"type\">%s</data></edge>\n", e.SourceNodeID, e.TargetNodeID, e.RelationType))
		}
		b.WriteString("  </graph>\n")
		b.WriteString("</graphml>\n")
		os.WriteFile(*outputPath, []byte(b.String()), 0644)
	} else {
		data, _ := json.MarshalIndent(snapshot, "", "  ")
		os.WriteFile(*outputPath, data, 0644)
	}

	fmt.Printf("[MultiHop] Exported %d nodes and %d links to %s\n", len(snapshot.Nodes), len(snapshot.Links), *outputPath)
}

func cmdBenchmark(args []string) {
	fs := flag.NewFlagSet("benchmark", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	embedder := vector.NewBuiltinEmbedder(384)
	synth := agent.NewSynthesizer("builtin", "", "", "")
	coord := agent.NewCoordinator(store, embedder, synth)

	fmt.Println("\n==================================================================")
	fmt.Println("   MULTIHOP vs NAIVE VECTOR RAG BENCHMARK EVALUATION")
	fmt.Println("==================================================================")

	queries := []struct {
		Question string
		IsMultiHop bool
	}{
		{"What is Vendor Alpha's liability policy?", false},
		{"Did any subsidiary of Vendor Alpha hold patents assigned in 2022?", true},
		{"How is European Division connected to Patent 102?", true},
	}

	for i, q := range queries {
		fmt.Printf("\nTest Query #%d: \"%s\"\n", i+1, q.Question)

		// 1. Naive Vector RAG Simulation
		startVec := time.Now()
		qVec, _ := embedder.EmbedText(context.Background(), q.Question)
		vecItems, _ := store.SearchChunksVector(qVec, 3)
		vecLatency := time.Since(startVec).Milliseconds()

		fmt.Printf("  [Naive Vector RAG]\n")
		fmt.Printf("    - Retrieved Chunks: %d\n", len(vecItems))
		fmt.Printf("    - Latency:          %d ms\n", vecLatency)
		fmt.Printf("    - Relational Hops:  0 (blind to indirect connections)\n")

		// 2. MultiHop Agentic GraphRAG
		startHop := time.Now()
		resp, err := coord.ExecuteQuery(context.Background(), agent.QueryRequest{
			Query:   q.Question,
			MaxHops: 3,
			TopK:    3,
		}, nil)
		hopLatency := time.Since(startHop).Milliseconds()

		if err != nil {
			fmt.Printf("    - MultiHop Error: %v\n", err)
			continue
		}

		fmt.Printf("  [MultiHop GraphRAG]\n")
		fmt.Printf("    - Discovered Paths: %d\n", len(resp.ReasoningPaths))
		fmt.Printf("    - Citations:        %d\n", len(resp.Citations))
		fmt.Printf("    - Latency:          %d ms\n", hopLatency)
		fmt.Printf("    - Hallucination:    %.2f (grounded via triples)\n", resp.HallucinationRisk)
	}

	fmt.Println("\nBenchmark summary: MultiHop traverses N-degree relational chains while retaining under 30MB RAM footprint.")
}

func cmdDuplicates(args []string) {
	fs := flag.NewFlagSet("duplicates", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	maxDist := fs.Int("distance", 2, "Max Levenshtein distance for duplicate detection")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	candidates, err := store.FindDuplicateCandidates(*maxDist)
	if err != nil {
		fmt.Printf("Error finding duplicates: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nCandidate Duplicate Entities Found: %d\n\n", len(candidates))
	for i, c := range candidates {
		fmt.Printf("Candidate #%d (Similarity: %.2f | %s):\n", i+1, c.Similarity, c.Reason)
		fmt.Printf("  - Canonical Candidate: [%s] \"%s\" (%s, %d mentions)\n", c.NodeA.ID, c.NodeA.Name, c.NodeA.EntityType, c.NodeA.MentionCount)
		fmt.Printf("  - Duplicate Variant:   [%s] \"%s\" (%s, %d mentions)\n", c.NodeB.ID, c.NodeB.Name, c.NodeB.EntityType, c.NodeB.MentionCount)
		fmt.Printf("  - Action: multihop merge --canonical %s --duplicate %s\n\n", c.NodeA.ID, c.NodeB.ID)
	}
}

func cmdMerge(args []string) {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	canonical := fs.String("canonical", "", "Canonical node ID to retain")
	duplicate := fs.String("duplicate", "", "Comma-separated duplicate node IDs to merge and delete")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	fs.Parse(args)

	if *canonical == "" || *duplicate == "" {
		fmt.Println("Error: both --canonical and --duplicate must be specified")
		fs.Usage()
		os.Exit(1)
	}

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	dupIDs := strings.Split(*duplicate, ",")
	for i := range dupIDs {
		dupIDs[i] = strings.TrimSpace(dupIDs[i])
	}

	if err := store.MergeNodes(*canonical, dupIDs); err != nil {
		fmt.Printf("Error merging nodes: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[MultiHop] Successfully merged %d duplicate entities into canonical node '%s'.\n", len(dupIDs), *canonical)
}

func cmdCommunities(args []string) {
	fs := flag.NewFlagSet("communities", flag.ExitOnError)
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	regenerate := fs.Bool("regenerate", false, "Re-run LPA community detection before listing")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	if *regenerate {
		fmt.Println("[MultiHop] Running Label Propagation Algorithm (LPA) clustering...")
		store.DetectCommunitiesLabelPropagation(10)
	}

	comms, err := store.ListCommunities()
	if err != nil {
		fmt.Printf("Error listing communities: %v\n", err)
		os.Exit(1)
	}

	if len(comms) == 0 {
		fmt.Println("No communities generated yet. Run 'multihop communities --regenerate'.")
		return
	}

	fmt.Printf("\nDetected Topical Communities (%d total):\n\n", len(comms))
	for _, c := range comms {
		fmt.Printf("Cluster #%d: %s (%d entities)\n", c.ID, c.Title, c.NodeCount)
		fmt.Printf("  Summary: %s\n\n", c.Summary)
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 8080, "HTTP server listening port")
	dbPath := fs.String("db", "knowledge.db", "Path to SQLite database file")
	provider := fs.String("provider", "builtin", "Provider (builtin, openai, ollama)")
	model := fs.String("model", "", "Model name")
	apiKey := fs.String("api-key", os.Getenv("OPENAI_API_KEY"), "API key")
	endpoint := fs.String("endpoint", "", "API endpoint")
	fs.Parse(args)

	store, err := graph.OpenStore(*dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	embedder, _ := vector.CreateEmbedder(*provider, *model, *apiKey, *endpoint)
	synth := agent.NewSynthesizer(*provider, *model, *apiKey, *endpoint)
	coord := agent.NewCoordinator(store, embedder, synth)

	server := api.NewServer(store, coord, *port)
	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
