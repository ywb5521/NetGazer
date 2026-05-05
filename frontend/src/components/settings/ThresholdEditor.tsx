import { useState, useEffect } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { fetchConfig, updateConfig, type ServerConfig } from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import type { AlertThresholds } from '@/types';
import { Save, RefreshCw } from 'lucide-react';

function parseCommaSep(v: string): number[] {
  return v
    .split(',')
    .map((s) => parseInt(s.trim(), 10))
    .filter((n) => !isNaN(n) && n > 0);
}

const defaults: AlertThresholds = {
  banned_ports: [23, 3389, 445, 135, 139],
  port_scan_threshold: 20,
  port_scan_window_sec: 60,
  flow_flood_threshold: 100,
  alert_cooldown_min: 5,
  dns_suspicious_ports: null,
};

export function ThresholdEditor() {
  const { t } = useI18n();
  const [config, setConfig] = useState<ServerConfig | null>(null);
  const [thresholdMbps, setThresholdMbps] = useState('');
  const [bannedPorts, setBannedPorts] = useState('');
  const [portScanThreshold, setPortScanThreshold] = useState('');
  const [portScanWindow, setPortScanWindow] = useState('');
  const [flowFloodThreshold, setFlowFloodThreshold] = useState('');
  const [alertCooldown, setAlertCooldown] = useState('');
  const [dnsSuspiciousPorts, setDnsSuspiciousPorts] = useState('');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    fetchConfig()
      .then((c) => {
        setConfig(c);
        setThresholdMbps(String(Math.round(c.bandwidth_threshold_bps / 1_000_000)));
        const t = c.alert_thresholds || defaults;
        setBannedPorts((t.banned_ports || []).join(', '));
        setPortScanThreshold(String(t.port_scan_threshold));
        setPortScanWindow(String(t.port_scan_window_sec));
        setFlowFloodThreshold(String(t.flow_flood_threshold));
        setAlertCooldown(String(t.alert_cooldown_min));
        setDnsSuspiciousPorts((t.dns_suspicious_ports || []).join(', '));
      })
      .catch(() => {});
  }, []);

  const handleSave = async () => {
    const bps = parseFloat(thresholdMbps) * 1_000_000;
    if (isNaN(bps) || bps <= 0) return;

    const thresholds: AlertThresholds = {
      banned_ports: parseCommaSep(bannedPorts),
      port_scan_threshold: parseInt(portScanThreshold, 10) || defaults.port_scan_threshold,
      port_scan_window_sec: parseInt(portScanWindow, 10) || defaults.port_scan_window_sec,
      flow_flood_threshold: parseInt(flowFloodThreshold, 10) || defaults.flow_flood_threshold,
      alert_cooldown_min: parseInt(alertCooldown, 10) || defaults.alert_cooldown_min,
      dns_suspicious_ports: dnsSuspiciousPorts.trim()
        ? parseCommaSep(dnsSuspiciousPorts)
        : null,
    };

    setSaving(true);
    try {
      await updateConfig({ bandwidth_threshold_bps: bps, alert_thresholds: thresholds } as any);
      setConfig((prev) => (prev ? { ...prev, bandwidth_threshold_bps: bps, alert_thresholds: thresholds } : prev));
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      // ignore
    }
    setSaving(false);
  };

  const handleReset = () => {
    setBannedPorts(defaults.banned_ports.join(', '));
    setPortScanThreshold(String(defaults.port_scan_threshold));
    setPortScanWindow(String(defaults.port_scan_window_sec));
    setFlowFloodThreshold(String(defaults.flow_flood_threshold));
    setAlertCooldown(String(defaults.alert_cooldown_min));
    setDnsSuspiciousPorts('');
  };

  const inputCls = 'w-24 h-8 text-xs';
  const labelCls = 'text-xs font-medium text-muted-foreground';
  const hintCls = 'text-[10px] text-muted-foreground';

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm">{t.settings.alertThresholds}</CardTitle>
        <CardDescription className="text-xs">
          {t.settings.thresholdDesc}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Bandwidth */}
        <div className="flex items-end gap-3">
          <div className="space-y-1">
            <label className={labelCls}>{t.settings.bandwidthThreshold}</label>
            <Input type="number" value={thresholdMbps} onChange={(e) => setThresholdMbps(e.target.value)} className={inputCls} min={1} />
          </div>
          {config && (
            <span className={hintCls}>
              {t.settings.current}: {Math.round(config.bandwidth_threshold_bps / 1_000_000).toLocaleString()} Mbps
            </span>
          )}
        </div>

        <div className="border-t border-border pt-3 grid grid-cols-2 gap-3">
          {/* Banned Ports */}
          <div className="space-y-1 col-span-2">
            <label className={labelCls}>{t.settings.bannedPorts}</label>
            <Input value={bannedPorts} onChange={(e) => setBannedPorts(e.target.value)} className="w-full h-8 text-xs font-mono" placeholder={t.settings.bannedPortsPlaceholder} />
            <p className={hintCls}>{t.settings.bannedPortsDesc}</p>
          </div>

          {/* Port Scan Threshold */}
          <div className="space-y-1">
            <label className={labelCls}>{t.settings.portScanThreshold}</label>
            <Input type="number" value={portScanThreshold} onChange={(e) => setPortScanThreshold(e.target.value)} className={inputCls} min={1} max={1000} />
            <p className={hintCls}>{t.settings.portScanThresholdDesc}</p>
          </div>

          {/* Port Scan Window */}
          <div className="space-y-1">
            <label className={labelCls}>{t.settings.portScanWindow}</label>
            <Input type="number" value={portScanWindow} onChange={(e) => setPortScanWindow(e.target.value)} className={inputCls} min={10} max={600} />
            <p className={hintCls}>{t.settings.portScanWindowDesc}</p>
          </div>

          {/* Flow Flood Threshold */}
          <div className="space-y-1">
            <label className={labelCls}>{t.settings.flowFloodThreshold}</label>
            <Input type="number" value={flowFloodThreshold} onChange={(e) => setFlowFloodThreshold(e.target.value)} className={inputCls} min={1} max={10000} />
            <p className={hintCls}>{t.settings.flowFloodThresholdDesc}</p>
          </div>

          {/* Alert Cooldown */}
          <div className="space-y-1">
            <label className={labelCls}>{t.settings.alertCooldown}</label>
            <Input type="number" value={alertCooldown} onChange={(e) => setAlertCooldown(e.target.value)} className={inputCls} min={1} max={1440} />
            <p className={hintCls}>{t.settings.alertCooldownDesc}</p>
          </div>

          {/* DNS Suspicious Ports */}
          <div className="space-y-1 col-span-2">
            <label className={labelCls}>{t.settings.dnsSuspiciousPorts}</label>
            <Input value={dnsSuspiciousPorts} onChange={(e) => setDnsSuspiciousPorts(e.target.value)} className="w-full h-8 text-xs font-mono" placeholder={t.settings.dnsSuspiciousPortsPlaceholder} />
            <p className={hintCls}>{t.settings.dnsSuspiciousPortsDesc}</p>
          </div>
        </div>

        <div className="flex gap-2 pt-2 border-t border-border">
          <Button onClick={handleSave} disabled={saving} size="sm">
            <Save className="mr-1 h-3 w-3" />
            {saved ? t.settings.saved : t.settings.save}
          </Button>
          <Button variant="outline" size="sm" onClick={handleReset}>
            <RefreshCw className="mr-1 h-3 w-3" />
            {t.settings.resetToDefaults}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
