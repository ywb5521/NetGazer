import { useState, useCallback } from 'react';
import { StatsCards } from '@/components/dashboard/StatsCards';
import { TrafficChart } from '@/components/dashboard/TrafficChart';
import { TopHostsCard } from '@/components/dashboard/TopHostsCard';
import { TopTalkersCard } from '@/components/dashboard/TopTalkersCard';
import { AlertSummaryCard } from '@/components/dashboard/AlertSummaryCard';
import { DnsStatsCard } from '@/components/dashboard/DnsStatsCard';
import { PacketSizeChart } from '@/components/dashboard/PacketSizeChart';
import { TopAppsCard } from '@/components/dashboard/TopAppsCard';
import { TrafficMatrixCard } from '@/components/dashboard/TrafficMatrixCard';
import { FlowDirectionChart } from '@/components/dashboard/FlowDirectionChart';
import { TopologyMap } from '@/components/dashboard/TopologyMap';
import { NodesOverview } from '@/components/dashboard/NodesOverview';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useI18n } from '@/i18n/I18nContext';
import { LayoutGrid } from 'lucide-react';

const WIDGET_KEYS = [
  'stats',
  'traffic',
  'topHosts',
  'topTalkers',
  'alertSummary',
  'dnsStats',
  'topApps',
  'packetSize',
  'flowDirection',
  'trafficMatrix',
  'topology',
  'nodes',
] as const;

type WidgetKey = (typeof WIDGET_KEYS)[number];

const WIDGET_DASHBOARD_KEY: Record<WidgetKey, string> = {
  stats: 'statsCards',
  traffic: 'trafficChart',
  topHosts: 'topHosts',
  topTalkers: 'topTalkers',
  alertSummary: 'alertSummary',
  dnsStats: 'dnsStats',
  topApps: 'topApps',
  packetSize: 'packetSize',
  flowDirection: 'flowDirection',
  trafficMatrix: 'trafficMatrix',
  topology: 'topologyMap',
  nodes: 'nodesOverview',
};

const STORAGE_KEY = 'gtopng-dashboard-widgets';

function loadWidgetVisibility(): Record<WidgetKey, boolean> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return Object.fromEntries(WIDGET_KEYS.map((k) => [k, true])) as Record<WidgetKey, boolean>;
}

function saveWidgetVisibility(v: Record<WidgetKey, boolean>) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(v));
  } catch {}
}

export default function DashboardPage() {
  const { t } = useI18n();
  const [visible, setVisible] = useState<Record<WidgetKey, boolean>>(loadWidgetVisibility);
  const [customizing, setCustomizing] = useState(false);

  const toggle = useCallback((key: WidgetKey) => {
    setVisible((prev) => {
      const next = { ...prev, [key]: !prev[key] };
      saveWidgetVisibility(next);
      return next;
    });
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t.nav.dashboard}</h1>
        <div className="relative">
          <Button variant="ghost" size="sm" onClick={() => setCustomizing(!customizing)}>
            <LayoutGrid className="mr-1 h-3 w-3" />
            {t.dashboard.customize}
          </Button>
          {customizing && (
            <div className="absolute right-0 top-10 z-50 w-52 rounded-lg border bg-background shadow-lg p-3">
              <div className="space-y-2">
                {WIDGET_KEYS.map((key) => (
                  <label key={key} className="flex items-center justify-between gap-2 cursor-pointer">
                    <span className="text-xs">{(t.dashboard as any)[WIDGET_DASHBOARD_KEY[key]]}</span>
                    <Switch checked={visible[key]} onCheckedChange={() => toggle(key)} />
                  </label>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>

      {visible.stats && <StatsCards />}

      {visible.traffic && <TrafficChart />}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {visible.topHosts && <div><TopHostsCard /></div>}
        {visible.topTalkers && <div className="lg:col-span-2"><TopTalkersCard /></div>}
      </div>

      {(!visible.topHosts && !visible.topTalkers) ? null : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          {(visible.topHosts || visible.topTalkers) && visible.alertSummary && (
            <div><AlertSummaryCard /></div>
          )}
        </div>
      )}

      {/* AlertSummary standalone if no hosts/talkers beside it */}
      {visible.alertSummary && !visible.topHosts && !visible.topTalkers && (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div><AlertSummaryCard /></div>
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {visible.dnsStats && <div className="lg:col-span-2"><DnsStatsCard /></div>}
        {visible.topApps && <div><TopAppsCard /></div>}
      </div>

      {visible.packetSize && <PacketSizeChart />}

      {visible.flowDirection && <FlowDirectionChart />}

      {visible.trafficMatrix && <TrafficMatrixCard />}

      {visible.topology && <TopologyMap />}

      {visible.nodes && <NodesOverview />}
    </div>
  );
}
