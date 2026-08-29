"use client";

import { createContext, useContext, useState, useCallback, useEffect, useRef, useMemo, type ReactNode } from "react";
import { en } from "./en";

type Locale = "en" | "zh";

interface I18nContextType {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: string, params?: Record<string, string | number>) => string;
  dir: "ltr" | "rtl";
}

const I18nContext = createContext<I18nContextType>({
  locale: "en",
  setLocale: () => {},
  t: (key: string) => key,
  dir: "ltr",
});

type Dict = Record<string, string>;

// english dict ships with the main bundle; the zh dict is fetched on demand
// so en-only deployments never parse the ~150KB zh strings.
const enDict: Dict = en;
const zhRef: { current: Dict | null } = { current: null };
let zhPromise: Promise<Dict> | null = null;

// G4 fix: on chunk load failure, reset the cached rejected promise so the
// user can retry by switching language again (rather than being stuck).
function loadZh(): Promise<Dict> {
  if (zhRef.current) return Promise.resolve(zhRef.current);
  if (!zhPromise) {
    zhPromise = import("./zh").then((m) => {
      zhRef.current = m.zh;
      return m.zh;
    }).catch((err) => {
      zhPromise = null;
      throw err;
    });
  }
  return zhPromise;
}

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");
  const localeRef = useRef<Locale>("en");
  const pendingRef = useRef<Locale | null>(null);

  useEffect(() => {
    const saved = localStorage.getItem("forgec2_lang") as Locale;
    if (saved && saved !== "en") {
      pendingRef.current = "zh";
      loadZh().then(() => {
        if (pendingRef.current !== "zh") return;
        localeRef.current = "zh";
        setLocaleState("zh");
      });
    } else if (saved === "en") {
      localeRef.current = "en";
      setLocaleState("en");
    }
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const setLocale = useCallback((newLocale: Locale) => {
    if (newLocale === localeRef.current) return;
    pendingRef.current = newLocale;
    if (newLocale === "zh") {
      loadZh().then(() => {
        if (pendingRef.current !== "zh") return;
        localeRef.current = "zh";
        setLocaleState("zh");
      });
    } else {
      localeRef.current = "en";
      setLocaleState("en");
    }
    localStorage.setItem("forgec2_lang", newLocale);
    document.cookie = "forgec2_lang=" + newLocale + "; path=/; max-age=31536000; SameSite=Strict";
    document.documentElement.lang = newLocale;
    document.documentElement.dir = "ltr";
  }, []);

  // G2 fix: include locale in deps so `t` identity changes on language switch,
  // causing memoized children (Sidebar, CommandPalette items, document.title
  // effect) to re-render correctly.
  const t = useCallback((key: string, params?: Record<string, string | number>): string => {
    const l = localeRef.current;
    const dict = l === "zh" && zhRef.current ? zhRef.current : enDict;
    let val = dict[key] || enDict[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        val = val.replace(new RegExp(`\\{${k}\\}`, "g"), String(v));
      });
    }
    return val;
  // eslint-disable-next-line react-hooks/exhaustive-deps -- locale is read via localeRef; the dep is intentional (G2): it rotates t's identity on language switch so memoized children re-render.
  }, [locale]);
  const dir: "ltr" | "rtl" = "ltr";
  const value = useMemo(() => ({ locale, setLocale, t, dir }), [locale, setLocale, t, dir]);

  return (
    <I18nContext.Provider value={value}>
      {children}
    </I18nContext.Provider>
  );
}

export function useI18n() {
  return useContext(I18nContext);
}