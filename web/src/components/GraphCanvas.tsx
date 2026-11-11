import React, { useRef, useEffect, useState, useCallback } from 'react';

export interface GraphNode {
  id: string;
  name: string;
  entity_type: string;
  description?: string;
  mention_count: number;
  community_id: number;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
}

export interface GraphLink {
  id: string;
  source: string;
  target: string;
  relation_type: string;
  weight: number;
}

interface GraphCanvasProps {
  nodes: GraphNode[];
  links: GraphLink[];
  highlightPathNodeIds?: string[];
  highlightEdgeIds?: string[];
  onSelectNode?: (node: GraphNode | null) => void;
  selectedNodeId?: string | null;
}

const TYPE_COLORS: Record<string, string> = {
  ORGANIZATION: '#06b6d4', // Cyan
  PATENT: '#f59e0b',       // Amber
  CLAUSE: '#10b981',       // Emerald
  MONEY: '#f43f5e',        // Rose
  PERSON: '#a855f7',       // Purple
  LOCATION: '#3b82f6',     // Blue
  DEFAULT: '#94a3b8',      // Slate
};

export default function GraphCanvas({
  nodes: initialNodes,
  links: initialLinks,
  highlightPathNodeIds = [],
  highlightEdgeIds = [],
  onSelectNode,
  selectedNodeId,
}: GraphCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [simNodes, setSimNodes] = useState<GraphNode[]>([]);
  const [simLinks, setSimLinks] = useState<GraphLink[]>([]);

  // Camera transform
  const transformRef = useRef({ x: 0, y: 0, k: 1 });
  const isDraggingRef = useRef(false);
  const dragStartRef = useRef({ x: 0, y: 0 });
  const draggedNodeRef = useRef<GraphNode | null>(null);

  // Initialize simulation positions
  useEffect(() => {
    if (!initialNodes.length) {
      setSimNodes([]);
      setSimLinks([]);
      return;
    }

    const width = containerRef.current?.clientWidth || 800;
    const height = containerRef.current?.clientHeight || 600;

    const positioned = initialNodes.map((n, i) => {
      const angle = (i / initialNodes.length) * 2 * Math.PI;
      const radius = Math.min(width, height) * 0.35 * (0.8 + Math.random() * 0.4);
      return {
        ...n,
        x: width / 2 + Math.cos(angle) * radius,
        y: height / 2 + Math.sin(angle) * radius,
        vx: 0,
        vy: 0,
      };
    });

    setSimNodes(positioned);
    setSimLinks(initialLinks);
  }, [initialNodes, initialLinks]);

  // Force-directed layout physics step
  useEffect(() => {
    if (!simNodes.length) return;

    let animId: number;
    let iteration = 0;
    const maxIterations = 250;

    const nodeMap = new Map<string, GraphNode>();
    simNodes.forEach(n => nodeMap.set(n.id, n));

    const stepSimulation = () => {
      if (iteration >= maxIterations && !draggedNodeRef.current) {
        return;
      }
      iteration++;

      const kRepulse = 3500;
      const kAttract = 0.04;
      const damping = 0.88;

      // 1. Repulsion between all pairs
      for (let i = 0; i < simNodes.length; i++) {
        for (let j = i + 1; j < simNodes.length; j++) {
          const n1 = simNodes[i];
          const n2 = simNodes[j];
          const dx = (n1.x || 0) - (n2.x || 0);
          const dy = (n1.y || 0) - (n2.y || 0);
          const distSq = dx * dx + dy * dy + 100;
          const dist = Math.sqrt(distSq);

          if (dist > 0 && dist < 450) {
            const force = kRepulse / distSq;
            const fx = (dx / dist) * force;
            const fy = (dy / dist) * force;

            n1.vx = (n1.vx || 0) + fx;
            n1.vy = (n1.vy || 0) + fy;
            n2.vx = (n2.vx || 0) - fx;
            n2.vy = (n2.vy || 0) - fy;
          }
        }
      }

      // 2. Attraction along links
      for (const link of simLinks) {
        const src = nodeMap.get(link.source);
        const tgt = nodeMap.get(link.target);
        if (!src || !tgt) continue;

        const dx = (tgt.x || 0) - (src.x || 0);
        const dy = (tgt.y || 0) - (src.y || 0);
        const dist = Math.sqrt(dx * dx + dy * dy) || 1;
        const targetDist = 120;
        const delta = dist - targetDist;

        const force = delta * kAttract;
        const fx = (dx / dist) * force;
        const fy = (dy / dist) * force;

        src.vx = (src.vx || 0) + fx;
        src.vy = (src.vy || 0) + fy;
        tgt.vx = (tgt.vx || 0) - fx;
        tgt.vy = (tgt.vy || 0) - fy;
      }

      // 3. Center gravity
      const width = containerRef.current?.clientWidth || 800;
      const height = containerRef.current?.clientHeight || 600;
      const cx = width / 2;
      const cy = height / 2;

      for (const n of simNodes) {
        if (n === draggedNodeRef.current) continue;
        const dx = cx - (n.x || cx);
        const dy = cy - (n.y || cy);
        n.vx = ((n.vx || 0) + dx * 0.005) * damping;
        n.vy = ((n.vy || 0) + dy * 0.005) * damping;

        n.x = (n.x || cx) + (n.vx || 0);
        n.y = (n.y || cy) + (n.vy || 0);
      }

      draw();
      animId = requestAnimationFrame(stepSimulation);
    };

    animId = requestAnimationFrame(stepSimulation);
    return () => cancelAnimationFrame(animId);
  }, [simNodes, simLinks]);

  // Render canvas
  const draw = useCallback(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const width = canvas.width;
    const height = canvas.height;
    ctx.clearRect(0, 0, width, height);

    ctx.save();
    // Camera Transform
    ctx.translate(transformRef.current.x, transformRef.current.y);
    ctx.scale(transformRef.current.k, transformRef.current.k);

    const nodeMap = new Map<string, GraphNode>();
    simNodes.forEach(n => nodeMap.set(n.id, n));

    const pathSet = new Set(highlightPathNodeIds);
    const edgeSet = new Set(highlightEdgeIds);

    // 1. Draw Links
    for (const link of simLinks) {
      const src = nodeMap.get(link.source);
      const tgt = nodeMap.get(link.target);
      if (!src || !tgt || src.x === undefined || tgt.x === undefined) continue;

      const isHighlighted = edgeSet.has(link.id) || (pathSet.has(src.id) && pathSet.has(tgt.id));

      ctx.beginPath();
      ctx.moveTo(src.x, src.y!);
      ctx.lineTo(tgt.x, tgt.y!);

      if (isHighlighted) {
        ctx.strokeStyle = '#06b6d4';
        ctx.lineWidth = 2.5;
        ctx.shadowColor = '#06b6d4';
        ctx.shadowBlur = 8;
      } else {
        ctx.strokeStyle = 'rgba(51, 65, 85, 0.6)';
        ctx.lineWidth = 1;
        ctx.shadowBlur = 0;
      }
      ctx.stroke();
      ctx.shadowBlur = 0;

      // Draw relation label on edge
      const midX = (src.x + tgt.x) / 2;
      const midY = (src.y! + tgt.y!) / 2;
      ctx.font = '9px monospace';
      ctx.fillStyle = isHighlighted ? '#38bdf8' : 'rgba(148, 163, 184, 0.6)';
      ctx.textAlign = 'center';
      ctx.fillText(link.relation_type, midX, midY - 4);
    }

    // 2. Draw Nodes
    for (const node of simNodes) {
      if (node.x === undefined || node.y === undefined) continue;

      const isSelected = selectedNodeId === node.id;
      const isPathNode = pathSet.has(node.id);
      const color = TYPE_COLORS[node.entity_type] || TYPE_COLORS.DEFAULT;
      const radius = Math.max(10, Math.min(22, 10 + (node.mention_count || 1) * 2));

      // Glow circle
      if (isSelected || isPathNode) {
        ctx.beginPath();
        ctx.arc(node.x, node.y, radius + 6, 0, 2 * Math.PI);
        ctx.fillStyle = isSelected ? 'rgba(59, 130, 246, 0.3)' : 'rgba(6, 182, 212, 0.3)';
        ctx.fill();
      }

      // Main Node Circle
      ctx.beginPath();
      ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI);
      ctx.fillStyle = color;
      ctx.fill();
      ctx.strokeStyle = isSelected ? '#ffffff' : '#0f172a';
      ctx.lineWidth = isSelected ? 2.5 : 1.5;
      ctx.stroke();

      // Node Label
      ctx.font = `${isSelected || isPathNode ? 'bold ' : ''}11px Plus Jakarta Sans, system-ui`;
      ctx.fillStyle = isSelected || isPathNode ? '#ffffff' : '#cbd5e1';
      ctx.textAlign = 'center';
      ctx.fillText(node.name, node.x, node.y + radius + 14);

      // Type Badge
      ctx.font = '8px monospace';
      ctx.fillStyle = 'rgba(148, 163, 184, 0.8)';
      ctx.fillText(node.entity_type, node.x, node.y + radius + 24);
    }

    ctx.restore();
  }, [simNodes, simLinks, highlightPathNodeIds, highlightEdgeIds, selectedNodeId]);

  // Handle Resize
  useEffect(() => {
    const handleResize = () => {
      const canvas = canvasRef.current;
      const container = containerRef.current;
      if (!canvas || !container) return;

      canvas.width = container.clientWidth * window.devicePixelRatio;
      canvas.height = container.clientHeight * window.devicePixelRatio;
      canvas.style.width = `${container.clientWidth}px`;
      canvas.style.height = `${container.clientHeight}px`;

      const ctx = canvas.getContext('2d');
      ctx?.scale(window.devicePixelRatio, window.devicePixelRatio);
      draw();
    };

    handleResize();
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, [draw]);

  // Mouse & Pan / Drag interactions
  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;

    const mouseX = (e.clientX - rect.left - transformRef.current.x) / transformRef.current.k;
    const mouseY = (e.clientY - rect.top - transformRef.current.y) / transformRef.current.k;

    // Check hit node
    let hitNode: GraphNode | null = null;
    for (const n of simNodes) {
      if (n.x === undefined || n.y === undefined) continue;
      const dx = mouseX - n.x;
      const dy = mouseY - n.y;
      if (dx * dx + dy * dy <= 24 * 24) {
        hitNode = n;
        break;
      }
    }

    if (hitNode) {
      draggedNodeRef.current = hitNode;
      onSelectNode?.(hitNode);
    } else {
      isDraggingRef.current = true;
      dragStartRef.current = { x: e.clientX, y: e.clientY };
      onSelectNode?.(null);
    }
  };

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (draggedNodeRef.current) {
      const rect = canvasRef.current?.getBoundingClientRect();
      if (!rect) return;
      const mouseX = (e.clientX - rect.left - transformRef.current.x) / transformRef.current.k;
      const mouseY = (e.clientY - rect.top - transformRef.current.y) / transformRef.current.k;
      draggedNodeRef.current.x = mouseX;
      draggedNodeRef.current.y = mouseY;
      draggedNodeRef.current.vx = 0;
      draggedNodeRef.current.vy = 0;
      draw();
      return;
    }

    if (isDraggingRef.current) {
      const dx = e.clientX - dragStartRef.current.x;
      const dy = e.clientY - dragStartRef.current.y;
      dragStartRef.current = { x: e.clientX, y: e.clientY };
      transformRef.current.x += dx;
      transformRef.current.y += dy;
      draw();
    }
  };

  const handleMouseUp = () => {
    draggedNodeRef.current = null;
    isDraggingRef.current = false;
  };

  const handleWheel = (e: React.WheelEvent<HTMLCanvasElement>) => {
    e.preventDefault();
    const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9;
    const newK = Math.min(3.0, Math.max(0.3, transformRef.current.k * zoomFactor));

    const rect = canvasRef.current?.getBoundingClientRect();
    if (!rect) return;
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;

    transformRef.current.x = mouseX - (mouseX - transformRef.current.x) * (newK / transformRef.current.k);
    transformRef.current.y = mouseY - (mouseY - transformRef.current.y) * (newK / transformRef.current.k);
    transformRef.current.k = newK;

    draw();
  };

  return (
    <div ref={containerRef} className="relative w-full h-full overflow-hidden bg-slate-950/80 rounded-xl border border-slate-800">
      <canvas
        ref={canvasRef}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onWheel={handleWheel}
        className="w-full h-full cursor-grab active:cursor-grabbing"
      />

      {/* Legend */}
      <div className="absolute bottom-3 left-3 bg-slate-900/90 backdrop-blur border border-slate-800/80 rounded-lg p-2.5 flex items-center gap-3 text-[11px] text-slate-300">
        <span className="font-semibold text-slate-400">Entity Types:</span>
        {Object.entries(TYPE_COLORS).map(([type, color]) => (
          <span key={type} className="inline-flex items-center gap-1.5">
            <span className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: color }} />
            {type}
          </span>
        ))}
      </div>
    </div>
  );
}
