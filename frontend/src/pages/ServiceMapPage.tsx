import { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { fetchServiceMap, type ServiceNode, type ServiceEdge } from '@/lib/api';
import { GitBranch, ExternalLink, X } from 'lucide-react';

const COLORS = [
  'hsl(var(--chart-1))', 'hsl(var(--chart-2))', 'hsl(var(--chart-3))',
  'hsl(var(--chart-4))', 'hsl(var(--chart-5))', '#8b5cf6', '#06b6d4',
  '#f59e0b', '#ef4444', '#10b981',
];

function circularLayout(services: ServiceNode[], r: number): Record<string, { x: number; y: number }> {
  const cx = 440, cy = 260;
  const n = services.length;
  const positions: Record<string, { x: number; y: number }> = {};
  if (n === 0) return positions;
  services.forEach((s, i) => {
    const angle = (2 * Math.PI * i) / n - Math.PI / 2;
    positions[s.name] = {
      x: cx + r * Math.cos(angle),
      y: cy + r * Math.sin(angle),
    };
  });
  return positions;
}

export default function ServiceMapPage() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [services, setServices] = useState<ServiceNode[]>([]);
  const [edges, setEdges] = useState<ServiceEdge[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedNode, setSelectedNode] = useState<ServiceNode | null>(null);
  const [selectedEdge, setSelectedEdge] = useState<ServiceEdge | null>(null);
  const [hoveredNode, setHoveredNode] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const data = await fetchServiceMap();
        if (!cancelled) {
          setServices(Array.isArray(data.services) ? data.services : []);
          setEdges(Array.isArray(data.edges) ? data.edges : []);
        }
      } catch {
        if (!cancelled) { setServices([]); setEdges([]); }
      }
      if (!cancelled) setLoading(false);
    };
    load();
    const interval = setInterval(load, 20000);
    return () => { cancelled = true; clearInterval(interval); };
  }, []);

  const maxBytes = useMemo(() => Math.max(...services.map((s) => s.bytes), 1), [services]);
  const positions = useMemo(() => circularLayout(services, 190), [services]);

  const colorMap = useMemo(() => {
    const m: Record<string, string> = {};
    services.forEach((s, i) => {
      m[s.name] = COLORS[i % COLORS.length];
    });
    return m;
  }, [services]);

  const handleNodeClick = (node: ServiceNode) => {
    setSelectedNode(node);
  };

  const handleEdgeClick = (edge: ServiceEdge) => {
    setSelectedEdge(edge);
  };

  const handleViewFlowsForService = (svcName: string) => {
    navigate(`/flows?app=${encodeURIComponent(svcName)}`);
  };

  const handleViewFlowsForEdge = (src: string, dst: string) => {
    navigate(`/flows?search=${encodeURIComponent(src)}`);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
        <GitBranch className="h-6 w-6" />
        {t.serviceMap.title}
      </h1>

      {loading && services.length === 0 ? (
        <p className="text-sm text-muted-foreground py-8 text-center">{t.serviceMap.loading}</p>
      ) : services.length === 0 ? (
        <p className="text-sm text-muted-foreground py-8 text-center">{t.serviceMap.empty}</p>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <Card className="lg:col-span-2">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">{t.serviceMap.title}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="w-full aspect-[1.7]">
                <svg viewBox="0 0 880 520" className="w-full h-full">
                  {/* Edges */}
                  {edges.map((e, i) => {
                    const src = positions[e.src];
                    const dst = positions[e.dst];
                    if (!src || !dst) return null;
                    const opacity = Math.max(0.12, Math.min(0.55, e.bytes / maxBytes));
                    const isHighlighted = selectedEdge && selectedEdge.src === e.src && selectedEdge.dst === e.dst;
                    return (
                      <g key={`edge-${i}`}>
                        {/* Invisible wider click target */}
                        <line
                          x1={src.x} y1={src.y} x2={dst.x} y2={dst.y}
                          stroke="transparent" strokeWidth={12}
                          className="cursor-pointer"
                          onClick={() => handleEdgeClick(e)}
                        />
                        <line
                          x1={src.x} y1={src.y} x2={dst.x} y2={dst.y}
                          stroke={isHighlighted ? 'hsl(var(--primary))' : 'hsl(var(--muted-foreground))'}
                          strokeOpacity={isHighlighted ? 0.7 : opacity}
                          strokeWidth={isHighlighted ? 2.5 : Math.max(1, 2.5 * (e.bytes / maxBytes))}
                          className="cursor-pointer transition-all"
                          onClick={() => handleEdgeClick(e)}
                        />
                      </g>
                    );
                  })}
                  {/* Nodes */}
                  {services.map((s) => {
                    const pos = positions[s.name];
                    if (!pos) return null;
                    const size = 22 + 32 * (s.bytes / maxBytes);
                    const isSelected = selectedNode?.name === s.name;
                    const isHovered = hoveredNode === s.name;
                    return (
                      <g
                        key={s.name}
                        className="cursor-pointer"
                        onClick={() => handleNodeClick(s)}
                        onMouseEnter={() => setHoveredNode(s.name)}
                        onMouseLeave={() => setHoveredNode(null)}
                      >
                        {/* Halo ring */}
                        {(isSelected || isHovered) && (
                          <circle
                            cx={pos.x} cy={pos.y} r={size + 8}
                            fill="none"
                            stroke={colorMap[s.name] || COLORS[0]}
                            strokeWidth={3}
                            strokeOpacity={0.3}
                          />
                        )}
                        <circle
                          cx={pos.x} cy={pos.y} r={size}
                          fill={colorMap[s.name] || COLORS[0]}
                          fillOpacity={isHovered ? 1 : 0.85}
                          stroke="var(--background)"
                          strokeWidth={isSelected ? 3 : 2}
                          className="transition-all"
                        />
                        <text
                          x={pos.x} y={pos.y + size + 15}
                          textAnchor="middle"
                          className="text-[11px] fill-muted-foreground font-semibold"
                          style={{ pointerEvents: 'none' }}
                        >
                          {s.name}
                        </text>
                        <text
                          x={pos.x} y={pos.y + 4}
                          textAnchor="middle"
                          className="text-[10px] fill-white font-semibold"
                          style={{ pointerEvents: 'none' }}
                        >
                          {formatBytes(s.bytes)}
                        </text>
                      </g>
                    );
                  })}
                </svg>
              </div>
            </CardContent>
          </Card>

          <div className="space-y-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-sm">{t.serviceMap.services}</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t.flows.protocol}</TableHead>
                      <TableHead className="text-right">{t.dashboard.bytes}</TableHead>
                      <TableHead className="text-right">{t.hosts.hosts}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {services.slice(0, 15).map((s) => (
                      <TableRow
                        key={s.name}
                        className="cursor-pointer hover:bg-muted/50"
                        onClick={() => handleNodeClick(s)}
                      >
                        <TableCell className="text-xs">
                          <div className="flex items-center gap-2">
                            <div className="h-3 w-3 rounded-full" style={{ backgroundColor: colorMap[s.name] }} />
                            {s.name}
                          </div>
                        </TableCell>
                        <TableCell className="text-xs text-right">{formatBytes(s.bytes)}</TableCell>
                        <TableCell className="text-xs text-right">{s.hosts}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
            {edges.length > 0 && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">{t.serviceMap.edges}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-1 max-h-[300px] overflow-auto">
                    {edges.slice(0, 20).map((e, i) => (
                      <div
                        key={i}
                        className="flex items-center justify-between text-xs p-1.5 rounded hover:bg-muted/50 cursor-pointer"
                        onClick={() => handleEdgeClick(e)}
                      >
                        <span>{e.src} → {e.dst}</span>
                        <span className="text-muted-foreground">{formatBytes(e.bytes)}</span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}
          </div>
        </div>
      )}

      {/* Service Node Detail Dialog */}
      <Dialog open={!!selectedNode} onOpenChange={() => setSelectedNode(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <div
                className="h-4 w-4 rounded-full"
                style={{ backgroundColor: selectedNode ? colorMap[selectedNode.name] : '#888' }}
              />
              {selectedNode?.name || ''}
            </DialogTitle>
            <DialogDescription>
              {selectedNode && (
                <span>{formatBytes(selectedNode.bytes)} total · {selectedNode.hosts} hosts</span>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {selectedNode && (
              <>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground text-xs">{t.dashboard.total}</p>
                    <p className="font-semibold">{formatBytes(selectedNode.bytes)}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground text-xs">{t.hosts.hosts}</p>
                    <p className="font-semibold">{selectedNode.hosts}</p>
                  </div>
                </div>
                {/* Show edges for this service */}
                {edges.filter((e) => e.src === selectedNode.name || e.dst === selectedNode.name).length > 0 && (
                  <div>
                    <p className="text-xs font-semibold mb-2">{t.serviceMap.edges}</p>
                    <div className="space-y-1 max-h-[200px] overflow-auto">
                      {edges
                        .filter((e) => e.src === selectedNode.name || e.dst === selectedNode.name)
                        .slice(0, 15)
                        .map((e, i) => (
                          <div
                            key={i}
                            className="flex items-center justify-between text-xs p-1.5 rounded hover:bg-muted/50 cursor-pointer"
                            onClick={() => { setSelectedEdge(e); setSelectedNode(null); }}
                          >
                            <span>{e.src} → {e.dst}</span>
                            <span className="text-muted-foreground">{formatBytes(e.bytes)}</span>
                          </div>
                        ))}
                    </div>
                  </div>
                )}
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full"
                  onClick={() => handleViewFlowsForService(selectedNode.name)}
                >
                  <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                  查看该服务的所有流
                </Button>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* Edge Detail Dialog */}
      <Dialog open={!!selectedEdge} onOpenChange={() => setSelectedEdge(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="text-sm">
              {selectedEdge?.src} → {selectedEdge?.dst}
            </DialogTitle>
            <DialogDescription>
              {selectedEdge && (
                <span>{formatBytes(selectedEdge.bytes)} traffic between services</span>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            {selectedEdge && (
              <>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <p className="text-muted-foreground text-xs">源服务</p>
                    <p className="font-semibold">{selectedEdge.src}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground text-xs">目标服务</p>
                    <p className="font-semibold">{selectedEdge.dst}</p>
                  </div>
                  <div className="col-span-2">
                    <p className="text-muted-foreground text-xs">{t.dashboard.total}</p>
                    <p className="font-semibold">{formatBytes(selectedEdge.bytes)}</p>
                  </div>
                </div>
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full"
                  onClick={() => handleViewFlowsForEdge(selectedEdge.src, selectedEdge.dst)}
                >
                  <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                  查看相关流
                </Button>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
