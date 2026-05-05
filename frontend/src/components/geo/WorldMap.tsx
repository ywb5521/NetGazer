import { useMemo, useState } from 'react';
import { formatBytes } from '@/lib/utils';
import { WORLD_PATHS, COUNTRY_NAMES } from './worldPaths';

interface CountryData {
  code: string;
  name: string;
  bytes: number;
  percentage: number;
}

interface WorldMapProps {
  data: CountryData[];
  onCountryClick?: (code: string, name: string) => void;
}

function getHeatColor(percentage: number, maxPct: number): string {
  if (maxPct <= 0) return '#e2e8f0';
  const t = Math.min(percentage / Math.max(maxPct, 0.01), 1);
  if (t > 0.6) return `hsl(215,80%,${30 - t * 15}%)`;
  if (t > 0.3) return `hsl(215,65%,${50 - t * 25}%)`;
  if (t > 0.08) return `hsl(215,50%,${70 - t * 30}%)`;
  return '#e2e8f0';
}

export default function WorldMap({ data, onCountryClick }: WorldMapProps) {
  const [hovered, setHovered] = useState<string | null>(null);

  const { colorMap, dataByCode, maxPct } = useMemo(() => {
    const max = Math.max(...data.map((d) => d.percentage), 0.5);
    const colors: Record<string, string> = {};
    const byCode: Record<string, CountryData> = {};
    for (const d of data) {
      const code = (d.code || '').toUpperCase();
      colors[code] = getHeatColor(d.percentage, max);
      byCode[code] = d;
    }
    return { colorMap: colors, dataByCode: byCode, maxPct: max };
  }, [data]);

  const hasData = data.length > 0;

  return (
    <div className="relative w-full" style={{ aspectRatio: '2/1' }}>
      <svg viewBox="0 0 1000 500" className="w-full h-full block">
        <defs>
          <filter id="wm-shadow">
            <feDropShadow dx="0" dy="0.5" stdDeviation="1" floodColor="#00000018" />
          </filter>
        </defs>

        {/* Ocean */}
        <rect x="0" y="0" width="1000" height="500" fill="#eef3f9" rx="8" />

        {/* Subtle grid */}
        <g stroke="#d4dee8" strokeWidth="0.3" strokeDasharray="3 6" opacity="0.5">
          {[100, 200, 300, 400].map((y) => <line key={`h${y}`} x1="0" y1={y} x2="1000" y2={y} />)}
          {[200, 400, 600, 800].map((x) => <line key={`v${x}`} x1={x} y1="0" x2={x} y2="500" />)}
        </g>

        {/* Equator highlight */}
        <line x1="0" y1="250" x2="1000" y2="250" stroke="#c8d6e4" strokeWidth="0.5" opacity="0.6" />

        {/* Countries */}
        <g filter="url(#wm-shadow)">
          {Object.entries(WORLD_PATHS).map(([iso, d]) => {
            const hasData = dataByCode[iso];
            const fill = colorMap[iso] || '#f1f5f9';
            const isHovered = hovered === iso;
            const name = COUNTRY_NAMES[iso] || iso;

            return (
              <path
                key={iso}
                d={d}
                fill={fill}
                stroke={isHovered ? '#334155' : '#c0ccda'}
                strokeWidth={isHovered ? 1.2 : 0.4}
                className="cursor-pointer transition-all duration-100"
                style={{ filter: isHovered ? 'brightness(1.08)' : undefined }}
                onMouseEnter={() => setHovered(iso)}
                onMouseLeave={() => setHovered(null)}
                onClick={(e) => {
                  e.stopPropagation();
                  onCountryClick?.(iso, name);
                }}
              >
                <title>
                  {hasData
                    ? `${name}: ${formatBytes(hasData.bytes)} (${hasData.percentage.toFixed(1)}%)`
                    : name}
                </title>
              </path>
            );
          })}
        </g>

        {/* Hover tooltip */}
        {hovered && (
          <g transform="translate(14, 14)" pointerEvents="none">
            <rect x="0" y="0" width="230" height="48" rx="6" fill="#0f172a" opacity="0.88" />
            <text x="12" y="20" fill="#f8fafc" fontSize="12" fontWeight="bold" fontFamily="system-ui, sans-serif">
              {COUNTRY_NAMES[hovered] || hovered}
            </text>
            {dataByCode[hovered] ? (
              <text x="12" y="38" fill="#94a3b8" fontSize="11" fontFamily="system-ui, sans-serif">
                {dataByCode[hovered]!.percentage.toFixed(1)}% of traffic · {formatBytes(dataByCode[hovered]!.bytes)}
              </text>
            ) : (
              <text x="12" y="38" fill="#64748b" fontSize="11" fontFamily="system-ui, sans-serif">
                No traffic data
              </text>
            )}
          </g>
        )}

        {/* Legend */}
        {hasData && (
          <g transform="translate(14, 456)" pointerEvents="none">
            <rect x="0" y="0" width="280" height="34" rx="4" fill="#0f172a" opacity="0.72" />
            <text x="8" y="14" fill="#94a3b8" fontSize="9" fontFamily="system-ui, sans-serif">Low</text>
            {[0.04, 0.15, 0.35, 0.65, 1.0].map((t, i) => (
              <rect key={i} x={36 + i * 40} y="7" width="36" height="10" rx="2" fill={getHeatColor(t * maxPct, maxPct)} />
            ))}
            <text x="246" y="14" fill="#94a3b8" fontSize="9" fontFamily="system-ui, sans-serif">High</text>
            <text x="8" y="29" fill="#64748b" fontSize="9" fontFamily="system-ui, sans-serif">Click a country to drill down</text>
          </g>
        )}
      </svg>
    </div>
  );
}
