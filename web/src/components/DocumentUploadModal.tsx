import React, { useState, useRef } from 'react';
import { UploadCloud, CheckCircle2, AlertCircle, X, FileText, Loader2 } from 'lucide-react';

interface DocumentUploadModalProps {
  isOpen: boolean;
  onClose: () => void;
  onUploadSuccess: () => void;
}

export default function DocumentUploadModal({ isOpen, onClose, onUploadSuccess }: DocumentUploadModalProps) {
  const [dragActive, setDragActive] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [stage, setStage] = useState<string>('');
  const [result, setResult] = useState<any | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  if (!isOpen) return null;

  const handleDrag = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      setFile(e.dataTransfer.files[0]);
    }
  };

  const handleUpload = async () => {
    if (!file) return;

    setUploading(true);
    setError(null);
    setResult(null);

    // Simulated progress stage updates while request processes
    setStage('Parsing document structure...');
    const t1 = setTimeout(() => setStage('Segmenting semantic chunks & computing vectors...'), 400);
    const t2 = setTimeout(() => setStage('Extracting named entities & directed relations...'), 900);

    try {
      const formData = new FormData();
      formData.append('file', file);

      const res = await fetch('/api/documents/upload', {
        method: 'POST',
        body: formData,
      });

      const data = await res.json();
      if (res.ok) {
        setResult(data);
        onUploadSuccess();
      } else {
        setError(data.error || 'Failed to upload document');
      }
    } catch (e: any) {
      setError(e.message || 'Upload error');
    } finally {
      clearTimeout(t1);
      clearTimeout(t2);
      setUploading(false);
      setStage('');
    }
  };

  return (
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-xl w-full max-w-lg shadow-2xl overflow-hidden space-y-4">
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <UploadCloud className="w-5 h-5 text-cyan-400" />
            <h3 className="font-semibold text-sm text-white">Live Ingestion & Graph Extraction</h3>
          </div>
          <button onClick={onClose} className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800">
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Drop Zone */}
        <div className="p-4 space-y-4">
          <div
            onDragEnter={handleDrag}
            onDragOver={handleDrag}
            onDragLeave={handleDrag}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
            className={`border-2 border-dashed rounded-xl p-8 flex flex-col items-center justify-center gap-3 cursor-pointer transition-colors ${
              dragActive
                ? 'border-cyan-500 bg-cyan-950/20'
                : 'border-slate-800 hover:border-slate-700 bg-slate-950/40'
            }`}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".md,.markdown,.txt,.json,.csv"
              onChange={e => e.target.files && setFile(e.target.files[0])}
              className="hidden"
            />
            <div className="w-12 h-12 rounded-full bg-slate-800/80 flex items-center justify-center text-cyan-400">
              <FileText className="w-6 h-6" />
            </div>
            <div className="text-center space-y-1">
              <p className="text-xs font-medium text-slate-200">
                {file ? file.name : 'Click to select or drag and drop document'}
              </p>
              <p className="text-[11px] text-slate-500">Supports Markdown, Text, and JSON (up to 32MB)</p>
            </div>
          </div>

          {/* Active Upload Stages */}
          {uploading && (
            <div className="bg-slate-950/80 border border-slate-800 rounded-lg p-3 flex items-center gap-3 text-xs text-cyan-300">
              <Loader2 className="w-4 h-4 animate-spin text-cyan-400 shrink-0" />
              <span>{stage}</span>
            </div>
          )}

          {/* Error Banner */}
          {error && (
            <div className="bg-rose-950/40 border border-rose-800/60 rounded-lg p-3 flex items-center gap-2 text-xs text-rose-300">
              <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {/* Success Banner */}
          {result && (
            <div className="bg-emerald-950/30 border border-emerald-800/50 rounded-lg p-3 space-y-2 text-xs text-emerald-300">
              <div className="flex items-center gap-2 font-medium">
                <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                <span>Document successfully indexed!</span>
              </div>
              <div className="grid grid-cols-3 gap-2 pt-1 border-t border-emerald-800/30 font-mono text-[11px]">
                <div>Chunks: <strong>{result.chunks_count}</strong></div>
                <div>Entities: <strong>{result.entities_count}</strong></div>
                <div>Relations: <strong>{result.relations_count}</strong></div>
              </div>
            </div>
          )}

          {/* Footer Action */}
          <div className="flex justify-end gap-2 pt-2">
            <button
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-medium"
            >
              Close
            </button>
            <button
              onClick={handleUpload}
              disabled={!file || uploading}
              className="px-4 py-2 rounded-lg bg-gradient-to-r from-cyan-500 to-blue-600 text-white text-xs font-medium hover:opacity-95 disabled:opacity-50 transition-all shadow-lg shadow-cyan-500/20"
            >
              {uploading ? 'Processing...' : 'Ingest & Index'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
