"use client";

import { createContext, useContext, useState, useCallback, useEffect, useRef, useMemo, type ReactNode } from "react";
import { en } from "./en";
import { zh } from "./zh";

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

const translations: Record<Locale, Record<string, string>> = { en, zh };

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>("en");
  const localeRef = useRef<Locale>("en");

  useEffect(() => {
    const saved = localStorage.getItem("forgec2_lang") as Locale;
    if (saved && saved !== "en") {
      localeRef.current = saved;
      setLocaleState(saved);
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
    localeRef.current = newLocale;
    setLocaleState(newLocale);
    localStorage.setItem("forgec2_lang", newLocale);
    document.cookie = "forgec2_lang=" + newLocale + "; path=/; max-age=31536000; SameSite=Strict";
    document.documentElement.lang = newLocale;
    document.documentElement.dir = "ltr";
  }, []);

  const t = useCallback((key: string, params?: Record<string, string | number>): string => {
    const l = localeRef.current;
    let val = translations[l]?.[key] || translations.en[key] || key;
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
