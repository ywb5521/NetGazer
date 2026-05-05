import { useEffect, useState, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { useI18n } from '@/i18n/I18nContext';
import { formatBytes } from '@/lib/utils';
import {
  fetchHostPools, createHostPool, updateHostPool, deleteHostPool, fetchHostPoolStats,
  type HostPool, type HostPoolStats,
} from '@/lib/api';
import HostPoolEditor from '@/components/settings/HostPoolEditor';
import { Layers, Plus, Trash2, Edit3, ChevronDown, ChevronRight } from 'lucide-react';

export default function HostPoolsPage() {
  const { t } = useI18n();
  const [pools, setPools] = useState<HostPool[]>([]);
  const [loading, setLoading] = useState(true);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<HostPool | null>(null);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [statsCache, setStatsCache] = useState<Record<string, HostPoolStats>>({});

  const reload = useCallback(async () => {
    try {
      const result = await fetchHostPools();
      setPools(Array.isArray(result) ? result : []);
    } catch { setPools([]); }
    setLoading(false);
  }, []);

  useEffect(() => { reload(); }, [reload]);

  const toggleExpand = async (id: string) => {
    if (expanded[id]) {
      setExpanded((prev) => ({ ...prev, [id]: false }));
      return;
    }
    setExpanded((prev) => ({ ...prev, [id]: true }));
    try {
      const stats = await fetchHostPoolStats(id);
      setStatsCache((prev) => ({ ...prev, [id]: stats }));
    } catch { /* ignore */ }
  };

  const handleCreate = async (data: { name: string; description: string; cidrs: string[] }) => {
    await createHostPool(data);
    reload();
  };

  const handleUpdate = async (data: { name: string; description: string; cidrs: string[] }) => {
    if (!editing) return;
    await updateHostPool(editing.id, data);
    reload();
  };

  const handleDelete = async (id: string) => {
    if (!confirm(t.hostPools.deleteConfirm)) return;
    await deleteHostPool(id);
    reload();
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight flex items-center gap-2">
          <Layers className="h-6 w-6" />
          {t.hostPools.title} ({pools.length})
        </h1>
        <Button size="sm" onClick={() => { setEditing(null); setEditorOpen(true); }}>
          <Plus className="mr-1 h-4 w-4" /> {t.hostPools.newPool}
        </Button>
      </div>

      <p className="text-sm text-muted-foreground">{t.hostPools.description}</p>

      {pools.length === 0 && !loading ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <Layers className="mx-auto h-8 w-8 mb-3 opacity-40" />
            <p>{t.hostPools.noPools}</p>
            <Button variant="outline" size="sm" className="mt-3" onClick={() => { setEditing(null); setEditorOpen(true); }}>
              <Plus className="mr-1 h-3 w-3" /> {t.hostPools.newPool}
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {pools.map((pool) => (
            <Card key={pool.id}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <div
                    className="flex items-center gap-2 cursor-pointer select-none"
                    onClick={() => toggleExpand(pool.id)}
                  >
                    {expanded[pool.id] ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    <CardTitle className="text-sm">{pool.name}</CardTitle>
                  </div>
                  <div className="flex gap-1">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => { setEditing(pool); setEditorOpen(true); }}>
                      <Edit3 className="h-3 w-3" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => handleDelete(pool.id)}>
                      <Trash2 className="h-3 w-3 text-destructive" />
                    </Button>
                  </div>
                </div>
                <p className="text-xs text-muted-foreground">{pool.description}</p>
                <div className="flex flex-wrap gap-2 mt-1">
                  {pool.cidrs?.map((c) => (
                    <span key={c} className="inline-block rounded bg-muted px-2 py-0.5 text-xs font-mono">{c}</span>
                  ))}
                </div>
              </CardHeader>
              {expanded[pool.id] && statsCache[pool.id] && (
                <CardContent>
                  <div className="flex items-center gap-6 mb-3 text-sm">
                    <div>
                      <span className="text-muted-foreground">{t.hostPools.hosts}: </span>
                      <span className="font-semibold">{statsCache[pool.id].hosts_count}</span>
                    </div>
                    <div>
                      <span className="text-muted-foreground">{t.hostPools.totalTraffic}: </span>
                      <span className="font-semibold">{formatBytes(statsCache[pool.id].total_bytes)}</span>
                    </div>
                  </div>
                  {statsCache[pool.id].hosts.length > 0 && (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>IP</TableHead>
                          <TableHead>{t.hosts.hostname}</TableHead>
                          <TableHead>{t.hosts.country}</TableHead>
                          <TableHead className="text-right">{t.hosts.bytesSent}</TableHead>
                          <TableHead className="text-right">{t.hosts.bytesReceived}</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {statsCache[pool.id].hosts.slice(0, 50).map((h) => (
                          <TableRow key={h.ip}>
                            <TableCell className="text-xs font-mono">{h.ip}</TableCell>
                            <TableCell className="text-xs">{h.hostname || '-'}</TableCell>
                            <TableCell className="text-xs">{h.country || '-'}</TableCell>
                            <TableCell className="text-xs text-right">{formatBytes(h.bytes_out)}</TableCell>
                            <TableCell className="text-xs text-right">{formatBytes(h.bytes_in)}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              )}
            </Card>
          ))}
        </div>
      )}

      <HostPoolEditor
        open={editorOpen}
        onClose={() => setEditorOpen(false)}
        onSave={editing ? handleUpdate : handleCreate}
        initial={editing ? { name: editing.name, description: editing.description, cidrs: editing.cidrs } : undefined}
      />
    </div>
  );
}
