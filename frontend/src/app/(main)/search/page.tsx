"use client";

import { useState, useEffect, useCallback, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { EmptyState, Spinner, PageHeader } from "@/components/UI";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { ChevronRight, Search } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface SearchResult {
  type: string;
  id: string;
  title: string;
  subtitle: string;
  url: string;
  icon: React.ReactNode;
}

const typeColors: Record<string, string> = {
  agent: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  listener: "bg-primary/10 text-primary",
  credential: "bg-amber-500/10 text-amber-600 dark:text-amber-400",
  bof: "bg-purple-500/10 text-purple-600 dark:text-purple-400",
  user: "bg-pink-500/10 text-pink-600 dark:text-pink-400",
  task: "bg-cyan-500/10 text-cyan-600 dark:text-cyan-400",
};

function SearchContent() {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const query = searchParams.get("q") || "";
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) { setResults([]); return; }
    setLoading(true);
    try {
      const data = await api.get<{ success?: boolean; results?: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}`);
      setResults(data.results || []);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => doSearch(query), 300);
    return () => clearTimeout(timer);
  }, [query, doSearch]);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("search.title")} />
      <p className="text-sm text-muted-foreground mb-6">
        {query ? (
          <>{t("search.results_for")} <span className="font-medium text-foreground">&ldquo;{query}&rdquo;</span></>
        ) : (
          t("search.hint")
        )}
      </p>

      {loading && (
        <div className="flex items-center gap-3 p-4 text-muted-foreground">
          <Spinner />
          <span className="text-sm">{t("search.searching")}</span>
        </div>
      )}

      {!loading && query && results.length === 0 && (
        <EmptyState icon={Search} title={t("search.no_results")} />
      )}

      {!loading && results.length > 0 && (
        <div className="space-y-2">
          {results.map((r, i) => (
            <Link key={`${r.type}-${r.id}-${i}`}
              href={r.url}
              className={cn(
                "w-full text-left p-4 flex items-center gap-4 transition-colors h-auto justify-start",
                "rounded-xl border border-transparent hover:border-primary/30 hover:bg-muted/50 dark:hover:bg-muted/30",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
              )}>
              <div className={cn("w-10 h-10 rounded-xl flex items-center justify-center text-sm font-bold shrink-0", typeColors[r.type] || "bg-muted/50 text-muted-foreground")}>
                {r.icon}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-0.5">
                  <span className="text-sm font-medium text-foreground">{r.title}</span>
                  <Badge variant="secondary" className="text-(--font-size-micro-sm) px-1.5 py-0.5 rounded capitalize">{r.type}</Badge>
                </div>
                <p className="text-xs text-muted-foreground truncate">{r.subtitle}</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

export default function SearchPage() {
  return (
    <Suspense fallback={
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
          <div className="text-xl font-semibold tracking-tight text-foreground leading-tight mb-1">Search Results</div>
        <div className="flex items-center gap-3 p-4 text-muted-foreground">
          <Spinner />
          <span className="text-sm">Loading...</span>
        </div>
      </div>
    }>
      <SearchContent />
    </Suspense>
  );
}
