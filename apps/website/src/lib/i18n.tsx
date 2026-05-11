/**
 * Minimal i18n for the public website.
 *
 * Keys are grouped by page / area.  Every visible string on the site should
 * have a key here so the language toggle works everywhere.
 */
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from "react";

export type Locale = "zh" | "en";

const STORAGE_KEY = "cialloclaw-website-locale";

function readStoredLocale(): Locale {
  if (typeof window === "undefined") {
    return "zh";
  }
  const stored = window.localStorage.getItem(STORAGE_KEY);
  if (stored === "en" || stored === "zh") {
    return stored;
  }
  return "zh";
}

// ── Dictionary ────────────────────────────────────────────────────────────

const dictionaries: Record<Locale, Record<string, string>> = {
  zh: {
    /* nav */
    "nav.docs": "文档",
    /* nav - mobile menu toggle */
    "nav.dark": "黑夜",
    "nav.light": "白天",
    /* nav - language */
    "nav.lang.zh": "中文",
    "nav.lang.en": "English",
    /* nav - search */
    "nav.search.placeholder": "搜索",
    /* home */
    "home.gallery.desktop-collab": "桌面协作",
    "home.gallery.voice-input": "语音承接",
    "home.gallery.doc-entry": "文档入口",
    "home.gallery.formal-delivery": "正式交付",
    "home.subtitle": "桌面悬浮球 Agent，你的专属桌宠助理",
    "home.action.download": "下载",
    "home.action.tutorial": "使用教程",
    /* docs */
    "docs.page-title": "文档导航",
    "docs.outline-title": "本页大纲",
    "docs.drawer-title": "文档导航",
    "docs.menu-button": "文档导航",
    "docs.eyebrow": "Docs",
  },

  en: {
    /* nav */
    "nav.docs": "Docs",
    /* nav - mobile menu toggle */
    "nav.dark": "Dark",
    "nav.light": "Light",
    /* nav - language */
    "nav.lang.zh": "中文",
    "nav.lang.en": "English",
    /* nav - search */
    "nav.search.placeholder": "Search",
    /* home */
    "home.gallery.desktop-collab": "Desktop Collaboration",
    "home.gallery.voice-input": "Voice Input",
    "home.gallery.doc-entry": "Document Entry",
    "home.gallery.formal-delivery": "Formal Delivery",
    "home.subtitle": "Desktop Companion Agent, Your Personal Mascot Assistant",
    "home.action.download": "Download",
    "home.action.tutorial": "Tutorial",
    /* docs */
    "docs.page-title": "Documentation",
    "docs.outline-title": "On This Page",
    "docs.drawer-title": "Documentation",
    "docs.menu-button": "Documentation",
    "docs.eyebrow": "Docs",
  },
};

// ── Context ───────────────────────────────────────────────────────────────

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string) => string;
}

const I18nContext = createContext<I18nContextValue>({
  locale: "zh",
  setLocale: () => {},
  t: (key: string) => key,
});

export function useI18n(): I18nContextValue {
  return useContext(I18nContext);
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(readStoredLocale);

  const setLocale = useCallback((next: Locale) => {
    setLocaleState(next);
    window.localStorage.setItem(STORAGE_KEY, next);
    document.documentElement.lang = next === "en" ? "en" : "zh-CN";
  }, []);

  const t = useCallback(
    (key: string): string => {
      return dictionaries[locale]?.[key] ?? dictionaries.zh[key] ?? key;
    },
    [locale],
  );

  const value = useMemo(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}
