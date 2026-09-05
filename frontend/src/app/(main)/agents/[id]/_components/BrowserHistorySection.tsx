"use client";

import { memo, useMemo, useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { Globe, History, Search } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { parseBrowserHistory } from "./browser-history";
import { useCollectTask } from "./useCollectTask";

interface BrowserHistorySectionProps {
  agentId: string;
  online: boolean;
}

const BROWSERS = ["all", "chrome", "edge", "brave", "firefox", "safari"] as const;

export default memo(function BrowserHistorySection({ agentId, online }: BrowserHistorySectionProps) {
  const { t } = useI18n();
  const [browser, setBrowser] = useState<string>("all");
  const { busy, collect } = useCollectTask(agentId);
  const collecting = busy !== null;
  const [raw, setRaw] = useState("");
  const [collected, setCollected] = useState(false);
  const [query, setQuery] = useState("");

  const rows = useMemo(() => parseBrowserHistory(raw), [raw]);
  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return rows;
    return rows.filter(
      (r) =>
        r.url.toLowerCase().includes(q) ||
        r.title.toLowerCase().includes(q) ||
        r.browser.toLowerCase().includes(q),
    );
  }, [rows, query]);

  const handleCollect = async () => {
    const output = await collect("collect", paths.agents.browserHistory(agentId), {
      body: { browser },
      storeResult: false,
      errorText: t("agents.history_collect_failed"),
    });
    if (output === null) return;
    setRaw(output);
    setCollected(true);
    const n = parseBrowserHistory(output).length;
    toast.success(t("agents.history_collected").replace("{n}", String(n)));
  };

  return (
    <Card className="mb-4 overflow-hidden">
      <div className="flex flex-wrap items-center gap-2 border-b border-border px-4 py-3">
        <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">
          <History className="size-3.5 text-primary" />
          {t("agents.history_title")}
        </h3>
        <span className="ml-auto flex items-center gap-2">
          <Select value={browser} onValueChange={(v) => v !== null && setBrowser(v)}>
            <SelectTrigger className="h-8 w-32">
              <SelectValue placeholder={t("agents.history_browser")} />
            </SelectTrigger>
            <SelectContent>
              {BROWSERS.map((b) => (
                <SelectItem key={b} value={b}>
                  {b === "all" ? t("agents.history_all_browsers") : b}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button size="sm" onClick={() => void handleCollect()} disabled={!online || collecting}>
            {collecting ? (
              <>
                <Spinner size="xs" /> {t("agents.history_collecting")}
              </>
            ) : (
              <>
                <Globe className="size-4" /> {t("agents.history_collect")}
              </>
            )}
          </Button>
        </span>
      </div>
      <div className="p-3">
        {!collected && !collecting && (
          <EmptyState
            icon={History}
            title={t("agents.history_empty")}
            message={t("agents.history_empty_hint")}
          />
        )}
        {collecting && !collected && (
          <div className="flex items-center justify-center gap-3 py-8">
            <Spinner size="sm" />
            <p className="text-sm text-muted-foreground">{t("agents.history_collecting")}</p>
          </div>
        )}
        {collected && (
          <>
            <div className="relative mb-2">
              <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
              <Input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t("agents.history_search_placeholder")}
                aria-label={t("agents.history_search_placeholder")}
                className="h-8 pl-8 text-xs"
              />
            </div>
            {visible.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">{t("agents.history_no_match")}</p>
            ) : (
              <div className="max-h-96 overflow-auto rounded-lg border border-border">
                <Table>
                  <TableHeader>
                    <TableRow className="bg-muted/50">
                      <TableHead className="w-36">{t("agents.history_col_browser")}</TableHead>
                      <TableHead>{t("agents.history_col_url")}</TableHead>
                      <TableHead className="w-16 text-right">{t("agents.history_col_visits")}</TableHead>
                      <TableHead className="w-40">{t("agents.history_col_time")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visible.slice(0, 500).map((r, i) => (
                      <TableRow key={`${r.browser}-${r.url}-${i}`}>
                        <TableCell className="text-xs text-muted-foreground">{r.browser}</TableCell>
                        <TableCell className="min-w-0">
                          <a
                            href={r.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="block max-w-xl truncate font-mono text-xs text-primary hover:underline"
                            title={r.url}
                          >
                            {r.title || r.url}
                          </a>
                          {r.title && (
                            <span className="block max-w-xl truncate text-xs text-muted-foreground" title={r.url}>
                              {r.url}
                            </span>
                          )}
                        </TableCell>
                        <TableCell className="text-right font-mono text-xs text-muted-foreground">
                          {r.visits || "—"}
                        </TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">{r.time || "—"}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
            {visible.length > 500 && (
              <p className="mt-1 text-xs text-muted-foreground">
                {t("agents.history_truncated").replace("{n}", String(visible.length - 500))}
              </p>
            )}
          </>
        )}
      </div>
    </Card>
  );
});
