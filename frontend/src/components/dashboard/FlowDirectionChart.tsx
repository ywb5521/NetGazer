import { useMemo, useState, useRef } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { ArrowLeftRight } from 'lucide-react';

interface FlowPair {
  src: string;
  dst: string;
  bytes: number;
  packets: number;
  protocol: string;
}

interface SourceNode {
  ip: string;
  totalOut: number;
  y: number;
}

interface DestNode {
  ip: string;
  totalIn: number;
  y: number;
}

export function FlowDirectionChart() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();
  const [hovered, setHovered] = useState<FlowPair | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const { sources, dests, pairs, maxBytes } = useMemo(() => {
    if (!snapshot?.flows) return { sources: [], dests: [], pairs: [], maxBytes: 0 };

    const pairMap = new Map<string, FlowPair>();
    for (const f of snapshot.flows) {
      const key = `${f.src_ip}→${f.dst_ip}`;
      const existing = pairMap.get(key);
      if (existing) {
        existing.bytes += f.bytes;
        existing.packets += f.packets;
      } else {
        pairMap.set(key, {
          src: f.src_ip,
          dst: f.dst_ip,
          bytes: f.bytes,
          packets: f.packets,
          protocol: f.protocol,
        });
      }
    }

    const topPairs = Array.from(pairMap.values())
      .sort((a, b) => b.bytes - a.bytes)
      .slice(0, 15);

    const maxB = topPairs[0]?.bytes || 1;

    const srcMap = new Map<string, number>();
    const dstMap = new Map<string, number>();
    for (const p of topPairs) {
      srcMap.set(p.src, (srcMap.get(p.src) || 0) + p.bytes);
      dstMap.set(p.dst, (dstMap.get(p.dst) || 0) + p.bytes);
    }

    const srcSorted = Array.from(srcMap.entries())
      .sort((a, b) => b[1] - a[1]);
    const dstSorted = Array.from(dstMap.entries())
      .sort((a, b) => b[1] - a[1]);

    const PADDING = 40;
    const srcSpacing = 280 / Math.max(srcSorted.length, 1);
    const dstSpacing = 280 / Math.max(dstSorted.length, 1);

    const sourcesOut: SourceNode[] = srcSorted.map(([ip, total], i) => ({
      ip,
      totalOut: total,
      y: PADDING + i * srcSpacing + srcSpacing / 2,
    }));

    const destsOut: DestNode[] = dstSorted.map(([ip, total], i) => ({
      ip,
      totalIn: total,
      y: PADDING + i * dstSpacing + dstSpacing / 2,
    }));

    return {
      sources: sourcesOut,
      dests: destsOut,
      pairs: topPairs,
      maxBytes: maxB,
    };
  }, [snapshot]);

  if (!snapshot || pairs.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium flex items-center gap-2">
            <ArrowLeftRight className="h-4 w-4" />
            {t.dashboard.flowDirection}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground py-8 text-center">
            {t.common.accumulatingData}
          </p>
        </CardContent>
      </Card>
    );
  }

  const getSrcY = (ip: string) => sources.find((s) => s.ip === ip)?.y ?? 0;
  const getDstY = (ip: string) => dests.find((d) => d.ip === ip)?.y ?? 0;

  const NODE_W = 96;
  const SRC_X = 2;           // source rect left
  const SRC_CX = SRC_X + NODE_W / 2;  // source text center
  const SRC_RX = SRC_X + NODE_W;       // right edge of source rect (curve start)
  const DST_X = 422;         // dest rect left
  const DST_CX = DST_X + NODE_W / 2;  // dest text center
  const DST_LX = DST_X;      // left edge of dest rect (curve end)

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <ArrowLeftRight className="h-4 w-4" />
          {t.dashboard.flowDirection} — Top {pairs.length}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="relative">
          <svg
            ref={svgRef}
            viewBox="0 0 540 320"
            className="w-full"
            style={{ maxHeight: 360 }}
          >
            {/* Flow ribbons */}
            {pairs.map((pair) => {
              const srcY = getSrcY(pair.src);
              const dstY = getDstY(pair.dst);
              const thickness = Math.max(2, (pair.bytes / maxBytes) * 24);
              const opacity = 0.25 + (pair.bytes / maxBytes) * 0.55;

              const cp = (SRC_RX + DST_LX) / 2;
              const d = `M ${SRC_RX} ${srcY} C ${cp} ${srcY}, ${cp} ${dstY}, ${DST_LX} ${dstY}`;

              const isHovered = hovered?.src === pair.src && hovered?.dst === pair.dst;

              return (
                <g key={`${pair.src}-${pair.dst}`}>
                  <path
                    d={d}
                    fill="none"
                    stroke="transparent"
                    strokeWidth={14}
                    style={{ cursor: 'pointer' }}
                    onMouseEnter={() => setHovered(pair)}
                    onMouseLeave={() => setHovered(null)}
                  />
                  <path
                    d={d}
                    fill="none"
                    stroke="hsl(var(--chart-1))"
                    strokeWidth={isHovered ? thickness + 2 : thickness}
                    strokeOpacity={isHovered ? opacity + 0.3 : opacity}
                    strokeLinecap="round"
                    style={{ transition: 'all 0.15s', cursor: 'pointer' }}
                    onMouseEnter={() => setHovered(pair)}
                    onMouseLeave={() => setHovered(null)}
                  />
                </g>
              );
            })}

            {/* Source nodes */}
            {sources.map((s) => (
              <g key={`src-${s.ip}`}>
                <rect
                  x={SRC_X}
                  y={s.y - 10}
                  width={NODE_W}
                  height={20}
                  rx={4}
                  fill="hsl(var(--chart-2))"
                  opacity={0.85}
                />
                <text
                  x={SRC_CX}
                  y={s.y + 4}
                  textAnchor="middle"
                  className="fill-white text-[7px] font-mono"
                >
                  {s.ip}
                </text>
              </g>
            ))}

            {/* Destination nodes */}
            {dests.map((d) => (
              <g key={`dst-${d.ip}`}>
                <rect
                  x={DST_X}
                  y={d.y - 10}
                  width={NODE_W}
                  height={20}
                  rx={4}
                  fill="hsl(var(--chart-3))"
                  opacity={0.85}
                />
                <text
                  x={DST_CX}
                  y={d.y + 4}
                  textAnchor="middle"
                  className="fill-white text-[7px] font-mono"
                >
                  {d.ip}
                </text>
              </g>
            ))}

            {/* Column labels */}
            <text x={SRC_CX} y={16} textAnchor="middle" className="fill-muted-foreground text-[10px] font-medium">
              {t.common.sourceIps}
            </text>
            <text x={DST_CX} y={16} textAnchor="middle" className="fill-muted-foreground text-[10px] font-medium">
              {t.common.destinationIps}
            </text>
          </svg>

          {/* Tooltip */}
          {hovered && (
            <div className="absolute top-0 right-0 rounded-lg border bg-background p-2 shadow-sm text-xs z-10">
              <div className="font-mono font-medium">
                {hovered.src} → {hovered.dst}
              </div>
              <div className="text-muted-foreground mt-0.5">
                {formatBytes(hovered.bytes)} · {hovered.packets.toLocaleString()} pkts · {hovered.protocol}
              </div>
            </div>
          )}
        </div>

        {/* Legend */}
        <div className="flex items-center gap-4 mt-2 text-[10px] text-muted-foreground justify-center">
          <div className="flex items-center gap-1">
            <div className="h-2.5 w-2.5 rounded-sm" style={{ background: 'hsl(var(--chart-2))' }} />
            {t.common.sourceIps}
          </div>
          <div className="flex items-center gap-1">
            <div className="h-2.5 w-2.5 rounded-sm" style={{ background: 'hsl(var(--chart-3))' }} />
            {t.common.destinationIps}
          </div>
          <div className="flex items-center gap-1">
            <svg width="24" height="10"><line x1="0" y1="5" x2="24" y2="5" stroke="hsl(var(--chart-1))" strokeWidth="3" opacity="0.5" /></svg>
            {t.common.trafficFlow}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
