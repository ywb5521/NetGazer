import { useState, useEffect, useCallback } from 'react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { fetchConfig, updateConfig, type ServerConfig } from '@/lib/api';
import { useI18n } from '@/i18n/I18nContext';
import { toast } from 'sonner';
import { Filter, Save, RotateCcw } from 'lucide-react';

export function BpfFilterEditor() {
  const { t } = useI18n();
  const [bpfFilter, setBpfFilter] = useState('');
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    fetchConfig().then((cfg: ServerConfig) => {
      setBpfFilter(cfg.bpf_filter || '');
      setLoaded(true);
    }).catch(() => {});
  }, []);

  const handleSave = useCallback(async () => {
    try {
      await updateConfig({ bpf_filter: bpfFilter });
      toast.success(t.settings.bpfFilterSaved);
    } catch (e) {
      toast.error(t.settings.bpfFilterSaveFailed);
    }
  }, [bpfFilter]);

  const handleReset = useCallback(async () => {
    try {
      await updateConfig({ bpf_filter: '' });
      setBpfFilter('');
      toast.success(t.settings.bpfFilterCleared);
    } catch (e) {
      toast.error(t.settings.bpfFilterClearFailed);
    }
  }, []);

  if (!loaded) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm flex items-center gap-2">
          <Filter className="h-4 w-4" />
          {t.settings.bpfFilter}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          {t.settings.bpfFilterDesc}
        </p>
        <div className="flex gap-2">
          <Input
            value={bpfFilter}
            onChange={(e) => setBpfFilter(e.target.value)}
            placeholder={t.settings.bpfFilterPlaceholder}
            className="h-8 text-xs font-mono"
          />
          <Button variant="outline" size="sm" className="h-8 shrink-0" onClick={handleSave}>
            <Save className="mr-1 h-3 w-3" /> {t.settings.save}
          </Button>
          <Button variant="ghost" size="sm" className="h-8 shrink-0" onClick={handleReset}>
            <RotateCcw className="mr-1 h-3 w-3" /> {t.settings.clear}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
