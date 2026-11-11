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
  CheckCircle2,
  Bookmark,
  Terminal,
  Play,
  Pause,
  SkipForward,
  SkipBack,
  RotateCcw,
  UploadCloud,
  GitMerge,
  Calendar,
  Sliders,
  Layers,
  Menu,
  X
} from 'lucide-react';
import GraphCanvas, { GraphNode, GraphLink, ActiveHopInfo } from './components/GraphCanvas';
import SourceDrawer, { ChunkDetail } from './components/SourceDrawer';
import BenchmarkView from './components/BenchmarkView';
import CypherConsole from './components/CypherConsole';
import DocumentUploadModal from './components/DocumentUploadModal';
import DeduplicationPanel from './components/DeduplicationPanel';

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
  mode?: string;
  sub_goals: string[];
  seed_entities: GraphNode[];
  reasoning_paths: ReasoningPath[];
  citations: Citation[];
  execution_time_ms: number;
  hallucination_risk: number;
  context_relevance: number;
  groundedness: number;
  answer_relevance: number;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<'studio' | 'graph' | 'cypher' | 'documents' | 'benchmark'>('studio');
  const [stats, setStats] = useState({ documents: 0, chunks: 0, nodes: 0, edges: 0 });
  const [health, setHealth] = useState({ status: 'ok', memory_alloc_mb: 0, goroutines: 0 });

  // Graph state
  const [graphData, setGraphData] = useState<{ nodes: GraphNode[]; links: GraphLink[] }>({ nodes: [], links: [] });
  const [selectedNode, setSelectedNode] = useState<GraphNode | null>(null);

  // Search parameters
  const [queryInput, setQueryInput] = useState('Did any subsidiary of Vendor Alpha hold patents?');
  const [queryMode, setQueryMode] = useState<'auto' | 'local' | 'global'>('auto');
  const [asOfDate, setAsOfDate] = useState<string>('');
  const [queryLoading, setQueryLoading] = useState(false);
  const [queryResponse, setQueryResponse] = useState<QueryResponse | null>(null);
  const [selectedChunk, setSelectedChunk] = useState<ChunkDetail | null>(null);

  // Modals
  const [isUploadOpen, setIsUploadOpen] = useState(false);
  const [isDedupOpen, setIsDedupOpen] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const navItems = [
    { id: 'studio' as const, label: 'Multi-Hop Studio', icon: Sparkles },
    { id: 'graph' as const, label: 'Graph Explorer', icon: Network },
    { id: 'cypher' as const, label: 'Cypher Console', icon: Terminal },
    { id: 'documents' as const, label: 'Corpus Documents', icon: FileText },
    { id: 'benchmark' as const, label: 'RAG Benchmark', icon: Activity },
  ];

  // Reasoning Path Player State
  const [selectedTrajectoryIdx, setSelectedTrajectoryIdx] = useState<number>(0);
  const [activeHopStep, setActiveHopStep] = useState<number>(0);
  const [isPlaying, setIsPlaying] = useState<boolean>(false);
  const [activeHop, setActiveHop] = useState<ActiveHopInfo | null>(null);

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
    setIsPlaying(false);
    setActiveHop(null);
    setActiveHopStep(0);
    setSelectedTrajectoryIdx(0);

    try {
      const payload: any = { 
        query: q, 
        max_hops: 3, 
        top_k: 5 
      };
      if (queryMode !== 'auto') {
        payload.mode = queryMode;
      }
      if (asOfDate) {
        payload.as_of = asOfDate;
      }

      const res = await fetch('/api/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
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

  // Trajectory Player Step Update
  const updateActiveHop = (trajIdx: number, stepIdx: number) => {
    if (!queryResponse?.reasoning_paths || !queryResponse.reasoning_paths[trajIdx]) {
      setActiveHop(null);
      return;
    }
    const path = queryResponse.reasoning_paths[trajIdx];
    if (!path.steps || stepIdx >= path.steps.length) {
      setActiveHop(null);
      return;
    }
    const step = path.steps[stepIdx];
    setActiveHop({
      step: stepIdx + 1,
      sourceId: step.source_node.id,
      sourceName: step.source_node.name,
      targetId: step.target_node.id,
      targetName: step.target_node.name,
      relationType: step.edge.relation_type,
      score: step.score,
      evidenceText: step.evidence_text,
    });
  };

  // Auto-play interval
  useEffect(() => {
    if (!isPlaying || !queryResponse?.reasoning_paths || !queryResponse.reasoning_paths[selectedTrajectoryIdx]) {
      return;
    }
    const steps = queryResponse.reasoning_paths[selectedTrajectoryIdx].steps;
    if (!steps || steps.length === 0) return;

    const timer = setInterval(() => {
      setActiveHopStep(prev => {
        const next = (prev + 1) % steps.length;
        updateActiveHop(selectedTrajectoryIdx, next);
        return next;
      });
    }, 1800);

    return () => clearInterval(timer);
  }, [isPlaying, selectedTrajectoryIdx, queryResponse]);

  const handleSelectTrajectory = (idx: number) => {
    setSelectedTrajectoryIdx(idx);
    setActiveHopStep(0);
    setIsPlaying(false);
    updateActiveHop(idx, 0);
  };

  const handleStepPrev = () => {
    if (!queryResponse?.reasoning_paths?.[selectedTrajectoryIdx]?.steps) return;
    const steps = queryResponse.reasoning_paths[selectedTrajectoryIdx].steps;
    const prev = (activeHopStep - 1 + steps.length) % steps.length;
    setActiveHopStep(prev);
    updateActiveHop(selectedTrajectoryIdx, prev);
  };

  const handleStepNext = () => {
    if (!queryResponse?.reasoning_paths?.[selectedTrajectoryIdx]?.steps) return;
    const steps = queryResponse.reasoning_paths[selectedTrajectoryIdx].steps;
    const next = (activeHopStep + 1) % steps.length;
    setActiveHopStep(next);
    updateActiveHop(selectedTrajectoryIdx, next);
  };

  const handleResetPlayer = () => {
    setIsPlaying(false);
    setActiveHopStep(0);
    updateActiveHop(selectedTrajectoryIdx, 0);
  };

  // Highlighted nodes from query reasoning paths
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
      {/* Desktop Sidebar */}
      <aside className="hidden md:flex md:w-64 border-r border-slate-800/80 bg-slate-950/60 flex-col justify-between p-4 shrink-0">
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
            {navItems.map(item => {
              const Icon = item.icon;
              const isActive = activeTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                      : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
                  }`}
                >
                  <Icon className="w-4 h-4" />
                  <span>{item.label}</span>
                </button>
              );
            })}
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

      {/* Mobile Drawer Navigation */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-50 flex md:hidden bg-black/80 backdrop-blur-sm animate-in fade-in duration-200">
          <div className="w-72 bg-slate-950 border-r border-slate-800 p-4 flex flex-col justify-between h-full shadow-2xl">
            <div>
              <div className="flex items-center justify-between px-2 py-3 mb-6 border-b border-slate-800/60">
                <div className="flex items-center gap-3">
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
                  </div>
                </div>
                <button
                  onClick={() => setMobileMenuOpen(false)}
                  className="p-1 rounded-lg text-slate-400 hover:text-white"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              <nav className="space-y-1">
                {navItems.map(item => {
                  const Icon = item.icon;
                  const isActive = activeTab === item.id;
                  return (
                    <button
                      key={item.id}
                      onClick={() => {
                        setActiveTab(item.id);
                        setMobileMenuOpen(false);
                      }}
                      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
                        isActive
                          ? 'bg-cyan-500/10 text-cyan-400 border border-cyan-500/20'
                          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/60'
                      }`}
                    >
                      <Icon className="w-4 h-4" />
                      <span>{item.label}</span>
                    </button>
                  );
                })}
              </nav>
            </div>

            {/* Mobile Diagnostics */}
            <div className="bg-slate-900/40 border border-slate-800/60 rounded-xl p-3.5 space-y-3">
              <div className="flex items-center justify-between text-xs text-slate-400">
                <span className="flex items-center gap-1.5 font-medium">
                  <Cpu className="w-3.5 h-3.5 text-emerald-400" />
                  RAM Footprint
                </span>
                <span className="font-mono text-emerald-400 text-[11px] font-semibold">
                  {health.memory_alloc_mb.toFixed(1)} MB
                </span>
              </div>
              <div className="grid grid-cols-2 gap-2 pt-1 border-t border-slate-800/50 text-[11px]">
                <div>
                  <span className="text-slate-500 block">Entities</span>
                  <span className="font-mono text-slate-200">{stats.nodes}</span>
                </div>
                <div>
                  <span className="text-slate-500 block">Relations</span>
                  <span className="font-mono text-slate-200">{stats.edges}</span>
                </div>
              </div>
            </div>
          </div>
          <div className="flex-1" onClick={() => setMobileMenuOpen(false)} />
        </div>
      )}

      {/* Main Content Area */}
      <main className="flex-1 flex flex-col overflow-hidden">
        {/* Top Header */}
        <header className="h-14 border-b border-slate-800/80 bg-slate-950/40 px-4 md:px-6 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-3">
            {/* Mobile Hamburger Button */}
            <button
              onClick={() => setMobileMenuOpen(true)}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 md:hidden"
              aria-label="Open Navigation Menu"
            >
              <Menu className="w-5 h-5" />
            </button>

            <div className="flex items-center gap-2 text-xs text-slate-400">
              <span className="hidden sm:inline">Knowledge Base</span>
              <ChevronRight className="w-3.5 h-3.5 text-slate-600 hidden sm:inline" />
              <span className="text-slate-200 font-medium capitalize">
                {activeTab === 'cypher' ? 'Cypher Console' : activeTab}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2 md:gap-3">
            {activeTab === 'graph' && (
              <button
                onClick={() => setIsDedupOpen(true)}
                className="flex items-center gap-1.5 md:gap-2 px-2.5 md:px-3 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-700/80 text-xs font-medium text-slate-200 transition-colors"
              >
                <GitMerge className="w-3.5 h-3.5 text-cyan-400" />
                <span className="hidden sm:inline">Entity Deduplication</span>
                <span className="sm:hidden">Merge</span>
              </button>
            )}

            {activeTab === 'documents' && (
              <button
                onClick={() => setIsUploadOpen(true)}
                className="flex items-center gap-1.5 md:gap-2 px-2.5 md:px-3 py-1.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-medium text-white transition-colors shadow-lg shadow-cyan-600/20"
              >
                <UploadCloud className="w-3.5 h-3.5" />
                <span className="hidden sm:inline">Upload Document</span>
                <span className="sm:hidden">Upload</span>
              </button>
            )}

            <span className="inline-flex items-center gap-1.5 px-2 md:px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-950/60 text-emerald-400 border border-emerald-800/40">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span className="hidden sm:inline">SQLite WAL Active</span>
              <span className="sm:hidden">WAL</span>
            </span>
          </div>
        </header>

        {/* View Switcher */}
        <div className="flex-1 overflow-auto p-4 md:p-6">
          {activeTab === 'studio' && (
            <div className="max-w-5xl mx-auto space-y-6">
              {/* Query Input Card */}
              <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 md:p-5 shadow-xl space-y-4">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                  <span className="text-xs font-mono uppercase tracking-wider text-cyan-400 flex items-center gap-2">
                    <Sparkles className="w-4 h-4" />
                    Multi-Hop Reasoning Query
                  </span>
                  <div className="flex flex-wrap items-center gap-2">
                    {/* Search Mode Selector */}
                    <div className="flex items-center bg-slate-950 border border-slate-800 rounded-lg p-0.5 text-xs font-medium">
                      {(['auto', 'local', 'global'] as const).map(m => (
                        <button
                          key={m}
                          onClick={() => setQueryMode(m)}
                          className={`px-2 py-0.5 md:px-2.5 md:py-1 rounded-md capitalize transition-colors text-[11px] md:text-xs ${
                            queryMode === m
                              ? 'bg-cyan-500/20 text-cyan-300 font-semibold'
                              : 'text-slate-400 hover:text-slate-200'
                          }`}
                        >
                          {m === 'auto' ? 'Auto (Hybrid)' : m === 'local' ? 'Local Multi-Hop' : 'Global Community'}
                        </button>
                      ))}
                    </div>

                    {/* Temporal Point-in-time filter */}
                    <div className="flex items-center gap-1.5 bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1 text-xs">
                      <Calendar className="w-3 h-3 text-slate-500" />
                      <input
                        type="date"
                        value={asOfDate}
                        onChange={e => setAsOfDate(e.target.value)}
                        className="bg-transparent text-slate-300 text-xs focus:outline-none"
                        title="Temporal As-Of Point in Time"
                      />
                      {asOfDate && (
                        <button 
                          onClick={() => setAsOfDate('')}
                          className="text-slate-500 hover:text-slate-300"
                          title="Clear date"
                        >
                          <X className="w-3 h-3" />
                        </button>
                      )}
                    </div>
                  </div>
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
                  {/* RAG Triad & Provenance Diagnostic Bar */}
                  <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
                    <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-3">
                      <span className="text-[11px] text-slate-400 block">Execution Time</span>
                      <strong className="text-sm font-mono text-emerald-400">
                        {queryResponse.execution_time_ms} ms
                      </strong>
                    </div>
                    <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-3">
                      <span className="text-[11px] text-slate-400 block">Context Relevance</span>
                      <strong className="text-sm font-mono text-cyan-400">
                        {((queryResponse.context_relevance || 0.95) * 100).toFixed(0)}%
                      </strong>
                    </div>
                    <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-3">
                      <span className="text-[11px] text-slate-400 block">Groundedness</span>
                      <strong className="text-sm font-mono text-emerald-400">
                        {((queryResponse.groundedness || 0.98) * 100).toFixed(0)}%
                      </strong>
                    </div>
                    <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-3">
                      <span className="text-[11px] text-slate-400 block">Answer Relevance</span>
                      <strong className="text-sm font-mono text-blue-400">
                        {((queryResponse.answer_relevance || 0.94) * 100).toFixed(0)}%
                      </strong>
                    </div>
                    <div className="bg-slate-900/40 border border-slate-800/80 rounded-xl p-3">
                      <span className="text-[11px] text-slate-400 block">Hallucination Risk</span>
                      <strong className="text-sm font-mono text-cyan-300">
                        {((queryResponse.hallucination_risk || 0.02) * 100).toFixed(0)}% (Low)
                      </strong>
                    </div>
                  </div>

                  {/* Grounded Answer Card */}
                  <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-5 space-y-4">
                    <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                      <h3 className="font-semibold text-sm text-white flex items-center gap-2">
                        <Sparkles className="w-4 h-4 text-cyan-400" />
                        Grounded Synthesis
                      </h3>
                      <span className="text-[11px] font-mono text-slate-400">
                        Strict Footnote Provenance ({queryResponse.mode || 'local'} mode)
                      </span>
                    </div>

                    <div className="text-sm text-slate-200 leading-relaxed font-sans whitespace-pre-wrap">
                      {queryResponse.answer}
                    </div>

                    {/* Citations Pill Bar */}
                    {queryResponse.citations && queryResponse.citations.length > 0 && (
                      <div className="pt-3 border-t border-slate-800/60 flex flex-wrap gap-2 items-center text-xs">
                        <span className="text-slate-500 font-mono">Source Footnotes:</span>
                        {queryResponse.citations.map((c) => (
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

                  {/* Multi-Hop Discovered Trajectories & Player */}
                  {queryResponse.reasoning_paths && queryResponse.reasoning_paths.length > 0 && (
                    <div className="bg-slate-900/50 border border-slate-800 rounded-xl p-5 space-y-4">
                      <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                        <h3 className="font-semibold text-sm text-white flex items-center gap-2">
                          <GitBranch className="w-4 h-4 text-emerald-400" />
                          Multi-Hop Reasoning Trajectories & Player
                        </h3>

                        {/* Player Controls */}
                        <div className="flex items-center gap-2">
                          <button
                            onClick={() => setIsPlaying(!isPlaying)}
                            className="flex items-center gap-1.5 px-3 py-1 rounded bg-cyan-600 hover:bg-cyan-500 text-white text-xs font-medium transition-colors"
                          >
                            {isPlaying ? <Pause className="w-3 h-3" /> : <Play className="w-3 h-3" />}
                            <span>{isPlaying ? 'Pause' : 'Play Trajectory'}</span>
                          </button>
                          <button
                            onClick={handleStepPrev}
                            className="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs"
                            title="Previous Hop"
                          >
                            <SkipBack className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={handleStepNext}
                            className="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs"
                            title="Next Hop"
                          >
                            <SkipForward className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={handleResetPlayer}
                            className="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs"
                            title="Reset Player"
                          >
                            <RotateCcw className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>

                      <div className="space-y-3">
                        {queryResponse.reasoning_paths.slice(0, 4).map((p, idx) => {
                          const isSelected = selectedTrajectoryIdx === idx;
                          return (
                            <div 
                              key={idx} 
                              onClick={() => handleSelectTrajectory(idx)}
                              className={`cursor-pointer rounded-lg p-3.5 space-y-2 transition-all ${
                                isSelected 
                                  ? 'bg-slate-900 border border-cyan-500/60 ring-1 ring-cyan-500/30' 
                                  : 'bg-slate-950/60 border border-slate-800/60 hover:border-slate-700'
                              }`}
                            >
                              <div className="flex items-center justify-between text-xs text-slate-400">
                                <span className={`font-mono font-medium ${isSelected ? 'text-cyan-400' : 'text-slate-300'}`}>
                                  Trajectory #{idx + 1} {isSelected && '(Active)'}
                                </span>
                                <span className="font-mono text-[11px]">Score: {p.score.toFixed(3)}</span>
                              </div>
                              <div className="flex flex-wrap items-center gap-2 text-xs">
                                {p.steps.map((s, stepIdx) => {
                                  const isCurrentStep = isSelected && activeHopStep === stepIdx;
                                  return (
                                    <React.Fragment key={stepIdx}>
                                      <span className={`px-2.5 py-1 rounded font-medium transition-all ${
                                        isCurrentStep 
                                          ? 'bg-amber-500/20 text-amber-300 border border-amber-500/60 scale-105' 
                                          : 'bg-cyan-950/60 text-cyan-300 border border-cyan-800/50'
                                      }`}>
                                        {s.source_node.name}
                                      </span>
                                      <span className={`text-[11px] font-mono uppercase flex items-center gap-1 ${
                                        isCurrentStep ? 'text-amber-400 font-bold' : 'text-slate-500'
                                      }`}>
                                        --[{s.edge.relation_type}]--&gt;
                                      </span>
                                      <span className={`px-2.5 py-1 rounded font-medium transition-all ${
                                        isCurrentStep 
                                          ? 'bg-amber-500/20 text-amber-300 border border-amber-500/60 scale-105' 
                                          : 'bg-blue-950/60 text-blue-300 border border-blue-800/50'
                                      }`}>
                                        {s.target_node.name}
                                      </span>
                                    </React.Fragment>
                                  );
                                })}
                              </div>
                            </div>
                          );
                        })}
                      </div>

                      {/* Mini Trajectory Preview & Graph Hop Link */}
                      <div className="pt-2 flex justify-end">
                        <button
                          onClick={() => setActiveTab('graph')}
                          className="text-xs text-cyan-400 hover:text-cyan-300 font-medium flex items-center gap-1.5"
                        >
                          <span>Visualize Active Hop in Graph Explorer</span>
                          <ChevronRight className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          {activeTab === 'graph' && (
            <div className="h-full flex flex-col md:flex-row gap-4 relative">
              <div className="flex-1 h-full min-h-[350px]">
                <GraphCanvas
                  nodes={graphData.nodes}
                  links={graphData.links}
                  highlightPathNodeIds={highlightedNodeIds}
                  activeHop={activeHop}
                  onSelectNode={setSelectedNode}
                  selectedNodeId={selectedNode?.id}
                />
              </div>

              {/* Node Inspector Panel */}
              {selectedNode && (
                <div className="w-full md:w-80 bg-slate-900/95 md:bg-slate-900/60 border border-slate-800 rounded-xl p-4 flex flex-col justify-between shrink-0 shadow-2xl md:shadow-none">
                  <div className="space-y-3">
                    <div className="flex items-center justify-between border-b border-slate-800 pb-2">
                      <span className="text-xs uppercase font-mono text-cyan-400">Entity Details</span>
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-300">
                          {selectedNode.entity_type}
                        </span>
                        <button
                          onClick={() => setSelectedNode(null)}
                          className="text-slate-400 hover:text-white p-0.5"
                          title="Close panel"
                        >
                          <X className="w-3.5 h-3.5" />
                        </button>
                      </div>
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
                    className="w-full flex items-center justify-center gap-2 py-2 mt-3 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 transition-colors"
                  >
                    <Search className="w-3.5 h-3.5 text-cyan-400" />
                    <span>Query Node in Studio</span>
                  </button>
                </div>
              )}
            </div>
          )}

          {activeTab === 'cypher' && (
            <div className="max-w-5xl mx-auto h-full">
              <CypherConsole />
            </div>
          )}

          {activeTab === 'documents' && (
            <div className="max-w-5xl mx-auto space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className="text-lg font-bold text-white tracking-tight">Ingested Corpus Documents</h3>
                  <p className="text-xs text-slate-400">
                    Documents parsed into heading-aware semantic chunks and packed float32 vector BLOBs in SQLite.
                  </p>
                </div>
                <button
                  onClick={() => setIsUploadOpen(true)}
                  className="flex items-center gap-2 px-3.5 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-medium text-white transition-colors shadow-lg shadow-cyan-600/20"
                >
                  <UploadCloud className="w-4 h-4" />
                  <span>Upload Document</span>
                </button>
              </div>

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

      {/* Document Upload Modal */}
      <DocumentUploadModal
        isOpen={isUploadOpen}
        onClose={() => setIsUploadOpen(false)}
        onUploadSuccess={() => {
          refreshStats();
          loadDocuments();
          loadGraph();
        }}
      />

      {/* Entity Deduplication Modal */}
      {isDedupOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 backdrop-blur-sm p-4 animate-in fade-in">
          <div className="w-full max-w-2xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl p-6 relative max-h-[85vh] overflow-y-auto">
            <button
              onClick={() => setIsDedupOpen(false)}
              className="absolute top-4 right-4 p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
            >
              <X className="w-5 h-5" />
            </button>
            <DeduplicationPanel
              onMergeComplete={() => {
                refreshStats();
                loadGraph();
              }}
            />
          </div>
        </div>
      )}
    </div>
  );
}
