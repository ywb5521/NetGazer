import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { useMemo, useState, useCallback, useRef } from 'react';
import type { Flow } from '@/types';

interface Node {
  ip: string;
  category: string;
  totalBytes: number;
  x: number;
  y: number;
  r: number;
}

interface Edge {
  src: string;
  dst: string;
  bytes: number;
  packets: number;
}

const NODE_COLORS: Record<string, string> = {
  Local: '#22c55e',
  Localhost: '#6b7280',
  Remote: '#3b82f6',
  Multicast: '#a855f7',
  Broadcast: '#f59e0b',
  'Link-Local': '#14b8a6',
};

export function TopologyMap() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();
  const [tooltip, setTooltip] = useState<{ x: number; y: number; text: string } | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const [offset, setOffset] = useState({ x: 0, y: 0 });
  const dragging = useRef(false);
  const lastPos = useRef({ x: 0, y: 0 });

  const { nodes, edges, maxBytes } = useMemo(() => {
    if (!snapshot?.flows) return { nodes: [], edges: [], maxBytes: 1 };

    // Aggregate traffic per host IP pair from flows
    const edgeMap: Record<string, Edge> = {};
    const hostBytes: Record<string, number> = {};
    const hostCategory: Record<string, string> = {};

    for (const f of snapshot.flows) {
      const key = [f.src_ip, f.dst_ip].sort().join('|');
      if (!edgeMap[key]) {
        edgeMap[key] = { src: f.src_ip, dst: f.dst_ip, bytes: 0, packets: 0 };
      }
      edgeMap[key].bytes += f.bytes;
      edgeMap[key].packets += f.packets;
      hostBytes[f.src_ip] = (hostBytes[f.src_ip] || 0) + f.bytes;
      hostBytes[f.dst_ip] = (hostBytes[f.dst_ip] || 0) + f.bytes;
    }

    // Get host categories
    for (const h of snapshot.hosts) {
      hostCategory[h.ip] = h.category || 'Remote';
    }

    // Top 20 hosts by traffic
    const topHosts = Object.entries(hostBytes)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 20);

    const topSet = new Set(topHosts.map(([ip]) => ip));

    // Filter edges where both endpoints are in top hosts
    const topEdges = Object.values(edgeMap)
      .filter((e) => topSet.has(e.src) && topSet.has(e.dst))
      .sort((a, b) => b.bytes - a.bytes)
      .slice(0, 40);

    const maxB = topEdges.length > 0 ? Math.max(...topEdges.map((e) => e.bytes)) : 1;

    // Circular layout
    const cx = 320, cy = 260, radius = 200;
    const hostList = topHosts.map(([ip]) => ip);
    const nodeList: Node[] = hostList.map((ip, i) => {
      const angle = (2 * Math.PI * i) / hostList.length - Math.PI / 2;
      const totalB = hostBytes[ip] || 0;
      const r = 8 + Math.min((totalB / maxB) * 20, 18);
      return {
        ip,
        category: hostCategory[ip] || 'Remote',
        totalBytes: totalB,
        x: cx + radius * Math.cos(angle),
        y: cy + radius * Math.sin(angle),
        r,
      };
    });

    return { nodes: nodeList, edges: topEdges, maxBytes: maxB };
  }, [snapshot]);

  const nodeMap = useMemo(() => {
    const m: Record<string, Node> = {};
    for (const n of nodes) m[n.ip] = n;
    return m;
  }, [nodes]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    dragging.current = true;
    lastPos.current = { x: e.clientX, y: e.clientY };
  }, []);

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!dragging.current) return;
    const dx = e.clientX - lastPos.current.x;
    const dy = e.clientY - lastPos.current.y;
    lastPos.current = { x: e.clientX, y: e.clientY };
    setOffset((prev) => ({ x: prev.x + dx, y: prev.y + dy }));
  }, []);

  const handleMouseUp = useCallback(() => {
    dragging.current = false;
  }, []);

  const strokeWidth = (bytes: number) => Math.max(0.5, (bytes / maxBytes) * 8);

  if (nodes.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t.dashboard.topologyMap}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground py-8 text-center">{t.common.empty}</p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.topologyMap} ({nodes.length} {t.hosts.hosts}, {edges.length} links)</CardTitle>
      </CardHeader>
      <CardContent>
        <svg
          ref={svgRef}
          viewBox="0 0 640 520"
          className="w-full h-auto select-none"
          style={{ cursor: dragging.current ? 'grabbing' : 'grab', minHeight: 400 }}
          onMouseDown={handleMouseDown}
          onMouseMove={handleMouseMove}
          onMouseUp={handleMouseUp}
          onMouseLeave={handleMouseUp}
        >
          <g transform={`translate(${offset.x},${offset.y})`}>
            {/* Edges */}
            {edges.map((e) => {
              const src = nodeMap[e.src];
              const dst = nodeMap[e.dst];
              if (!src || !dst) return null;
              const midX = (src.x + dst.x) / 2;
              const midY = (src.y + dst.y) / 2;
              return (
                <g key={`${e.src}|${e.dst}`}>
                  <line
                    x1={src.x} y1={src.y} x2={dst.x} y2={dst.y}
                    stroke="#334155"
                    strokeWidth={strokeWidth(e.bytes)}
                    strokeOpacity={0.6}
                    className="transition-opacity hover:stroke-blue-400"
                    onMouseEnter={(ev) => {
                      const rect = svgRef.current?.getBoundingClientRect();
                      if (rect) {
                        setTooltip({
                          x: ev.clientX - rect.left,
                          y: ev.clientY - rect.top,
                          text: `${e.src} → ${e.dst}\n${formatBytes(e.bytes)}`,
                        });
                      }
                    }}
                    onMouseLeave={() => setTooltip(null)}
                  />
                </g>
              );
            })}

            {/* Nodes */}
            {nodes.map((n) => (
              <g
                key={n.ip}
                onMouseEnter={(ev) => {
                  const rect = svgRef.current?.getBoundingClientRect();
                  if (rect) {
                    setTooltip({
                      x: ev.clientX - rect.left,
                      y: ev.clientY - rect.top,
                      text: `${n.ip}\n${formatBytes(n.totalBytes)}`,
                    });
                  }
                }}
                onMouseLeave={() => setTooltip(null)}
              >
                <circle
                  cx={n.x} cy={n.y} r={n.r}
                  fill={NODE_COLORS[n.category] || '#6b7280'}
                  fillOpacity={0.8}
                  stroke="#1e293b"
                  strokeWidth={1.5}
                  className="cursor-pointer hover:fill-opacity-100"
                />
                <text
                  x={n.x}
                  y={n.y + n.r + 12}
                  textAnchor="middle"
                  className="fill-muted-foreground"
                  fontSize={9}
                  fontFamily="monospace"
                >
                  {n.ip.length > 14 ? n.ip.slice(0, 11) + '...' : n.ip}
                </text>
              </g>
            ))}
          </g>

          {/* Tooltip */}
          {tooltip && (
            <foreignObject x={tooltip.x + 10} y={tooltip.y - 10} width={200} height={60}>
              <div className="rounded-lg border bg-background px-2 py-1 text-xs shadow-sm whitespace-pre-line">
                {tooltip.text}
              </div>
            </foreignObject>
          )}
        </svg>
        <div className="flex flex-wrap gap-3 mt-3 justify-center text-[10px] text-muted-foreground">
          {Object.entries(NODE_COLORS).map(([cat, color]) => (
            <span key={cat} className="inline-flex items-center gap-1">
              <span className="inline-block h-2 w-2 rounded-full" style={{ backgroundColor: color }} />
              {cat}
            </span>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
