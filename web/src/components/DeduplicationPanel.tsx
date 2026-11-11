import React, { useState, useEffect } from 'react';
import { GitMerge, Check, AlertCircle, RefreshCw, Loader2 } from 'lucide-react';

interface Candidate {
  node_a: { id: string; name: string; entity_type: string; mention_count: number };
  node_b: { id: string; name: string; entity_type: string; mention_count: number };
  similarity: number;
  reason: string;
}

interface DeduplicationPanelProps {
  onMergeComplete: () => void;
}

export default function DeduplicationPanel({ onMergeComplete }: DeduplicationPanelProps) {
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [loading, setLoading] = useState(false);
  const [mergingId, setMergingId] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  const fetchCandidates = async () => {
    setLoading(true);
    setMessage(null);
    try {
      const res = await fetch('/api/nodes/duplicates');
      const data = await res.json();
      setCandidates(data || []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchCandidates();
  }, []);

  const handleMerge = async (canonicalId: string, duplicateId: string) => {
    setMergingId(duplicateId);
    try {
      const res = await fetch('/api/nodes/merge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          canonical_id: canonicalId,
          duplicate_ids: [duplicateId],
        }),
      });
      if (res.ok) {
        setMessage(`Successfully merged into ${canonicalId}`);
        fetchCandidates();
        onMergeComplete();
      }
    } catch (e) {
      console.error(e);
    } finally {
      setMergingId(null);
    }
  };

  return (
    <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
      <div className="flex items-center justify-between border-b border-slate-800 pb-3">
        <div>
          <h3 className="font-semibold text-sm text-white flex items-center gap-2">
            <GitMerge className="w-4 h-4 text-cyan-400" />
            Entity Disambiguation & Deduplication
          </h3>
          <p className="text-xs text-slate-400 mt-0.5">
            Identify and merge lexical variants into canonical graph nodes.
          </p>
        </div>
        <button
          onClick={fetchCandidates}
          disabled={loading}
          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {message && (
        <div className="bg-emerald-950/40 border border-emerald-800/50 rounded-lg p-2.5 text-xs text-emerald-300 flex items-center gap-2">
          <Check className="w-3.5 h-3.5 text-emerald-400" />
          <span>{message}</span>
        </div>
      )}

      {loading ? (
        <div className="py-6 flex items-center justify-center text-xs text-slate-500 gap-2">
          <Loader2 className="w-4 h-4 animate-spin" />
          <span>Scanning knowledge graph for duplicate entities...</span>
        </div>
      ) : candidates.length === 0 ? (
        <p className="text-xs text-slate-500 italic py-2">
          No duplicate entity candidates detected. The knowledge graph is clean and normalized.
        </p>
      ) : (
        <div className="space-y-3">
          {candidates.map((c, i) => (
            <div key={i} className="bg-slate-950/60 border border-slate-800/80 rounded-lg p-3 flex items-center justify-between gap-3 text-xs">
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium text-slate-200">"{c.node_a.name}"</span>
                  <span className="text-slate-500 font-mono">⟷</span>
                  <span className="font-medium text-slate-300">"{c.node_b.name}"</span>
                </div>
                <div className="flex items-center gap-3 text-[11px] text-slate-500 font-mono">
                  <span>Match: {(c.similarity * 100).toFixed(0)}%</span>
                  <span>{c.reason}</span>
                </div>
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => handleMerge(c.node_a.id, c.node_b.id)}
                  disabled={mergingId === c.node_b.id}
                  className="px-2.5 py-1 rounded bg-cyan-950/60 hover:bg-cyan-900 border border-cyan-800/60 text-cyan-300 text-[11px] font-medium transition-colors"
                >
                  Merge into "{c.node_a.name}"
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
