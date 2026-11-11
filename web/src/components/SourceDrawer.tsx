import React from 'react';
import { X, FileText, Hash, Calendar, Copy, Check } from 'lucide-react';

export interface ChunkDetail {
  id: string;
  document_id: string;
  document_title?: string;
  header_context?: string;
  content: string;
  token_count: number;
}

interface SourceDrawerProps {
  chunk: ChunkDetail | null;
  onClose: () => void;
}

export default function SourceDrawer({ chunk, onClose }: SourceDrawerProps) {
  const [copied, setCopied] = React.useState(false);

  if (!chunk) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(chunk.content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="fixed inset-y-0 right-0 w-96 bg-slate-900/95 backdrop-blur-md border-l border-slate-800 shadow-2xl z-50 flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <FileText className="w-4 h-4 text-cyan-400" />
          <h3 className="font-semibold text-sm text-slate-100">Document Chunk Provenance</h3>
        </div>
        <button
          onClick={onClose}
          className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
        >
          <X className="w-4 h-4" />
        </button>
      </div>

      {/* Metadata Bar */}
      <div className="p-4 bg-slate-950/40 border-b border-slate-800/60 space-y-2 text-xs">
        <div>
          <span className="text-slate-500 block text-[10px] uppercase font-mono">Document Title</span>
          <span className="font-medium text-slate-200">{chunk.document_title || chunk.document_id}</span>
        </div>
        {chunk.header_context && (
          <div>
            <span className="text-slate-500 block text-[10px] uppercase font-mono">Header Context</span>
            <span className="text-cyan-400 font-mono text-[11px]">{chunk.header_context}</span>
          </div>
        )}
        <div className="flex items-center justify-between pt-1">
          <span className="font-mono text-slate-400 flex items-center gap-1">
            <Hash className="w-3.5 h-3.5 text-slate-500" />
            {chunk.token_count} tokens
          </span>
          <span className="font-mono text-slate-400 text-[11px]">ID: {chunk.id}</span>
        </div>
      </div>

      {/* Chunk Content Body */}
      <div className="flex-1 overflow-auto p-4 space-y-3">
        <div className="flex items-center justify-between">
          <span className="text-xs uppercase font-mono text-slate-400">Raw Text Evidence</span>
          <button
            onClick={handleCopy}
            className="flex items-center gap-1 text-xs text-slate-400 hover:text-white px-2 py-1 rounded bg-slate-800/60 hover:bg-slate-800"
          >
            {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            <span>{copied ? 'Copied' : 'Copy'}</span>
          </button>
        </div>
        <div className="bg-slate-950/80 p-3.5 rounded-lg border border-slate-800/80 text-xs text-slate-200 leading-relaxed font-sans whitespace-pre-wrap">
          {chunk.content}
        </div>
      </div>
    </div>
  );
}
