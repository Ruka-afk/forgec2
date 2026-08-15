"use client";

import { createContext, useContext, useState, useCallback, useEffect, ReactNode } from "react";
import { DEFAULT_THEME, resolveStoredTheme, type Theme } from "./theme-defaults";

export type { Theme };

interface ThemeContextType {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  resolved: "light" | "dark";
}

const ThemeContext = createContext<ThemeContextType>({
  theme: DEFAULT_THEME,
  setTheme: () => {},
  resolved: "dark",
});

function applyTheme(theme: Theme) {
  const dark =
    theme === "dark" ||
    (theme === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.classList.toggle("dark", dark);
  return dark ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(DEFAULT_THEME);
  const [resolved, setResolved] = useState<"light" | "dark">("dark");

  useEffect(() => {
    const initial = resolveStoredTheme(localStorage.getItem("forgec2_theme"));
    Promise.resolve().then(() => {
      setThemeState(initial);
      setResolved(applyTheme(initial));
    });

    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (resolveStoredTheme(localStorage.getItem("forgec2_theme")) === "system") {
        setResolved(applyTheme("system"));
      }
    };
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, []);

  const setTheme = useCallback((next: Theme) => {
    setThemeState(next);
    localStorage.setItem("forgec2_theme", next);
    setResolved(applyTheme(next));
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolved }}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  return useContext(ThemeContext);
}