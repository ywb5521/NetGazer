import { useEffect, useState, useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { fetchHostProtocols } from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { getCategory, CATEGORY_COLORS, baseProtocol } from '@/lib/categories';
import type { ProtocolStat } from '@/types';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', '#8b5cf6', '#ec4899', '#f59e0b'];

interface Props {
  ip: string;
}

export function HostProtocolsTab({ ip }: Props) {
  const { t } = useI18n();
  const [protocols, setProtocols] = useState<ProtocolStat[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    fetchHostProtocols(ip)
      .then((data) => {
        if (!cancelled) { setProtocols(data); setLoading(false); }
      })
      .catch(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [ip]);

  const grouped = useMemo(() => {
    // Group by base protocol (TLS (github.com) -> TLS)
    const map: Record<string, { bytes: number; packets: number; raw: string[] }> = {};
    for (const p of protocols) {
      const base = baseProtocol(p.protocol) || p.protocol;
      if (!map[base]) map[base] = { bytes: 0, packets: 0, raw: [] };
      map[base].bytes += p.bytes;
      map[base].packets += p.packets;
      if (!map[base].raw.includes(base)) map[base].raw.push(base);
    }
    const totalBytes = Object.values(map).reduce((s, v) => s + v.bytes, 0);
    return Object.entries(map)
      .map(([name, stats]) => ({
        name,
        bytes: stats.bytes,
        packets: stats.packets,
        category: getCategory(name),
        percentage: totalBytes > 0 ? (stats.bytes / totalBytes) * 100 : 0,
      }))
      .sort((a, b) => b.bytes - a.bytes);
  }, [protocols]);

  if (loading) {
    return <p className="text-sm text-muted-foreground py-8 text-center">{t.common.loading}</p>;
  }

  if (protocols.length === 0) {
    return <p className="text-sm text-muted-foreground py-8 text-center">{t.common.empty}</p>;
  }

  const totalBytes = grouped.reduce((s, g) => s + g.bytes, 0);

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.nav.protocols}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{grouped.length}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.total}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{formatBytes(totalBytes)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.packets}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-xl font-bold">{grouped.reduce((s, g) => s + g.packets, 0).toLocaleString()}</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">{t.nav.protocols}</CardTitle>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={280}>
              <PieChart>
                <Pie data={grouped} dataKey="bytes" nameKey="name" cx="50%" cy="50%" innerRadius={50} outerRadius={100}>
                  {grouped.map((g, i) => (
                    <Cell key={g.name} fill={CATEGORY_COLORS[g.category] || COLORS[i % COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip formatter={(value: number) => formatBytes(value)} />
                <Legend />
              </PieChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium">{t.hosts.traffic}</CardTitle>
          </CardHeader>
          <CardContent>
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-xs text-muted-foreground">
                  <th className="pb-2 font-medium">{t.flows.protocol}</th>
                  <th className="pb-2 font-medium text-right">{t.dashboard.bytes}</th>
                  <th className="pb-2 font-medium text-right">{t.dashboard.packets}</th>
                  <th className="pb-2 font-medium text-right">Share</th>
                </tr>
              </thead>
              <tbody>
                {grouped.map((g) => (
                  <tr key={g.name} className="border-b border-border/50">
                    <td className="py-1.5 text-xs font-medium">{g.name}</td>
                    <td className="py-1.5 text-xs text-right">{formatBytes(g.bytes)}</td>
                    <td className="py-1.5 text-xs text-right">{g.packets.toLocaleString()}</td>
                    <td className="py-1.5 text-xs text-right">
                      <div className="flex items-center justify-end gap-2">
                        <div className="h-1.5 w-16 rounded-full bg-muted overflow-hidden">
                          <div className="h-full rounded-full bg-primary" style={{ width: `${g.percentage}%` }} />
                        </div>
                        {g.percentage.toFixed(1)}%
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
