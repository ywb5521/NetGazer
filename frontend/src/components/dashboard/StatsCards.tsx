import { useRef, useEffect, useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Activity, Monitor, Workflow, ArrowUp } from 'lucide-react';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes, formatPackets } from '@/lib/utils';

export function StatsCards() {
  const { snapshot, prevSnapshot } = useAppContext();
  const { t } = useI18n();
  const emaRef = useRef<{ bytes: number; packets: number } | null>(null);
  const [smoothBytes, setSmoothBytes] = useState(0);
  const [smoothPackets, setSmoothPackets] = useState(0);

  useEffect(() => {
    if (!snapshot) return;
    const bytes = snapshot.traffic.bytes_per_sec || 0;
    const packets = snapshot.traffic.packets_per_sec || 0;
    const prev = emaRef.current;
    const alpha = 0.3;
    if (prev) {
      setSmoothBytes(prev.bytes * (1 - alpha) + bytes * alpha);
      setSmoothPackets(prev.packets * (1 - alpha) + packets * alpha);
    } else {
      setSmoothBytes(bytes);
      setSmoothPackets(packets);
    }
    emaRef.current = { bytes: smoothBytes || bytes, packets: smoothPackets || packets };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [snapshot]);

  if (!snapshot) {
    return (
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {[...Array(4)].map((_, i) => (
          <Card key={i}>
            <CardHeader className="pb-2">
              <div className="h-4 w-24 bg-muted rounded animate-pulse" />
            </CardHeader>
            <CardContent>
              <div className="h-8 w-32 bg-muted rounded animate-pulse" />
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  const onlineNodes = (snapshot.nodes || []).filter((n) => n.online).length;

  const cards = [
    {
      title: t.dashboard.throughput,
      value: `${formatBytes(smoothBytes)}/s`,
      icon: Activity,
      subtitle: `${onlineNodes}${t.dashboard.nodes}`,
    },
    {
      title: t.dashboard.packetsPerSec,
      value: formatPackets(smoothPackets),
      icon: ArrowUp,
      subtitle: t.dashboard.realtime,
    },
    {
      title: t.dashboard.activeHosts,
      value: (snapshot.hosts || []).length.toLocaleString(),
      icon: Monitor,
      subtitle: t.dashboard.totalTracked,
    },
    {
      title: t.dashboard.activeFlows,
      value: (snapshot.flows || []).length.toLocaleString(),
      icon: Workflow,
      subtitle: t.dashboard.currentConnections,
    },
  ];

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {cards.map((card) => (
        <Card key={card.title}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {card.title}
            </CardTitle>
            <card.icon className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold tracking-tight">{card.value}</div>
            <p className="text-xs text-muted-foreground mt-1">{card.subtitle}</p>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
