"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { AppWindow, Apple, Binary, CheckCircle2, ChevronDown, Disc, HardDrive, History, PackageOpen, Puzzle, Terminal, XCircle } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { BuildHistoryEntry } from "@/types/generate";

const FORMAT_ICONS: Record<string, React.ReactNode> = {
  exe: <AppWindow className="size-4" />, dll: <Puzzle className="size-4" />, ps1: <Terminal className="size-4" />,
  linux: <HardDrive className="size-4" />, macos: <Apple className="size-4" />,
  stager: <PackageOpen className="size-4" />, shellcode: <Binary className="size-4" />, donut: <Disc className="size-4" />,
};

export default function BuildHistorySection({ refreshKey }: { refreshKey?: number }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const { data, loading, refresh } = useApiResource<{ builds: BuildHistoryEntry[] }>({
    fetcher: async () => {
      const d = await api.get<{ builds?: BuildHistoryEntry[]; Logs?: BuildHistoryEntry[]; logs?: BuildHistoryEntry[] }>(
        paths.builds.list("pageSize=20"),
      );
      return { builds: d.builds || d.logs || d.Logs || [] };
    },
    toastThrottleMs: 0,
    errorMessage: t("generate.toast.build_history_load_failed"),
  });
  useEffect(() => { if (refreshKey) void refresh(); }, [refreshKey, refresh]);
  const builds = data?.builds ?? [];

  if (loading) return null;

  return (
    <div className="mt-8">
      <Collapsible open={expanded} onOpenChange={setExpanded}>
        <CollapsibleTrigger>
          <Button type="button" variant="ghost" className="flex items-center gap-x-3 mb-5 w-full text-left justify-start h-auto py-0">
            <div className="size-10 bg-card ring-1 ring-border/50 rounded-lg flex items-center justify-center text-muted-foreground"><History className="size-4" /></div>
            <div className="flex-1">
              <div className="text-sm font-semibold text-foreground">{t("builds.history_title")}</div>
              <div className="text-xs text-muted-foreground">{t("generate.history_recent", { count: builds.length })}</div>
            </div>
            <ChevronDown className="size-3 text-muted-foreground transition-transform" data-rotate={expanded ? "180" : undefined} />
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          {builds.length === 0 ? (
            <Card className="p-4">
              <EmptyState
                icon={History}
                title={t("generate.history_empty")}
                message={t("generate.history_empty_desc")}
              />
            </Card>
          ) : (
            <Card className="overflow-hidden p-0">
              <Table>
                <TableHeader>
                  <TableRow className="text-xs text-muted-foreground">
                    <TableHead className="text-left font-medium">{t("builds.col_time")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_format")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_filename")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.status")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_user")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {builds.slice(0, 20).map((b) => (
                    <TableRow key={b.id}>
                      <TableCell className="text-xs text-muted-foreground whitespace-nowrap">{formatTime(b.created_at)}</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-x-1.5 text-xs font-medium text-muted-foreground">
                          <span className="text-muted-foreground/100">{FORMAT_ICONS[b.format]}</span>
                          <span className="uppercase">{b.format}</span>
                        </span>
                      </TableCell>
                      <TableCell className="text-xs font-mono text-muted-foreground">{b.filename || "\u2014"}</TableCell>
                      <TableCell>
                        {b.status === "success" ? (
                          <Badge variant="success" className="text-xs gap-1"><CheckCircle2 className="size-4" /> {t("builds.success")}</Badge>
                        ) : (
                          <Tooltip>
                            <TooltipTrigger>
                              <Badge variant="destructive" className="text-xs gap-1"><XCircle className="size-4" /> {t("builds.error")}</Badge>
                            </TooltipTrigger>
                            <TooltipContent>{b.error}</TooltipContent>
                          </Tooltip>
                        )}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{b.user || "system"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Card>
          )}
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
