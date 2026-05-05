import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { fetchHostPeers, type HostPeer } from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';

interface Props {
  ip: string;
}

export function HostPeersTab({ ip }: Props) {
  const { t } = useI18n();
  const [peers, setPeers] = useState<HostPeer[]>([]);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    let cancelled = false;
    fetchHostPeers(ip)
      .then((data) => {
        if (!cancelled) { setPeers(data); setLoading(false); }
      })
      .catch(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [ip]);

  if (loading) {
    return <p className="text-sm text-muted-foreground py-8 text-center">{t.common.loading}</p>;
  }

  if (peers.length === 0) {
    return <p className="text-sm text-muted-foreground py-8 text-center">{t.common.empty}</p>;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">{t.hosts.peers} ({peers.length})</CardTitle>
      </CardHeader>
      <CardContent>
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-left text-xs text-muted-foreground">
              <th className="pb-2 font-medium">{t.dashboard.ip}</th>
              <th className="pb-2 font-medium text-right">{t.dashboard.bytes}</th>
              <th className="pb-2 font-medium text-right">{t.dashboard.packets}</th>
              <th className="pb-2 font-medium text-right">{t.nav.flows}</th>
            </tr>
          </thead>
          <tbody>
            {peers.map((p) => (
              <tr key={p.peer_ip} className="border-b border-border/50">
                <td className="py-1.5">
                  <button
                    className="font-mono text-xs text-primary hover:underline cursor-pointer"
                    onClick={() => navigate(`/hosts/${p.peer_ip}`)}
                  >
                    {p.peer_ip}
                  </button>
                </td>
                <td className="py-1.5 text-xs text-right">{formatBytes(p.bytes)}</td>
                <td className="py-1.5 text-xs text-right">{p.packets.toLocaleString()}</td>
                <td className="py-1.5 text-xs text-right">{p.flow_count}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
