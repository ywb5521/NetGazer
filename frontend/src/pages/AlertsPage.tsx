import { useState, useMemo } from 'react';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useAppContext } from '@/context/AppContext';
import { useI18n } from '@/i18n/I18nContext';
import { useLocalStorage } from '@/lib/useLocalStorage';
import { exportCSV } from '@/lib/api';
import { Check, ChevronLeft, ChevronRight, Search, CheckCheck, ArrowUpDown, Download } from 'lucide-react';
import { Input } from '@/components/ui/input';

const severityVariant: Record<string, 'destructive' | 'secondary' | 'outline'> = {
  critical: 'destructive',
  warning: 'secondary',
  info: 'outline',
};

const alertTypeLabels: Record<string, { zh: string; en: string }> = {
  high_bandwidth: { zh: '高带宽占用', en: 'High bandwidth' },
  new_device: { zh: '新设备', en: 'New device' },
  suspicious_port: { zh: '可疑端口', en: 'Suspicious port' },
  flow_flood: { zh: '流量洪泛', en: 'Flow flood' },
  port_scan: { zh: '端口扫描', en: 'Port scan' },
  dns_suspicious_port: { zh: 'DNS 非标准端口', en: 'DNS suspicious port' },
  dns_exfiltration: { zh: 'DNS 外传', en: 'DNS exfiltration' },
  icmp_flood: { zh: 'ICMP 洪泛', en: 'ICMP flood' },
  syn_flood: { zh: 'SYN 洪泛', en: 'SYN flood' },
  horizontal_scan: { zh: '水平扫描', en: 'Horizontal scan' },
  data_exfiltration: { zh: '数据外传', en: 'Data exfiltration' },
  unexpected_protocol: { zh: '异常协议', en: 'Unexpected protocol' },
  arp_spoof: { zh: 'ARP 欺骗', en: 'ARP spoofing' },
  long_flow: { zh: '长连接', en: 'Long-running flow' },
  test: { zh: '测试通知', en: 'Test notification' },
};

const PAGE_SIZE = 20;

export default function AlertsPage() {
  const { snapshot, ackAlert, selectedNode } = useAppContext();
  const { t, lang } = useI18n();
  const [severityFilter, setSeverityFilter] = useLocalStorage('netgazer-alerts-severity', 'all');
  const [typeFilter, setTypeFilter] = useLocalStorage('netgazer-alerts-type', 'all');
  const [searchText, setSearchText] = useState('');
  const [sortNewest, setSortNewest] = useState(true);
  const [page, setPage] = useState(0);

  const labelAlertType = (value: string) => alertTypeLabels[value]?.[lang] || value.replace(/_/g, ' ');

  const alertTypes = useMemo(() => {
    if (!snapshot) return [];
    const types = new Set(snapshot.alerts.map((a) => a.type));
    return Array.from(types).sort();
  }, [snapshot]);

  const filtered = useMemo(() => {
    if (!snapshot) return [];
    let list = snapshot.alerts;
    if (selectedNode) list = list.filter((a) => a.node_id === selectedNode);
    if (severityFilter !== 'all') list = list.filter((a) => a.severity === severityFilter);
    if (typeFilter !== 'all') list = list.filter((a) => a.type === typeFilter);
    if (searchText) {
      const q = searchText.toLowerCase();
      list = list.filter((a) =>
        a.message.toLowerCase().includes(q) ||
        a.type.toLowerCase().includes(q) ||
        (a.source_ip && a.source_ip.includes(q)) ||
        a.node_id.toLowerCase().includes(q)
      );
    }
    list = [...list].sort((a, b) => sortNewest ? b.timestamp - a.timestamp : a.timestamp - b.timestamp);
    return list;
  }, [snapshot, selectedNode, severityFilter, typeFilter, searchText, sortNewest]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const paged = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  const severityLabel: Record<string, string> = {
    all: t.alerts.all,
    critical: t.alerts.critical,
    warning: t.alerts.warning,
    info: t.alerts.info,
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t.alerts.title} ({filtered.length})</h1>
        <Button variant="outline" size="sm" disabled={filtered.length === 0} onClick={() => {
          const headers = [t.alerts.exportTime, t.alerts.exportType, t.alerts.exportSeverity, t.alerts.exportMessage, t.alerts.exportSourceIp, t.alerts.exportNode, t.alerts.exportAcknowledged];
          const rows = filtered.map((a) => [
            new Date(a.timestamp).toISOString(), labelAlertType(a.type), severityLabel[a.severity] || a.severity, a.message,
            a.source_ip || '', a.node_id, String(a.acknowledged),
          ]);
          exportCSV(headers, rows, `alerts-${new Date().toISOString().slice(0, 10)}.csv`);
        }}>
          <Download className="mr-1 h-3 w-3" /> CSV
        </Button>
      </div>

      <div className="flex flex-wrap gap-3 items-center">
        <div className="relative w-56">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <Input
            placeholder={t.alerts.searchPlaceholder}
            value={searchText}
            onChange={(e) => { setSearchText(e.target.value); setPage(0); }}
            className="pl-7 h-8 text-xs"
          />
        </div>
        <Select value={severityFilter} onValueChange={(v) => { setSeverityFilter(v); setPage(0); }}>
          <SelectTrigger className="w-28 h-8 text-xs">
            <SelectValue placeholder={t.alerts.all} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t.alerts.all}</SelectItem>
            <SelectItem value="critical">{t.alerts.critical}</SelectItem>
            <SelectItem value="warning">{t.alerts.warning}</SelectItem>
            <SelectItem value="info">{t.alerts.info}</SelectItem>
          </SelectContent>
        </Select>

        <Select value={typeFilter} onValueChange={(v) => { setTypeFilter(v); setPage(0); }}>
          <SelectTrigger className="w-40 h-8 text-xs">
            <SelectValue placeholder={t.alerts.allTypes} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t.alerts.allTypes}</SelectItem>
            {alertTypes.map((at) => (
              <SelectItem key={at} value={at}>{labelAlertType(at)}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Button
          variant="ghost"
          size="sm"
          className="h-8 text-xs"
          onClick={() => setSortNewest(!sortNewest)}
        >
          <ArrowUpDown className="mr-1 h-3 w-3" />
          {sortNewest ? t.alerts.newest : t.alerts.oldest}
        </Button>

        {paged.some((a) => !a.acknowledged) && (
          <Button
            variant="outline"
            size="sm"
            className="h-8 text-xs"
            onClick={() => paged.filter((a) => !a.acknowledged).forEach((a) => ackAlert(a.id))}
          >
            <CheckCheck className="mr-1 h-3 w-3" />
            {t.alerts.ackPage}
          </Button>
        )}
      </div>

      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col gap-3">
            {paged.length > 0 ? (
              paged.map((alert) => (
                <div
                  key={alert.id}
                  className="flex items-start justify-between rounded-lg border border-border p-4"
                >
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <Badge variant={severityVariant[alert.severity] || 'outline'}>
                        {severityLabel[alert.severity] || alert.severity}
                      </Badge>
                      <Badge variant="outline" className="text-xs">
                        {labelAlertType(alert.type)}
                      </Badge>
                      {alert.acknowledged && (
                        <Badge variant="outline" className="text-muted-foreground text-xs">
                          {t.alerts.acked}
                        </Badge>
                      )}
                    </div>
                    <p className="text-sm">{alert.message}</p>
                    <div className="flex gap-3 mt-2">
                      {alert.source_ip && (
                        <span className="text-xs text-muted-foreground">
                          {t.flows.source}: {alert.source_ip}
                        </span>
                      )}
                      <span className="text-xs text-muted-foreground">
                        {t.flows.node}: {alert.node_id}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {new Date(alert.timestamp).toLocaleString()}
                      </span>
                    </div>
                  </div>
                  {!alert.acknowledged && (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => ackAlert(alert.id)}
                      className="shrink-0 ml-4"
                    >
                      <Check className="mr-1 h-3 w-3" />
                      {t.alerts.ack}
                    </Button>
                  )}
                </div>
              ))
            ) : (
              <p className="text-center text-muted-foreground py-8">{t.alerts.noAlerts}</p>
            )}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-between mt-6 pt-4 border-t border-border">
              <span className="text-xs text-muted-foreground">
                {t.alerts.page} {page + 1} {t.alerts.of} {totalPages} ({filtered.length} {t.alerts.title.toLowerCase()})
              </span>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={page === 0} onClick={() => setPage((p) => p - 1)}>
                  <ChevronLeft className="h-4 w-4" /> {t.alerts.prev}
                </Button>
                <Button variant="outline" size="sm" disabled={page >= totalPages - 1} onClick={() => setPage((p) => p + 1)}>
                  {t.alerts.next} <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
