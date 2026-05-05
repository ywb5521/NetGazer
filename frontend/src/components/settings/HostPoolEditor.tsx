import { useState } from 'react';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Plus, Trash2 } from 'lucide-react';
import { useI18n } from '@/i18n/I18nContext';

interface HostPoolEditorProps {
  open: boolean;
  onClose: () => void;
  onSave: (data: { name: string; description: string; cidrs: string[] }) => void;
  initial?: { name: string; description: string; cidrs: string[] };
}

export default function HostPoolEditor({ open, onClose, onSave, initial }: HostPoolEditorProps) {
  const { t } = useI18n();
  const [name, setName] = useState(initial?.name || '');
  const [description, setDescription] = useState(initial?.description || '');
  const [cidrs, setCidrs] = useState<string[]>(initial?.cidrs || ['']);

  const handleSave = () => {
    if (!name.trim()) return;
    const filtered = cidrs.filter((c) => c.trim());
    onSave({ name: name.trim(), description: description.trim(), cidrs: filtered.length > 0 ? filtered : [] });
    setName('');
    setDescription('');
    setCidrs(['']);
    onClose();
  };

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{initial ? t.hostPools.editPool : t.hostPools.newPool}</DialogTitle>
          <DialogDescription>{t.hostPools.description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label htmlFor="pool-name">{t.hostPools.name}</Label>
            <Input
              id="pool-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t.hostPools.name}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="pool-desc">{t.hostPools.description}</Label>
            <Input
              id="pool-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t.hostPools.name}
            />
          </div>
          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label>{t.hostPools.cidrs}</Label>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="h-6 text-xs"
                onClick={() => setCidrs([...cidrs, ''])}
              >
                <Plus className="mr-1 h-3 w-3" />
                {t.hostPools.addCidr}
              </Button>
            </div>
            <div className="space-y-2">
              {cidrs.map((c, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Input
                    value={c}
                    onChange={(e) => {
                      const updated = [...cidrs];
                      updated[i] = e.target.value;
                      setCidrs(updated);
                    }}
                    placeholder={t.hostPools.cidrsPlaceholder}
                    className="h-8 text-xs"
                  />
                  {cidrs.length > 1 && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7 shrink-0"
                      onClick={() => setCidrs(cidrs.filter((_, j) => j !== i))}
                    >
                      <Trash2 className="h-3 w-3 text-destructive" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>{t.hostPools.cancel}</Button>
          <Button onClick={handleSave} disabled={!name.trim()}>{t.hostPools.save}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
