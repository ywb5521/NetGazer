import { useState, useEffect } from 'react';
import { fetchLuaScripts, createLuaScript, deleteLuaScript, testLuaScript } from '@/lib/api';
import type { LuaScript } from '@/types';
import { useI18n } from '@/i18n/I18nContext';

export function LuaEditor() {
  const { t } = useI18n();
  const [scripts, setScripts] = useState<LuaScript[]>([]);
  const [selected, setSelected] = useState<LuaScript | null>(null);
  const [name, setName] = useState('');
  const [content, setContent] = useState('');
  const [enabled, setEnabled] = useState(true);
  const [testResult, setTestResult] = useState('');
  const [saving, setSaving] = useState(false);
  const [nodeId, setNodeId] = useState('');

  const load = async () => {
    try {
      const res = await fetchLuaScripts();
      setScripts(res.items);
    } catch { /* ignore */ }
  };

  useEffect(() => { load(); }, []);

  const handleSelect = (s: LuaScript) => {
    setSelected(s);
    setName(s.name);
    setContent(s.content);
    setEnabled(s.enabled);
    setTestResult('');
  };

  const handleNew = () => {
    setSelected(null);
    setName('');
    setContent('');
    setEnabled(true);
    setTestResult('');
  };

  const handleSave = async () => {
    if (!name.trim()) return;
    setSaving(true);
    try {
      await createLuaScript({ name: name.trim(), content, enabled });
      await load();
      setSaving(false);
    } catch (e: any) {
      setTestResult(`${t.lua.testError}: ${e.message}`);
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!selected) return;
    try {
      await deleteLuaScript(selected.name);
      setSelected(null);
      setName('');
      setContent('');
      setEnabled(true);
      await load();
    } catch (e: any) {
      setTestResult(`${t.lua.testError}: ${e.message}`);
    }
  };

  const handleTest = async () => {
    try {
      const res = await testLuaScript(content, nodeId);
      setTestResult(res.error ? `${t.lua.testError}: ${res.error}` : t.lua.testOk);
    } catch (e: any) {
      setTestResult(`${t.lua.testError}: ${e.message}`);
    }
  };

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-lg font-semibold">{t.lua.title}</h3>
        <p className="text-sm text-muted-foreground mt-1">{t.lua.description}</p>
      </div>

      <div className="flex gap-4">
        <div className="w-48 space-y-2">
          <button
            onClick={handleNew}
            className="w-full rounded-md border px-3 py-2 text-sm hover:bg-muted"
          >
            {t.lua.newScript}
          </button>
          <div className="space-y-1">
            {scripts.map((s) => (
              <button
                key={s.name}
                onClick={() => handleSelect(s)}
                className={`w-full rounded-md px-3 py-2 text-sm text-left flex items-center justify-between ${
                  selected?.name === s.name ? 'bg-accent text-accent-foreground' : 'hover:bg-muted'
                }`}
              >
                <span className="truncate">{s.name}</span>
                <span className={`inline-block h-1.5 w-1.5 rounded-full ${s.enabled ? 'bg-green-500' : 'bg-gray-400'}`} />
              </button>
            ))}
            {scripts.length === 0 && (
              <p className="text-xs text-muted-foreground px-3">{t.lua.noScripts}</p>
            )}
          </div>
        </div>

        <div className="flex-1 space-y-3">
          <div className="flex items-center gap-3">
            <input
              type="text"
              placeholder={t.lua.scriptName}
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="h-9 rounded-md border border-input bg-background px-3 text-sm flex-1"
            />
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="rounded"
              />
              {t.lua.enabled}
            </label>
          </div>
          <textarea
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder={t.lua.placeholder}
            className="h-80 w-full rounded-md border border-input bg-background px-3 py-2 text-sm font-mono"
            spellCheck={false}
          />
          <div className="flex items-center gap-2">
            <button
              onClick={handleSave}
              disabled={saving || !name.trim()}
              className="rounded-md bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
            >
              {saving ? t.lua.saving : t.lua.save}
            </button>
            <button
              onClick={handleTest}
              className="rounded-md border px-4 py-2 text-sm hover:bg-muted"
            >
              {t.lua.test}
            </button>
            {selected && (
              <button
                onClick={handleDelete}
                className="rounded-md border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
              >
                {t.lua.delete}
              </button>
            )}
            <input
              type="text"
              placeholder={t.lua.nodeId}
              value={nodeId}
              onChange={(e) => setNodeId(e.target.value)}
              className="h-9 rounded-md border border-input bg-background px-3 text-sm w-40"
            />
          </div>
          {testResult && (
            <div className={`rounded-md p-2 text-sm ${testResult.startsWith('OK') ? 'bg-green-50 text-green-700' : 'bg-red-50 text-red-700'}`}>
              {testResult}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
