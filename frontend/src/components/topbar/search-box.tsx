"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Search, X } from "lucide-react";

export function SearchBox() {
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);
  const router = useRouter();
  const { t } = useI18n();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const q = query.trim();
    if (q) { router.push(`/search?q=${encodeURIComponent(q)}`); setMobileOpen(false); }
  };

  return (
    <>
      {/* Desktop search */}
      <form onSubmit={handleSubmit} className="relative flex-1 max-w-sm hidden sm:flex">
        <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground/100" />
        <Input id="global-search" type="text" value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t("topbar.search_placeholder")}
          className="h-9 rounded-lg border-transparent bg-secondary/50 pl-8 pr-4 text-(--fs-compact) placeholder:text-muted-foreground/100 focus:border-border focus:bg-card" />
        {query && (
          <Button type="button" onClick={() => setQuery("")}
            variant="ghost" size="icon-xs" className="absolute right-2 top-1/2 -translate-y-1/2" aria-label={t("common.clear_search")}>
            <X className="size-3" />
          </Button>
        )}
      </form>
      {/* Mobile search toggle */}
      <Button variant="ghost" size="icon" className="sm:hidden" onClick={() => setMobileOpen(!mobileOpen)} aria-label={t("nav.search")}>
        <Search className="size-5" />
      </Button>
      {mobileOpen && (
        <form onSubmit={handleSubmit} className="sm:hidden absolute top-full left-0 right-0 z-50 p-3 bg-card border-b border-border shadow-lg animate-fade-in">
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground/100" />
            <Input type="text" value={query} autoFocus
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t("topbar.search_placeholder")}
              className="h-9 pl-8 pr-8 text-sm bg-secondary/50" />
            {query && (
              <Button type="button" onClick={() => setQuery("")} variant="ghost" size="icon-xs"
                className="absolute right-2 top-1/2 -translate-y-1/2" aria-label={t("common.clear_search")}>
                <X className="size-3" />
              </Button>
            )}
          </div>
        </form>
      )}
    </>
  );
}