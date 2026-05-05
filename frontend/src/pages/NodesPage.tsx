import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import { CircleCheck, CircleX, Server, Activity, ChevronDown, ChevronRight, Network } from 'lucide-react';

function formatUptime(seconds: number): string {
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function HealthBar({ label, value, unit, warn = 50, crit = 80 }: { label: string; value: number; unit: string; warn?: number; crit?: number }) {
  const color = value >= crit ? 'bg-red-500' : value >= warn ? 'bg-yellow-500' : 'bg-green-500';
  return (
    <div className="flex items-center gap-2">
      <span className="text-[10px] w-8 text-muted-foreground">{label}</span>
      <div className="flex-1 h-2 bg-muted rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${Math.min(value, 100)}%` }} />
      </div>
      <span className="text-[10px] w-12 text-right tabular-nums">{value.toFixed(1)}{unit}</span>
    </div>
  );
}

function NodeInterfaces({ node }: { node: import('@/types').NodeInfo }) {
  const [expanded, setExpanded] = useState(false);
  const ifaces = node.interface_info;
  if (!ifaces || ifaces.length === 0) return null;

  return (
    <div className="mt-3 border-t border-border pt-3">
      <Button
        variant="ghost"
        size="sm"
        className="h-6 w-full text-xs justify-start px-1 -ml-1"
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <ChevronDown className="mr-1 h-3 w-3" /> : <ChevronRight className="mr-1 h-3 w-3" />}
        {ifaces.length} interfaces
      </Button>
      {expanded && (
        <div className="mt-2 space-y-1.5">
          {ifaces.map((iface) => (
            <div key={iface.name} className="flex items-center gap-2 rounded bg-muted/40 px-2 py-1.5">
              <Network className="h-3 w-3 text-muted-foreground shrink-0" />
              <span className="text-xs font-medium min-w-0 truncate">{iface.name}</span>
              <span className="text-xs text-muted-foreground ml-auto tabular-nums whitespace-nowrap">
                {formatBytes(iface.bytes_per_sec)}/s
              </span>
              <span className="text-xs text-muted-foreground tabular-nums whitespace-nowrap">
                {iface.hosts_count} hosts
              </span>
              <span className="text-xs text-muted-foreground tabular-nums whitespace-nowrap">
                {iface.flows_count} flows
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export default function NodesPage() {
  const { snapshot, connected } = useAppContext();
  const { t } = useI18n();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight">{t.nav.nodes}</h1>

      {!snapshot || snapshot.nodes.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center">
            <Server className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-muted-foreground">
              {connected ? t.nodes.online + ' - ' + t.common.empty : t.common.disconnected}
            </p>
            <p className="text-xs text-muted-foreground mt-1">
              {t.nodes.startAgent}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
          {snapshot.nodes.map((node) => (
            <Card key={node.node_id}>
              <CardHeader className="pb-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    {node.online ? (
                      <CircleCheck className="h-5 w-5 text-green-500" />
                    ) : (
                      <CircleX className="h-5 w-5 text-red-500" />
                    )}
                    <div>
                      <CardTitle className="text-base">{node.node_id}</CardTitle>
                      <p className="text-xs text-muted-foreground">{node.interface}</p>
                    </div>
                  </div>
                  <Badge variant={node.online ? 'default' : 'secondary'}>
                    {node.online ? t.nodes.online : t.nodes.offline}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-3 gap-4">
                  <div className="rounded-lg bg-muted/50 p-3 text-center">
                    <Activity className="mx-auto h-4 w-4 text-muted-foreground mb-1" />
                    <div className="text-lg font-bold">{formatBytes(node.bytes_per_sec)}/s</div>
                    <div className="text-xs text-muted-foreground">{t.dashboard.throughput}</div>
                  </div>
                  <div className="rounded-lg bg-muted/50 p-3 text-center">
                    <div className="text-lg font-bold">{node.hosts_count}</div>
                    <div className="text-xs text-muted-foreground">{t.nav.hosts}</div>
                  </div>
                  <div className="rounded-lg bg-muted/50 p-3 text-center">
                    <div className="text-lg font-bold">{node.flows_count}</div>
                    <div className="text-xs text-muted-foreground">{t.nav.flows}</div>
                  </div>
                </div>

                <NodeInterfaces node={node} />

                {node.system_health && (
                  <div className="mt-3 border-t border-border pt-3 space-y-2">
                    <div className="text-xs font-medium text-muted-foreground">{t.nodes.systemHealth}</div>
                    <div className="space-y-1.5">
                      <HealthBar label="CPU" value={node.system_health.cpu_percent} unit="%" warn={50} crit={80} />
                      <HealthBar label="MEM" value={node.system_health.mem_percent} unit="%" warn={50} crit={80} />
                    </div>
                    <div className="flex gap-4 text-[10px] text-muted-foreground">
                      <span>{t.nodes.diskFree} {formatBytes(node.system_health.disk_free_bytes)}</span>
                      <span>{t.nodes.uptime} {formatUptime(node.system_health.uptime_seconds)}</span>
                    </div>
                  </div>
                )}

                {node.tcp_metrics && node.tcp_metrics.active_tcp_flows > 0 && (
                  <div className="mt-3 border-t border-border pt-3 space-y-2">
                    <div className="text-xs font-medium text-muted-foreground">{t.nodes.tcpHealth}</div>
                    <div className="grid grid-cols-4 gap-2 text-center">
                      <div className="rounded bg-muted/40 p-2">
                        <div className="text-lg font-bold tabular-nums">{node.tcp_metrics.active_tcp_flows}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.flows_}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-2">
                        <div className="text-lg font-bold tabular-nums text-orange-500">{node.tcp_metrics.total_retransmits}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.retx}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-2">
                        <div className="text-lg font-bold tabular-nums text-red-500">{node.tcp_metrics.total_rsts}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.rst}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-2">
                        <div className="text-lg font-bold tabular-nums">{node.tcp_metrics.rtt_avg_ms > 0 ? node.tcp_metrics.rtt_avg_ms.toFixed(1) + 'ms' : t.nodes.na}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.rtt}</div>
                      </div>
                    </div>
                    {node.tcp_metrics.packet_loss_pct > 0 && (
                      <div className="flex items-center gap-2 text-xs">
                        <span className="text-muted-foreground">{t.nodes.packetLoss}</span>
                        <span className="text-red-500 font-medium">{node.tcp_metrics.packet_loss_pct.toFixed(1)}%</span>
                        <span className="text-muted-foreground">({node.tcp_metrics.total_lost_pkts}/{node.tcp_metrics.total_expected_pkts} {t.nodes.pkts})</span>
                      </div>
                    )}
                    {node.tcp_metrics.total_zero_windows > 0 && (
                      <div className="flex gap-4 text-[10px] text-muted-foreground">
                        <span>{t.nodes.zeroWindow} {node.tcp_metrics.total_zero_windows}</span>
                        <span>{t.nodes.outOfOrder} {node.tcp_metrics.total_out_of_order}</span>
                      </div>
                    )}
                  </div>
                )}

                {(node.dns_latency || node.tls_latency || node.tcp_latency) && (
                  <div className="mt-3 border-t border-border pt-3 space-y-2">
                    <div className="text-xs font-medium text-muted-foreground">{t.nodes.appLatency}</div>
                    <div className="grid grid-cols-3 gap-2 text-center">
                      {node.dns_latency && node.dns_latency.samples > 0 && (
                        <div className="rounded bg-muted/40 p-1.5">
                          <div className="text-xs font-bold tabular-nums">{node.dns_latency.avg_ms.toFixed(1)}ms</div>
                          <div className="text-[10px] text-muted-foreground">{t.nodes.dnsLatency}</div>
                        </div>
                      )}
                      {node.tls_latency && node.tls_latency.samples > 0 && (
                        <div className="rounded bg-muted/40 p-1.5">
                          <div className="text-xs font-bold tabular-nums">{node.tls_latency.avg_ms.toFixed(1)}ms</div>
                          <div className="text-[10px] text-muted-foreground">{t.nodes.tlsLatency}</div>
                        </div>
                      )}
                      {node.tcp_latency && node.tcp_latency.samples > 0 && (
                        <div className="rounded bg-muted/40 p-1.5">
                          <div className="text-xs font-bold tabular-nums">{node.tcp_latency.avg_ms.toFixed(1)}ms</div>
                          <div className="text-[10px] text-muted-foreground">{t.nodes.tcpRtt}</div>
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {node.voip_stats && node.voip_stats.active_sessions > 0 && (
                  <div className="mt-3 border-t border-border pt-3 space-y-2">
                    <div className="text-xs font-medium text-muted-foreground">{t.nodes.voipQuality}</div>
                    <div className="grid grid-cols-4 gap-2 text-center">
                      <div className="rounded bg-muted/40 p-1.5">
                        <div className="text-xs font-bold tabular-nums">{node.voip_stats.active_sessions}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.voipSessions}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-1.5">
                        <div className="text-xs font-bold tabular-nums">{node.voip_stats.avg_jitter_ms.toFixed(1)}ms</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.voipJitter}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-1.5">
                        <div className="text-xs font-bold tabular-nums text-green-500">{node.voip_stats.avg_mos.toFixed(1)}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.voipMos}</div>
                      </div>
                      <div className="rounded bg-muted/40 p-1.5">
                        <div className="text-xs font-bold tabular-nums text-yellow-500">{node.voip_stats.total_lost}</div>
                        <div className="text-[10px] text-muted-foreground">{t.nodes.voipLost}</div>
                      </div>
                    </div>
                  </div>
                )}

                <div className="mt-4 flex flex-wrap gap-2 text-xs text-muted-foreground">
                  {node.tags?.map((tag) => (
                    <Badge key={tag} variant="outline" className="text-xs">
                      {tag}
                    </Badge>
                  ))}
                  <span className="ml-auto">
                    v{node.version} · {t.nodes.lastSeen}: {node.last_seen ? new Date(node.last_seen).toLocaleTimeString() : t.nodes.na}
                  </span>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
