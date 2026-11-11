import React, { useState } from 'react';
import { Terminal, Play, Sparkles, Database, Check, AlertCircle } from 'lucide-react';

const PRESET_QUERIES = [
  {
    name: "Corporate Ownership Chains",
    query: "MATCH (a:ORGANIZATION)-[r:OWNS]->(b:ORGANIZATION) RETURN a, r, b",
  },
  {
    name: "Patent Holdings",
    query: "MATCH (a)-[r:HOLDS]->(b) RETURN a, b",
  },
  {
    name: "Liability Caps",
    query: "MATCH (a)-[r:LIMITS_LIABILITY]->(b) RETURN a, b",
  },
  {
    name: "All Directed Relationships",
    query: "MATCH (a)-[r]->(b) RETURN a, r, b",
  },
];

export default function CypherConsole() {
  const [query, setQuery] = useState(PRESET_QUERIES[0].query);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<any[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const executeQuery = async (queryText?: string) => {
    const q = queryText || query;
    if (!q.trim() || running) return;

    setRunning(true);
    setError(null);

    try {
      const res = await fetch('/api/cypher', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: q }),
      });
      const data = await res.json();
      if (res.ok && data.results) {
        setResults(data.results);
      } else {
        setError(data.error || 'Failed to execute Cypher query');
        setResults(null);
      }
    } catch (e: any) {
      setError(e.message || 'Network error');
      setResults(null);
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-white tracking-tight flex items-center gap-2">
          <Terminal className="w-5 h-5 text-cyan-400" />
          Cypher-Lite Interactive Terminal
        </h2>
        <p className="text-sm text-slate-400 mt-1">
          Execute micro graph pattern matching queries against SQLite using recursive Common Table Expressions (CTEs).
        </p>
      </div>

      {/* Query Console Card */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 shadow-xl space-y-4">
        {/* Preset Queries */}
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <span className="text-slate-500 font-mono">Presets:</span>
          {PRESET_QUERIES.map((p, idx) => (
            <button
              key={idx}
              onClick={() => {
                setQuery(p.query);
                executeQuery(p.query);
              }}
              className="px-2.5 py-1 rounded bg-slate-800/80 hover:bg-slate-700 text-slate-300 hover:text-white transition-colors"
            >
              {p.name}
            </button>
          ))}
        </div>

        {/* Input Box */}
        <div className="space-y-2">
          <textarea
            value={query}
            onChange={e => setQuery(e.target.value)}
            rows={3}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs font-mono text-cyan-300 placeholder-slate-600 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500"
            placeholder="MATCH (a:TYPE)-[r:REL]->(b) WHERE a.name = 'X' RETURN b"
          />
        </div>

        <div className="flex items-center justify-between pt-1">
          <span className="text-xs text-slate-500 font-mono">Format: MATCH (a:TYPE)-[r:REL*1..3]-&gt;(b) [WHERE a.name = '...']</span>
          <button
            onClick={() => executeQuery()}
            disabled={running}
            className="flex items-center gap-2 px-4 py-2 rounded-lg bg-gradient-to-r from-cyan-500 to-blue-600 text-white font-medium text-xs hover:opacity-95 disabled:opacity-50 transition-all shadow-lg shadow-cyan-500/20"
          >
            <Play className="w-3.5 h-3.5 fill-current" />
            <span>{running ? 'Executing...' : 'Run Query'}</span>
          </button>
        </div>
      </div>

      {/* Error Banner */}
      {error && (
        <div className="bg-rose-950/40 border border-rose-800/60 rounded-xl p-4 flex items-center gap-3 text-xs text-rose-300">
          <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Results View */}
      {results && (
        <div className="bg-slate-900/50 border border-slate-800 rounded-xl overflow-hidden space-y-3">
          <div className="p-4 border-b border-slate-800 flex items-center justify-between">
            <span className="text-xs font-semibold text-white flex items-center gap-2">
              <Database className="w-4 h-4 text-cyan-400" />
              Pattern Matches ({results.length} records found)
            </span>
          </div>

          <div className="p-4 overflow-x-auto">
            {results.length === 0 ? (
              <p className="text-xs text-slate-500 italic">No matching subgraphs found for this pattern.</p>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                {results.map((r, i) => (
                  <div key={i} className="bg-slate-950/80 p-3.5 rounded-lg border border-slate-800/80 text-xs font-mono space-y-1.5">
                    <span className="text-slate-500 text-[10px] block">MATCH #{i + 1}</span>
                    <pre className="text-slate-300 overflow-x-auto whitespace-pre-wrap">
                      {JSON.stringify(r.values, null, 2)}
                    </pre>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
