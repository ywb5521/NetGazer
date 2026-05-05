import { useMemo } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

const BUCKETS = [
  { key: 'size_64', label: '0-64' },
  { key: 'size_128', label: '65-128' },
  { key: 'size_256', label: '129-256' },
  { key: 'size_512', label: '257-512' },
  { key: 'size_1024', label: '513-1024' },
  { key: 'size_1500', label: '1025-1500' },
  { key: 'size_gt1500', label: '1500+' },
] as const;

export function PacketSizeChart() {
  const { snapshot } = useAppContext();
  const { t } = useI18n();

  const data = useMemo(() => {
    if (!snapshot?.packet_size_dist) return [];
    const psd = snapshot.packet_size_dist;
    return BUCKETS.map((b) => ({
      name: b.label,
      packets: (psd as unknown as Record<string, number>)[b.key] || 0,
    }));
  }, [snapshot]);

  const hasData = data.some((d) => d.packets > 0);

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.dashboard.packetSizeDist}</CardTitle>
      </CardHeader>
      <CardContent>
        {!hasData ? (
          <p className="text-sm text-muted-foreground">{t.dashboard.noPacketData}</p>
        ) : (
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={data} margin={{ top: 4, right: 4, bottom: 4, left: 4 }}>
              <XAxis dataKey="name" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip
                contentStyle={{ fontSize: 12 }}
                formatter={(value: number) => [value.toLocaleString(), t.dashboard.packets]}
              />
              <Bar dataKey="packets" fill="hsl(var(--chart-2))" radius={[2, 2, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </CardContent>
    </Card>
  );
}
