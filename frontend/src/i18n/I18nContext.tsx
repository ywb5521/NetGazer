import { createContext, useContext, useState, useCallback, type ReactNode } from 'react';
import { zh } from './translations';
import { en } from './en';
import type { Translations } from './translations';

type Lang = 'zh' | 'en';

const translations: Record<Lang, Translations> = { zh, en };

const LANG_KEY = 'netgazer-lang';

function getInitialLang(): Lang {
  try {
    const stored = localStorage.getItem(LANG_KEY);
    if (stored === 'en' || stored === 'zh') return stored;
    const nav = navigator.language?.toLowerCase() || '';
    if (nav.startsWith('zh')) return 'zh';
    return 'zh';
  } catch {
    return 'zh';
  }
}

interface I18nState {
  lang: Lang;
  t: Translations;
  setLang: (l: Lang) => void;
  toggleLang: () => void;
}

const I18nContext = createContext<I18nState>({
  lang: 'zh',
  t: zh,
  setLang: () => {},
  toggleLang: () => {},
});

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(getInitialLang);

  const setLang = useCallback((l: Lang) => {
    setLangState(l);
    try { localStorage.setItem(LANG_KEY, l); } catch { /* ignore */ }
  }, []);

  const toggleLang = useCallback(() => {
    setLangState((prev) => {
      const next = prev === 'zh' ? 'en' : 'zh';
      try { localStorage.setItem(LANG_KEY, next); } catch { /* ignore */ }
      return next;
    });
  }, []);

  return (
    <I18nContext.Provider value={{ lang, t: translations[lang], setLang, toggleLang }}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}
