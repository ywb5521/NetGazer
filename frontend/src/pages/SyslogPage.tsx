import { useState, useEffect } from 'react';
import { fetchSyslog } from '@/lib/api';
import type { SyslogRecord } from '@/types';
import { useI18n } from '@/i18n/I18nContext';

const SEV_COLORS: Record<string, string> = {
  emerg: 'bg-red-800 text-red-100',
  alert: 'bg-red-700 text-red-100',
  crit: 'bg-red-600 text-red-100',
  err: 'bg-orange-600 text-orange-100',
  warning: 'bg-yellow-600 text-yellow-100',
  notice: 'bg-blue-600 text-blue-100',
  info: 'bg-green-600 text-green-100',
  debug: 'bg-gray-500 text-gray-100',
};

const SEV_FILTERS = ['', 'emerg', 'alert', 'crit', 'err', 'warning', 'notice', 'info', 'debug'];

export default function SyslogPage() {
  const { t } = useI18n();
  const [records, setRecords] = useState<SyslogRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [severity, setSeverity] = useState('');
  const [source, setSource] = useState('');
  const [loading, setLoading] = useState(false);
  const limit = 50;

  const load = async (newOffset: number) => {
    setLoading(true);
    try {
      const res = await fetchSyslog(severity || undefined, source || undefined, limit, newOffset);
      setRecords(res?.items || []);
      setTotal(res?.total || 0);
      setOffset(newOffset);
    } catch {
      // silently fail
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(0); }, [severity, source]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">{t.syslog.title}</h1>
        <div className="flex items-center gap-3">
          <select
            value={severity}
            onChange={(e) => setSeverity(e.target.value)}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm"
          >
            <option value="">{t.syslog.allSeverities}</option>
            {SEV_FILTERS.slice(1).map((s) => (
              <option key={s} value={s}>{s.toUpperCase()}</option>
            ))}
          </select>
          <input
            type="text"
            placeholder={t.syslog.filterSource}
            value={source}
            onChange={(e) => setSource(e.target.value)}
            className="h-9 rounded-md border border-input bg-background px-3 text-sm w-48"
          />
          <span className="text-sm text-muted-foreground">{total + t.syslog.records}</span>
        </div>
      </div>

      <div className="rounded-md border">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b bg-muted/50">
              <th className="px-3 py-2 text-left font-medium">{t.syslog.time}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.severity}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.facility}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.hostname}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.appName}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.message}</th>
              <th className="px-3 py-2 text-left font-medium">{t.syslog.source}</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">{t.syslog.loading}</td></tr>
            ) : records.length === 0 ? (
              <tr><td colSpan={7} className="px-3 py-8 text-center text-muted-foreground">{t.syslog.noMessages}</td></tr>
            ) : (
              records.map((r) => (
                <tr key={r.id} className="border-b hover:bg-muted/30">
                  <td className="px-3 py-2 whitespace-nowrap text-muted-foreground">
                    {new Date(r.timestamp).toLocaleString()}
                  </td>
                  <td className="px-3 py-2">
                    <span className={`inline-block rounded px-1.5 py-0.5 text-xs font-medium ${SEV_COLORS[r.severity] || 'bg-gray-500 text-gray-100'}`}>
                      {r.severity}
                    </span>
                  </td>
                  <td className="px-3 py-2 text-muted-foreground">{r.facility}</td>
                  <td className="px-3 py-2">{r.hostname}</td>
                  <td className="px-3 py-2 text-muted-foreground">{r.app_name}</td>
                  <td className="px-3 py-2 max-w-md truncate">{r.message}</td>
                  <td className="px-3 py-2 text-muted-foreground text-xs">{r.source}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between">
        <span className="text-sm text-muted-foreground">
          {offset + 1}-{Math.min(offset + limit, total)} {t.syslog.of} {total}
        </span>
        <div className="flex gap-2">
          <button
            onClick={() => load(Math.max(0, offset - limit))}
            disabled={offset === 0 || loading}
            className="h-8 rounded-md border px-3 text-sm hover:bg-muted disabled:opacity-50"
          >
            {t.syslog.prev}
          </button>
          <button
            onClick={() => load(offset + limit)}
            disabled={offset + limit >= total || loading}
            className="h-8 rounded-md border px-3 text-sm hover:bg-muted disabled:opacity-50"
          >
            {t.syslog.next}
          </button>
        </div>
      </div>
    </div>
  );
}
