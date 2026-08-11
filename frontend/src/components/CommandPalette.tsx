"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { NAV_SECTIONS } from "@/lib/navigation";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { Search, CornerDownLeft } from "lucide-react";

interface PaletteItem {
  href: string;
  label: string;
  section: string;
  icon: React.ComponentType<{ className?: string }>;
}

function normalize(s: string): string {
  return s.toLowerCase().replace(/[-_/]/g, " ");
}

export default function CommandPalette() {
  const { t } = useI18n();
  const router = useRouter();
  const open = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const items: PaletteItem[] = useMemo(() => {
    const out: PaletteItem[] = [];
    for (const section of NAV_SECTIONS) {
      for (const item of section.items) {
        out.push({
          href: item.href,
          label: t(item.labelKey),
          section: t("section." + section.titleKey),
          icon: item.icon,
        });
      }
    }
    return out;
  }, [t]);

  const filtered = useMemo(() => {
    const q = normalize(query.trim());
    if (!q) return items;
    return items.filter(
      (i) => normalize(i.label).includes(q) || normalize(i.href).includes(q) || normalize(i.section).includes(q),
    );
  }, [items, query]);

  const showSearchAction = query.trim().length > 0;

  useEffect(() => {
    if (open) {
      setQuery("");
      setActiveIndex(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const close = useCallback(() => setOpen(false), [setOpen]);

  const handleSelect = useCallback((index: number) => {
    const item = filtered[index];
    if (!item) return;
    close();
    router.push(item.href);
  }, [filtered, close, router]);

  const handleSubmitSearch = useCallback(() => {
    const q = query.trim();
    if (!q) return;
    close();
    router.push(`/search?q=${encodeURIComponent(q)}`);
  }, [query, close, router]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(!useAppStore.getState().commandPaletteOpen);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [setOpen]);

  useEffect(() => {
    if (!open) return;
    const onKeyDown = (e: KeyboardEvent) => {
      const total = filtered.length + (showSearchAction ? 1 : 0);
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setActiveIndex((i) => (i + 1) % total);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        setActiveIndex((i) => (i - 1 + total) % total);
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (showSearchAction && activeIndex === 0) handleSubmitSearch();
        else handleSelect(activeIndex - (showSearchAction ? 1 : 0));
      } else if (e.key === "Escape") {
        e.preventDefault();
        close();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, filtered, showSearchAction, activeIndex, handleSelect, handleSubmitSearch, close]);

  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  useEffect(() => {
    const el = listRef.current?.querySelector('[data-active="true"]');
    if (el && typeof el.scrollIntoView === "function") {
      el.scrollIntoView({ block: "nearest" });
    }
  }, [activeIndex]);

  if (!open) return null;

  const groups = new Map<string, PaletteItem[]>();
  for (const item of filtered) {
    const list = groups.get(item.section) || [];
    list.push(item);
    groups.set(item.section, list);
  }

  const searchOffset = showSearchAction ? 1 : 0;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[12vh] px-4"
      role="dialog"
      aria-modal="true"
      aria-label={t("palette.title")}
      onClick={close}
    >
      <div
        className="absolute inset-0 bg-black/40 backdrop-blur-sm"
        onClick={close}
      />
      <div
        className="relative w-full max-w-xl rounded-2xl border border-border bg-card shadow-2xl shadow-black/20 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border/60">
          <Search className="w-4 h-4 text-muted-foreground shrink-0" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("palette.placeholder")}
            className="flex-1 bg-transparent outline-none text-sm placeholder:text-muted-foreground/70"
            aria-label={t("palette.placeholder")}
          />
          <kbd className="text-(--fs-micro-sm) text-muted-foreground/70 bg-secondary px-1.5 py-0.5 rounded border border-border">Esc</kbd>
        </div>

        <div ref={listRef} className="max-h-[45vh] overflow-y-auto py-2">
          {showSearchAction && (
            <button
              data-active={activeIndex === 0}
              onClick={handleSubmitSearch}
              className={`w-full flex items-center gap-3 px-4 py-2.5 text-left text-sm ${activeIndex === 0 ? "bg-primary/10" : ""}`}
            >
              <Search className="w-4 h-4 text-muted-foreground" />
              <span className="text-primary">{t("palette.search_for", { query: query.trim() })}</span>
              <CornerDownLeft className="w-3.5 h-3.5 text-muted-foreground/60 ml-auto" />
            </button>
          )}
          {filtered.length === 0 ? (
            <div className="px-4 py-10 text-center text-sm text-muted-foreground">{t("palette.no_results")}</div>
          ) : (
            [...groups.entries()].map(([section, group]) => (
              <div key={section}>
                <div className="px-4 pt-3 pb-1.5 text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground/70">
                  {section}
                </div>
                {group.map((item) => {
                  const globalIndex = searchOffset + filtered.indexOf(item);
                  const Icon = item.icon;
                  return (
                    <button
                      key={item.href}
                      data-active={activeIndex === globalIndex}
                      onClick={() => handleSelect(filtered.indexOf(item))}
                      className={`w-full flex items-center gap-3 px-4 py-2.5 text-left text-sm ${activeIndex === globalIndex ? "bg-primary/10" : ""}`}
                    >
                      <span className="flex items-center justify-center w-6 h-6 rounded-md bg-secondary/70 shrink-0">
                        <Icon className="w-3.5 h-3.5 text-muted-foreground" />
                      </span>
                      <span className="text-foreground">{item.label}</span>
                      <span className="text-xs text-muted-foreground/70 font-mono ml-auto truncate max-w-40">{item.href}</span>
                    </button>
                  );
                })}
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}