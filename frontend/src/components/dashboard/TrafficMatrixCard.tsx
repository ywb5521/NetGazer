import { useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { fetchTrafficMatrix } from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import type { TrafficMatrixCell } from '@/types';

function formatIP(ip: string): string {
  return ip;
}

export function TrafficMatrixCard() {
  const { t } = useI18n();
  const [cells, setCells] = useState<TrafficMatrixCell[]>([]);
  const [hosts, setHosts] = useState<string[]>([]);
  const [maxBytes, setMaxBytes] = useState(1);

  useEffect(() => {
    let alive = true;
    const load = async () => {
      try {
        const data = await fetchTrafficMatrix();
        if (!alive) return;
        setCells(data);
        const ipSet = new Set<string>();
        for (const c of data) {
          ipSet.add(c.source);
          ipSet.add(c.destination);
        }
        const sorted = Array.from(ipSet).sort();
        setHosts(sorted);
        let max = 1;
        for (const c of data) {
          if (c.bytes > max) max = c.bytes;
        }
        setMaxBytes(max);
      } catch {
        // Matrix not critical, fail silently
      }
    };
    load();
    const timer = setInterval(load, 30000);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, []);

  if (hosts.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">{t.dashboard.trafficMatrix}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">{t.dashboard.loadingMatrix}</p>
        </CardContent>
      </Card>
    );
  }

  const byteMap = new Map<string, number>();
  for (const c of cells) {
    byteMap.set(`${c.source}|${c.destination}`, c.bytes);
  }

  const maxLabelLen = Math.max(...hosts.map((h) => formatIP(h).length));

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">
          {t.dashboard.trafficMatrix} (top {hosts.length})
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="overflow-auto max-h-[320px]">
          <table className="w-full text-xs border-collapse">
            <thead>
              <tr>
                <th className="sticky left-0 bg-card z-10 p-1" style={{ minWidth: maxLabelLen * 8 + 16 }} />
                {hosts.map((dst) => (
                  <th key={dst} className="p-1 font-mono font-normal text-muted-foreground whitespace-nowrap" style={{ writingMode: 'vertical-rl', textOrientation: 'mixed' }}>
                    {formatIP(dst)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {hosts.map((src) => (
                <tr key={src}>
                  <td className="sticky left-0 bg-card z-10 p-1 font-mono whitespace-nowrap">
                    {formatIP(src)}
                  </td>
                  {hosts.map((dst) => {
                    const bytes = byteMap.get(`${src}|${dst}`) || 0;
                    const intensity = bytes / maxBytes;
                    const bg = bytes > 0
                      ? `rgba(var(--chart-2-rgb, 34, 197, 94), ${0.1 + intensity * 0.8})`
                      : 'transparent';
                    return (
                      <td
                        key={dst}
                        className="p-1 text-center border border-border/20"
                        style={{ background: bg }}
                        title={bytes > 0 ? `${src} → ${dst}: ${formatBytes(bytes)}` : undefined}
                      >
                        {bytes > 0 ? formatBytes(bytes) : ''}
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );
}
