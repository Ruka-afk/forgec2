"use client";

import { useState, useEffect, useCallback, useRef, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { ChevronRight, Search } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { safeHref } from "@/lib/safeUrl";

interface SearchResult {
  type: string;
  id: string;
  title: string;
  subtitle: string;
  url: string;
  icon: React.ReactNode;
}

const typeColors: Record<string, string> = {
  agent: "bg-success/10 text-success",
  listener: "bg-primary/10 text-primary",
  credential: "bg-warning/10 text-warning",
  bof: "bg-chart-6/purple text-chart-6",
  user: "bg-chart-5/10 text-chart-5 dark:text-chart-5",
  task: "bg-chart-2/10 text-info",
};

function SearchContent() {
  const { t } = useI18n();
  const searchParams = useSearchParams();
  const query = searchParams.get("q") || "";
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);

  const requestIdRef = useRef(0);

  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) { setResults([]); return; }
    const reqId = ++requestIdRef.current;
    setLoading(true);
    try {
      const data = await api.get<{ success?: boolean; results?: SearchResult[] }>(`/search?q=${encodeURIComponent(q)}`);
      if (reqId !== requestIdRef.current) return;
      setResults(data.results || []);
    } catch {
      if (reqId !== requestIdRef.current) return;
      setResults([]);
    } finally {
      if (reqId === requestIdRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => doSearch(query), 300);
    return () => clearTimeout(timer);
  }, [query, doSearch]);

  return (
    <>
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
              href={safeHref(r.url) ?? "#"}
              className={cn(
                "w-full text-left p-4 flex items-center gap-4 transition-colors h-auto justify-start",
                "rounded-lg border border-transparent hover:border-primary/30 hover:bg-muted/50 dark:hover:bg-muted/30",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2"
              )}>
              <div className={cn("w-10 h-10 rounded-lg flex items-center justify-center text-sm font-bold shrink-0", typeColors[r.type] || "bg-muted/50 text-muted-foreground")}>
                {r.icon}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-0.5">
                  <span className="text-sm font-medium text-foreground">{r.title}</span>
                  <Badge variant="secondary" className="text-(--fs-micro-sm) px-1.5 py-0.5 rounded">{t(`search.type_${r.type}`)}</Badge>
                </div>
                <p className="text-xs text-muted-foreground truncate">{r.subtitle}</p>
              </div>
              <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
            </Link>
          ))}
        </div>
      )}
    </>
  );
}

export default function SearchPage() {
  const { t } = useI18n();
  return (
    <PageContainer title={t("search.title")}>
      <Suspense fallback={
        <div className="flex items-center gap-3 p-4 text-muted-foreground">
          <Spinner />
          <span className="text-sm">{t("common.loading")}</span>
        </div>
      }>
        <SearchContent />
      </Suspense>
    </PageContainer>
  );
}
