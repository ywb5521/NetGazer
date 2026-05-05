import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { HostOverviewTab } from '@/components/hosts/HostOverviewTab';
import { HostTrafficTab } from '@/components/hosts/HostTrafficTab';
import { HostProtocolsTab } from '@/components/hosts/HostProtocolsTab';
import { HostPeersTab } from '@/components/hosts/HostPeersTab';
import { ArrowLeft } from 'lucide-react';
import type { Host } from '@/types';

const BASE = `${import.meta.env.BASE_URL}api`;

async function fetchSingleHost(ip: string, nodeId?: string): Promise<Host> {
  const url = nodeId
    ? `${BASE}/hosts/${encodeURIComponent(ip)}?node_id=${encodeURIComponent(nodeId)}`
    : `${BASE}/hosts/${encodeURIComponent(ip)}`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to fetch host: ${res.status}`);
  return res.json();
}

export default function HostDetailPage() {
  const { ip } = useParams<{ ip: string }>();
  const navigate = useNavigate();
  const { snapshot, selectedNode } = useAppContext();
  const { t } = useI18n();
  const [host, setHost] = useState<Host | undefined>(undefined);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!ip) {
      setLoading(false);
      return;
    }
    // Try WebSocket snapshot first (fast path)
    const wsHost = snapshot?.hosts?.find((h) => h.ip === ip);
    if (wsHost) {
      setHost(wsHost);
      setLoading(false);
      return;
    }
    // Fallback to REST
    setLoading(true);
    fetchSingleHost(ip, selectedNode || undefined)
      .then(setHost)
      .catch(() => setHost(undefined))
      .finally(() => setLoading(false));
  }, [ip, snapshot, selectedNode]);

  if (!ip) {
    return <p className="text-muted-foreground py-8 text-center">{t.hosts.hostDetail}</p>;
  }

  if (loading) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => navigate('/hosts')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> {t.hosts.backToList}
        </Button>
        <p className="text-muted-foreground py-8 text-center text-sm">{t.common.loading}</p>
      </div>
    );
  }

  if (!host) {
    return (
      <div className="space-y-4">
        <Button variant="ghost" onClick={() => navigate('/hosts')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> {t.hosts.backToList}
        </Button>
        <p className="text-muted-foreground py-8 text-center">{t.hosts.hostDetail}: {ip}</p>
      </div>
    );
  }

  const label = host.hostname || host.vendor || ip;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => navigate('/hosts')}>
          <ArrowLeft className="mr-2 h-4 w-4" /> {t.hosts.backToList}
        </Button>
        <div>
          <h1 className="text-2xl font-bold tracking-tight">{label}</h1>
          <p className="text-sm text-muted-foreground font-mono">{ip}</p>
        </div>
      </div>

      <Tabs defaultValue="overview">
        <TabsList>
          <TabsTrigger value="overview">{t.hosts.overview}</TabsTrigger>
          <TabsTrigger value="traffic">{t.hosts.traffic}</TabsTrigger>
          <TabsTrigger value="protocols">{t.hosts.protocols}</TabsTrigger>
          <TabsTrigger value="peers">{t.hosts.peers}</TabsTrigger>
        </TabsList>
        <TabsContent value="overview" className="mt-4">
          <HostOverviewTab host={host} ip={ip} />
        </TabsContent>
        <TabsContent value="traffic" className="mt-4">
          <HostTrafficTab ip={ip} />
        </TabsContent>
        <TabsContent value="protocols" className="mt-4">
          <HostProtocolsTab ip={ip} />
        </TabsContent>
        <TabsContent value="peers" className="mt-4">
          <HostPeersTab ip={ip} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
