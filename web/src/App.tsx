import React, { useState, useEffect, useRef } from 'react';
import { 
  Network, 
  Search, 
  FileText, 
  Activity, 
  ChevronRight, 
  Sparkles, 
  Cpu, 
  Send,
  Loader2,
  GitBranch,
  ShieldCheck,
  ShieldAlert,
  ArrowRight,
  Database,
  Layers,
  CheckCircle2,
  Bookmark
} from 'lucide-react';
import GraphCanvas, { GraphNode, GraphLink } from './components/GraphCanvas';
import SourceDrawer, { ChunkDetail } from './components/SourceDrawer';
import BenchmarkView from './components/BenchmarkView';

interface Citation {
  index: number;
  chunk_id: string;
  document_id: string;
  document_title: string;
  header_context?: string;
  content: string;
  relevance: number;
}

interface ReasoningPath {
  steps: {
    source_node: GraphNode;
    edge: GraphLink;
    target_node: GraphNode;
    evidence_text?: string;
    score: number;
  }[];
  total_weight: number;
  score: number;
}

interface QueryResponse {
  query: string;
  answer: string;
  sub_goals: string[];
  seed_entities: GraphNode[];
  reasoning_paths: ReasoningPath[];
  citations: Citation[];
  execution_time_ms: number;
  hallucination_risk: number;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<'studio' | 'graph' | 'documents' | 'benchmark'>('studio');
  const [stats, setStats] = useState({ documents: 0, chunks: 0, nodes: 0, edges: 0 });
  const [health, setHealth] = useState({ status: 'ok', memory_alloc_mb: 0, goroutines: 0 });

  // Graph state
  const [graphData, setGraphData] = useState<{ nodes: GraphNode[]; links: GraphLink[] }>({ nodes: [], links: [] });
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);

  // Studio query state
  const [queryInput, setQueryInput] = useState('Did any subsidiary of Vendor Alpha hold patents?');
  const [queryLoading, setQueryLoading] = useState(false);
  const [queryResponse, setQueryResponse] = useState<QueryResponse | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);

  // Documents state
  const [documents, setDocuments] = useState<any[]>([]);

  useEffect(() => {
    refreshStats();
    loadGraph();
    loadDocuments();
  }, []);

  const refreshStats = () => {
    fetch('/api/stats')
      .then(res => res.json())
      .then(data => setStats(data))
      .catch(() => {});

    fetch('/api/health')
      .then(res => res.json())
      .then(data => setHealth(data))
      .catch(() => {});
  };

  const loadGraph = () => {
    fetch('/api/graph?limit=500')
      .then(res => res.json())
      .then(data => {
        if (data && data.nodes && data.links) {
          setGraphData({
            nodes: data.nodes,
            links: data.links,
          });
        }
      })
      .catch(() => {});
  };

  const loadDocuments = () => {
    fetch('/api/documents')
      .then(res => res.json())
      .then(data => setDocuments(data || []))
      .catch(() => {});
  };

  const handleRunQuery = async (queryText?: string) => {
    const q = queryText || queryInput;
    if (!q.trim() || queryLoading) return;

    setQueryLoading(true);
    setQueryResponse(null);

    try {
      const res = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: q, max_hops: 3, top_k: 5 }),
      });
      const data = await res.json();
      setQueryResponse(data);
    } catch (e) {
      console.error(e);
    } finally {
      setQueryLoading(false);
      refreshStats();
    }
  };

  // Find node IDs traversed in reasoning paths
  const highlightedNodeIds = React.useMemo(() => {
    if (!queryResponse?.reasoning_paths) return [];
    const set = new Set<string>();
    queryResponse.reasoning_paths.forEach(p => {
      p.steps.forEach(s => {
        if (s.source_node?.id) set.add(s.source_node.id);
        if (s.target_node?.id) set.add(s.target_node.id);
      });
    });
    return Array.from(set);
  }, [queryResponse]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-[#090d16] text-slate-100">
      {/* Sidebar */}
      <aside className="w-64 border-r border-slate-800/80 bg-slate-950/60 flex flex-col justify-between p-4">
        <div>
          {/* Logo / Header */}
          <div className="flex items-center gap-3 px-2 py-3 mb-6 border-b border-slate-800/60">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-cyan-500 to-blue-600 flex items-center justify-center shadow-lg shadow-cyan-500/20">
              <Network className="w-5 h-5 text-white" />
            </div>
            <div>
              <h1 className="font-bold text-base tracking-tight text-white flex items-center gap-1.5">
                MultiHop
                <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800/50">
                  RAG
                </span>
              </h1>
              <p className="text-xs text-slate-400">Hybrid Vector-Graph Engine</p>
            </div>
          </div>

          {/* Navigation */}
          <nav className="space-y-1">
            <button
              onClick={() => setActiveTab('studio')}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                activeTab === 'studio'
                  ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Sparkles className="w-4 h-4" />
              <span>Multi-Hop Studio</span>
            </button>

            <button
              onClick={() => setActiveTab('graph')}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                activeTab === 'graph'
                  ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Network className="w-4 h-4" />
              <span>Graph Explorer</span>
            </button>

            <button
              onClick={() => setActiveTab('documents')}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                activeTab === 'documents'
                  ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <FileText className="w-4 h-4" />
              <span>Corpus Documents</span>
            </button>

            <button
              onClick={() => setActiveTab('benchmark')}
              className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                activeTab === 'benchmark'
                  ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
              }`}
            >
              <Activity className="w-4 h-4" />
              <span>RAG Benchmark</span>
            </button>
          </nav>
        </div>

        {/* Runtime Diagnostics */}
        <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3.5 space-y-3">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span className="flex items-center gap-1.5 font-medium">
              <Cpu className="w-3.5 h-3.5 text-emerald-400" />
              RAM Footprint
            </span>
            <span className="font-mono text-emerald-400 text-[11px] font-semibold">
              {health.memory_alloc_mb.toFixed(1)} MB / &lt;30MB
            </span>
          </div>
          <div className="w-full bg-slate-800/80 rounded-full h-1.5 overflow-hidden">
            <div 
              className="bg-emerald-500 h-1.5 rounded-full transition-all duration-500" 
              style={{ width: `${Math.min(100, (health.memory_alloc_mb / 30) * 100)}%` }}
            />
          </div>
          <div className="grid grid-cols-2 gap-2 pt-1 border-t border-slate-800/50 text-[11px]">
            <div>
              <span className="text-slate-500 block">Entities</span>
              <span className="font-mono text-slate-200 font-medium">{stats.nodes}</span>
            </div>
            <div>
              <span className="text-slate-500 block">Relations</span>
              <span className="font-mono text-slate-200 font-medium">{stats.edges}</span>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* Top Header */}
        <header className="h-14 border-b border-slate-800/80 bg-slate-950/40 px-6 flex items-center justify-between">
          <div className="flex items-center gap-2 text-xs text-slate-400">
            <span>Knowledge Base</span>
            <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
            <span className="text-slate-200 font-medium capitalize">{activeTab}</span>
          </div>

          <div className="flex items-center gap-3">
            <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              SQLite WAL Active
            </span>
          </div>
        </header>

        {/* View Switcher */}
        <div className="flex-1 overflow-auto p-6">
          {activeTab === 'studio' && (
            <div className="max-w-5xl mx-auto space-y-6">
              {/* Query Input Card */}
              <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 shadow-xl space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-mono uppercase tracking-wider text-cyan-400 flex items-center gap-2">
                    <Sparkles className="w-4 h-4" />
                    Multi-Hop Reasoning Query
                  </span>
                  <span className="text-xs text-slate-500 font-mono">SQLite Graph + SIMD Cosine</span>
                </div>

                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <input
                      type="text"
                      value={queryInput}
                      onChange={e => setQueryInput(e.target.value)}
                      onKeyDown={e => e.key === 'Enter' && handleRunQuery()}
                      placeholder="Ask a relational reasoning question..."
                      className="w-full bg-slate-950/80 border border-slate-800 rounded-lg px-4 py-2.5 text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500/80 focus:ring-1 focus:ring-cyan-500"
                    />
                  </div>
                  <button
                    onClick={() => handleRunQuery()}
                    disabled={queryLoading}
                    className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-gradient-to-r from-cyan-500 to-blue-600 text-white font-medium text-sm hover:opacity-95 disabled:opacity-50 transition-all shadow-lg shadow-cyan-500/20"
                  >
                    {queryLoading ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <Send className="w-4 h-4" />
                    )}
                    <span>Traverse</span>
                  </button>
                </div>

                {/* Suggested Queries */}
                <div className="flex flex-wrap items-center gap-2 pt-1 text-xs text-slate-400">
                  <span className="text-slate-500 font-mono">Examples:</span>
                  {[
                    "Did any subsidiary of Vendor Alpha hold patents?",
                    "What liability caps apply to European subsidiaries?",
                    "How is European Division connected to Patent 9821?"
                  ].map((q, i) => (
                    <button
                      key={i}
                      onClick={() => {
                        setQueryInput(q);
                        handleRunQuery(q);
                      }}
                      className="px-2.5 py-1 rounded bg-slate-800/60 hover:bg-slate-800 text-slate-300 hover:text-white transition-colors"
                    >
                      {q}
                    </button>
                  ))}
                </div>
              </div>

              {/* Reasoning Results View */}
              {queryResponse && (
                <div className="space-y-6">
                  {/* Performance & Provenance Banner */}
                  <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-4 flex items-center justify-between text-xs">
                    <div className="flex items-center gap-4">
                      <span className="flex items-center gap-1.5 text-slate-300">
                        <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                        Execution Time: <strong className="text-emerald-400 font-mono">{queryResponse.execution_time_ms} ms</strong>
                      </span>
                      <span className="flex items-center gap-1.5 text-slate-300">
                        <ShieldCheck className="w-4 h-4 text-cyan-400" />
                        Hallucination Risk: <strong className="text-cyan-400 font-mono">{(queryResponse.hallucination_risk * 100).toFixed(0)}% (Low)</strong>
                      </span>
                    </div>
                    <span className="text-slate-500 font-mono">
                      {queryResponse.reasoning_paths.length} Paths Discovered
                    </span>
                  </div>

                  {/* Grounded Answer Card */}
                  <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-5 space-y-4">
                    <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                      <h3 className="font-semibold text-sm text-white flex items-center gap-2">
                        <Sparkles className="w-4 h-4 text-cyan-400" />
                        Grounded Synthesis
                      </h3>
                      <span className="text-[11px] font-mono text-slate-400">Strict Footnote Provenance</span>
                    </div>

                    <div className="text-sm text-slate-200 leading-relaxed font-sans whitespace-pre-wrap">
                      {queryResponse.answer}
                    </div>

                    {/* Citations Pill Bar */}
                    {queryResponse.citations.length > 0 && (
                      <div className="pt-3 border-t border-slate-800/60 flex flex-wrap gap-2 items-center text-xs">
                        <span className="text-slate-500 font-mono">Source Footnotes:</span>
                        {queryResponse.citations.map((c, idx) => (
                          <button
                            key={c.chunk_id}
                            onClick={() => setSelectedChunk({
                              id: c.chunk_id,
                              document_id: c.document_id,
                              document_title: c.document_title,
                              header_context: c.header_context,
                              content: c.content,
                              token_count: Math.round(c.content.length / 4),
                            })}
                            className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded bg-slate-800/80 hover:bg-cyan-950/60 hover:text-cyan-300 border border-slate-700/60 transition-colors text-slate-300"
                          >
                            <Bookmark className="w-3 h-3 text-cyan-400" />
                            <span>[^{c.chunk_id.slice(-6)}] {c.document_title}</span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>

                  {/* Multi-Hop Discovered Trajectories */}
                  <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-5 space-y-4">
                    <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                      <h3 className="font-semibold text-sm text-white flex items-center gap-2">
                        <GitBranch className="w-4 h-4 text-emerald-400" />
                        Multi-Hop Reasoning Trajectories
                      </h3>
                      <span className="text-[11px] font-mono text-slate-400">Bounded Beam Search (Max 3 Hops)</span>
                    </div>

                    <div className="space-y-3">
                      {queryResponse.reasoning_paths.slice(0, 4).map((p, idx) => (
                        <div key={idx} className="bg-slate-950/60 border border-slate-800/60 rounded-lg p-3.5 space-y-2">
                          <div className="flex items-center justify-between text-xs text-slate-400">
                            <span className="font-mono text-cyan-400 font-medium">Trajectory #{idx + 1}</span>
                            <span className="font-mono text-[11px]">Score: {p.score.toFixed(3)}</span>
                          </div>
                          <div className="flex flex-wrap items-center gap-2 text-xs">
                            {p.steps.map((s, stepIdx) => (
                              <React.Fragment key={stepIdx}>
                                <span className="px-2.5 py-1 rounded bg-cyan-950/60 text-cyan-300 border border-cyan-800/50 font-medium">
                                  {s.source_node.name}
                                </span>
                                <span className="text-[11px] font-mono text-slate-500 uppercase flex items-center gap-1">
                                  --[{s.edge.relation_type}]--&gt;
                                </span>
                                <span className="px-2.5 py-1 rounded bg-blue-950/60 text-blue-300 border border-blue-800/50 font-medium">
                                  {s.target_node.name}
                                </span>
                              </React.Fragment>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {activeTab === 'graph' && (
            <div className="h-full flex gap-4">
              <div className="flex-1 h-full min-h-[500px]">
                <GraphCanvas
                  nodes={graphData.nodes}
                  links={graphData.links}
                  highlightPathNodeIds={highlightedNodeIds}
                  onSelectNode={setSelectedNode}
                  selectedNodeId={selectedNode?.id}
                />
              </div>

              {/* Node Inspector Panel */}
              {selectedNode && (
                <div className="w-80 bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex flex-col justify-between">
                  <div className="space-y-3">
                    <div className="flex items-center justify-between border-b border-slate-800 pb-2">
                      <span className="text-xs uppercase font-mono text-cyan-400">Entity Details</span>
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-300">
                        {selectedNode.entity_type}
                      </span>
                    </div>

                    <div>
                      <h4 className="text-base font-bold text-white">{selectedNode.name}</h4>
                      <p className="text-xs text-slate-400 mt-1">{selectedNode.description || 'No description available'}</p>
                    </div>

                    <div className="grid grid-cols-2 gap-2 text-xs pt-2">
                      <div className="bg-slate-950/60 p-2.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Mentions</span>
                        <span className="font-mono text-slate-200 font-medium">{selectedNode.mention_count || 1}</span>
                      </div>
                      <div className="bg-slate-950/60 p-2.5 rounded border border-slate-800">
                        <span className="text-slate-500 block text-[10px]">Community</span>
                        <span className="font-mono text-slate-200 font-medium">#{selectedNode.community_id}</span>
                      </div>
                    </div>
                  </div>

                  <button
                    onClick={() => {
                      setQueryInput(`Tell me about ${selectedNode.name} and its relationships`);
                      setActiveTab('studio');
                    }}
                    className="w-full flex items-center justify-center gap-2 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200"
                  >
                    <Search className="w-3.5 h-3.5 text-cyan-400" />
                    <span>Query Node in Studio</span>
                  </button>
                </div>
              )}
            </div>
          )}

          {activeTab === 'documents' && (
            <div className="max-w-5xl mx-auto space-y-4">
              <h3 className="text-lg font-bold text-white tracking-tight">Ingested Corpus Documents</h3>
              <p className="text-xs text-slate-400">
                Documents parsed into heading-aware semantic chunks and packed float32 vector BLOBs in SQLite.
              </p>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 pt-2">
                {documents.map((doc, idx) => (
                  <div key={doc.id || idx} className="bg-slate-900/50 border border-slate-800 rounded-xl p-4 space-y-2">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-sm text-slate-200">{doc.title}</span>
                      <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-400 uppercase">
                        {doc.file_type}
                      </span>
                    </div>
                    <p className="text-xs text-slate-400 truncate font-mono">{doc.source_path}</p>
                    <div className="flex items-center justify-between pt-2 border-t border-slate-800/60 text-xs text-slate-500">
                      <span>{doc.token_count} estimated tokens</span>
                      <span>{doc.byte_size} bytes</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {activeTab === 'benchmark' && (
            <div className="max-w-5xl mx-auto">
              <BenchmarkView />
            </div>
          )}
        </div>
      </main>

      {/* Source Provenance Drawer */}
      <SourceDrawer chunk={selectedChunk} onClose={() => setSelectedChunk(null)} />
    </div>
  );
}
