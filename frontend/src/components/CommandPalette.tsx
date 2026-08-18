"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { NAV_SECTIONS, filterNavByPermissions } from "@/lib/navigation";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { Search, CornerDownLeft, Server } from "lucide-react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Kbd } from "@/components/ui/kbd";
import { ScrollArea } from "@/components/ui/scroll-area";

interface PaletteItem {
  href: string;
  label: string;
  section: string;
  icon: React.ComponentType<{ className?: string }>;
  subtitle?: string;
}

function normalize(s: string): string {
  return s.toLowerCase().replace(/[-_/]/g, " ");
}

export default function CommandPalette() {
  const { t } = useI18n();
  const router = useRouter();
  const open = useAppStore((s) => s.commandPaletteOpen);
  const setOpen = useAppStore((s) => s.setCommandPaletteOpen);
  const permissions = useAppStore((s) => s.currentPermissions);
  const { agents } = useAgentList();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const items: PaletteItem[] = useMemo(() => {
    const out: PaletteItem[] = [];
    for (const section of NAV_SECTIONS) {
      for (const item of filterNavByPermissions(section.items, permissions)) {
        out.push({
          href: item.href,
          label: t(item.labelKey),
          section: t("section." + section.titleKey),
          icon: item.icon,
        });
      }
    }
    return out;
  }, [t, permissions]);

  const agentItems: PaletteItem[] = useMemo(() => {
    const q = normalize(query.trim());
    if (!q) return [];
    const out: PaletteItem[] = [];
    for (const agent of agents) {
      const id = agent.id;
      if (!id) continue;
      const haystack = normalize([agent.hostname, agent.ip, agent.username, agent.os].filter(Boolean).join(" "));
      if (!haystack.includes(q)) continue;
      out.push({
        href: `/agents/${id}`,
        label: agent.hostname ?? id,
        section: t("palette.agents"),
        icon: Server,
        subtitle: [agent.username, agent.ip].filter(Boolean).join(" \u00b7 "),
      });
      if (out.length === 6) break;
    }
    return out;
  }, [agents, query, t]);

  const filtered = useMemo(() => {
    const q = normalize(query.trim());
    if (!q) return items;
    return [...items, ...agentItems].filter(
      (i) => normalize(i.label).includes(q) || normalize(i.href).includes(q) || normalize(i.section).includes(q) || (i.subtitle && normalize(i.subtitle).includes(q)),
    );
  }, [items, agentItems, query]);

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

  const groups = new Map<string, PaletteItem[]>();
  for (const item of filtered) {
    const list = groups.get(item.section) || [];
    list.push(item);
    groups.set(item.section, list);
  }

  const searchOffset = showSearchAction ? 1 : 0;

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) close(); }}>
      <DialogContent
        showCloseButton={false}
        className="top-[18vh] translate-y-0 sm:max-w-xl gap-0 overflow-hidden p-0"
        aria-label={t("palette.title")}
      >
        <DialogTitle className="sr-only">{t("palette.title")}</DialogTitle>
        <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
          <Search className="w-4 h-4 shrink-0 text-muted-foreground" />
          <Input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("palette.placeholder")}
            aria-label={t("palette.placeholder")}
            role="combobox"
            aria-expanded={open}
            aria-controls="command-palette-list"
            aria-autocomplete="list"
            aria-activedescendant={activeIndex >= 0 ? `palette-option-${activeIndex}` : undefined}
            className="h-8 flex-1 border-0 bg-transparent px-0 shadow-none focus-visible:ring-0 dark:bg-transparent"
          />
          <Kbd>Esc</Kbd>
        </div>

        <ScrollArea className="max-h-[45vh]">
          <div ref={listRef} id="command-palette-list" role="listbox" aria-label={t("palette.title")} className="py-2">
            {showSearchAction && (
                <Button
                  type="button"
                  variant="ghost"
                  role="option"
                  id="palette-option-0"
                  aria-selected={activeIndex === 0}
                  data-active={activeIndex === 0}
                  onClick={handleSubmitSearch}
                  className={`h-auto w-full justify-start gap-3 rounded-none px-4 py-2.5 text-left text-sm ${activeIndex === 0 ? "bg-primary/10" : ""}`}
                >
                <Search className="w-4 h-4 text-muted-foreground" />
                <span className="text-primary">{t("palette.search_for", { query: query.trim() })}</span>
                <CornerDownLeft className="ml-auto w-3.5 h-3.5 text-muted-foreground/60" />
              </Button>
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
                      <Button
                        key={item.href}
                        type="button"
                        variant="ghost"
                        role="option"
                        id={`palette-option-${globalIndex}`}
                        aria-selected={activeIndex === globalIndex}
                        data-active={activeIndex === globalIndex}
                        onClick={() => handleSelect(filtered.indexOf(item))}
                        className={`h-auto w-full justify-start gap-3 rounded-none px-4 py-2.5 text-left text-sm ${activeIndex === globalIndex ? "bg-primary/10" : ""}`}
                      >
                        <span className="flex size-6 shrink-0 items-center justify-center rounded-md bg-secondary/70">
                          <Icon className="w-3.5 h-3.5 text-muted-foreground" />
                        </span>
                        <span className="flex min-w-0 flex-col">
                          <span className="truncate text-foreground">{item.label}</span>
                          {item.subtitle && (
                            <span className="truncate text-xs text-muted-foreground/70">{item.subtitle}</span>
                          )}
                        </span>
                        <span className="ml-auto max-w-40 truncate font-mono text-xs text-muted-foreground/70">{item.href}</span>
                      </Button>
                    );
                  })}
                </div>
              ))
            )}
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  );
}