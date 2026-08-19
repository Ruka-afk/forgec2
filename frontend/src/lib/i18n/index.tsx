"use client";

import { createContext, useContext, useState, useCallback, useEffect, useRef, useMemo, type ReactNode } from "react";
import { en } from "./en";

export type Locale = "en" | "zh";

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

function loadZh(): Promise<Dict> {
  if (zhRef.current) return Promise.resolve(zhRef.current);
  zhPromise ??= import("./zh").then((m) => {
    zhRef.current = m.zh;
    return m.zh;
  });
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
  }, []);
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