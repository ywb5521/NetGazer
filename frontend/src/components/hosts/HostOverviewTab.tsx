import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes, countryToFlag } from '@/lib/utils';
import type { Host } from '@/types';
import { Monitor, Clock, Network, HardDrive } from 'lucide-react';

interface Props {
  host?: Host;
  ip: string;
}

export function HostOverviewTab({ host, ip }: Props) {
  const { t } = useI18n();

  if (!host) {
    return <p className="text-muted-foreground py-8 text-center">{t.common.empty}</p>;
  }

  const totalBytes = host.bytes_sent + host.bytes_received;
  const totalPackets = host.packets_sent + host.packets_received;
  const now = Date.now();
  const durationMs = now - host.first_seen;
  const durationSec = durationMs / 1000;

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.total}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-lg font-bold">{formatBytes(totalBytes)}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.dashboard.packets}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-lg font-bold">{totalPackets.toLocaleString()}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.hosts.activeFlows}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-lg font-bold">{host.active_flows}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-xs font-medium text-muted-foreground">{t.flows.duration}</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-lg font-bold">{durationSec >= 3600 ? `${Math.round(durationSec / 3600)}h` : durationSec >= 60 ? `${Math.round(durationSec / 60)}m` : `${Math.round(durationSec)}s`}</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Monitor className="h-4 w-4" /> {t.hosts.identity}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.dashboard.ip}</dt>
                <dd className="font-mono">{host.ip}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">MAC</dt>
                <dd className="font-mono">{host.mac || '-'}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.dashboard.hostname}</dt>
                <dd>{host.hostname || '-'}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Vendor</dt>
                <dd>{host.vendor || '-'}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Category</dt>
                <dd>{host.category ? <Badge variant="outline" className="text-xs">{host.category}</Badge> : '-'}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Country</dt>
                <dd>{host.country ? <>{countryToFlag(host.country) || ''} {host.country}</> : '-'}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.flows.node}</dt>
                <dd><Badge variant="outline" className="text-xs">{host.node_id}</Badge></dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Network className="h-4 w-4" /> {t.hosts.trafficStats}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.bytesSent}</dt>
                <dd className="font-mono text-emerald-500">{formatBytes(host.bytes_sent)}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.bytesReceived}</dt>
                <dd className="font-mono text-blue-500">{formatBytes(host.bytes_received)}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.packetsSent}</dt>
                <dd className="font-mono">{host.packets_sent.toLocaleString()}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.packetsReceived}</dt>
                <dd className="font-mono">{host.packets_received.toLocaleString()}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <Clock className="h-4 w-4" /> {t.hosts.timeline}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.firstSeen}</dt>
                <dd>{new Date(host.first_seen).toLocaleString()}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">{t.hosts.lastSeen}</dt>
                <dd>{new Date(host.last_seen).toLocaleString()}</dd>
              </div>
            </dl>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-medium flex items-center gap-2">
              <HardDrive className="h-4 w-4" /> {t.hosts.avgRate}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <dl className="space-y-2 text-sm">
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Avg Bytes/s</dt>
                <dd className="font-mono">
                  {durationSec > 0 ? formatBytes((totalBytes / durationSec)) + '/s' : '-'}
                </dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-muted-foreground">Avg Packets/s</dt>
                <dd className="font-mono">
                  {durationSec > 0 ? Math.round(totalPackets / durationSec).toLocaleString() + '/s' : '-'}
                </dd>
              </div>
            </dl>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
