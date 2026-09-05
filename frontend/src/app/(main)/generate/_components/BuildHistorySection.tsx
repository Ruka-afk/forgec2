"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { AppWindow, Apple, Binary, CheckCircle2, ChevronDown, Disc, Download, HardDrive, History, PackageOpen, Puzzle, RefreshCw, Terminal, XCircle } from "lucide-react";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { BuildHistoryEntry } from "@/types/generate";
import { isPayloadFormat } from "./generate-format";

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

  const handleDownload = async (build: BuildHistoryEntry) => {
    if (!build.id) return;
    try {
      const { blob, filename } = await api.downloadGet(
        paths.builds.download(String(build.id)),
        build.filename || `build-${build.id}`,
      );
      downloadBlob(blob, filename);
    } catch {
      toast.error(t("builds.toast.download_failed"));
    }
  };

  const rebuildHref = (build: BuildHistoryEntry) => {
    const params = new URLSearchParams();
    if (build.listener_id) params.set("listener_id", String(build.listener_id));
    const fmt = (build.format || "").toLowerCase();
    if (isPayloadFormat(fmt)) params.set("format", fmt);
    const q = params.toString();
    return q ? `/generate?${q}` : "/generate";
  };

  if (loading) return null;

  return (
    <div className="rounded-xl border border-border/60 bg-card p-3.5 shadow-sm">
      <Collapsible open={expanded} onOpenChange={setExpanded}>
        <CollapsibleTrigger className="w-full">
          <div className="flex w-full items-center gap-x-3 text-left">
            <div className="grid size-10 shrink-0 place-items-center rounded-xl bg-muted text-muted-foreground ring-1 ring-border/40"><History className="size-4" /></div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold tracking-tight text-foreground">{t("builds.history_title")}</div>
              <div className="text-xs leading-4 text-muted-foreground">{t("generate.history_recent", { count: builds.length })}</div>
            </div>
            <Badge variant="outline" className="rounded-full">{builds.length}</Badge>
            <ChevronDown className={`size-4 shrink-0 text-muted-foreground transition-transform duration-200 ${expanded ? "rotate-180" : ""}`} />
          </div>
        </CollapsibleTrigger>
        <CollapsibleContent className="mt-4">
          {builds.length === 0 ? (
            <div className="rounded-xl border border-dashed border-border/70 bg-muted/20 p-4">
              <EmptyState
                icon={History}
                title={t("generate.history_empty")}
                message={t("generate.history_empty_desc")}
              />
            </div>
          ) : (
            <Card className="overflow-hidden rounded-xl border-border/60 p-0 shadow-sm">
              <div className="max-h-[420px] overflow-y-auto [scrollbar-width:thin]">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-muted/80 backdrop-blur-sm">
                  <TableRow className="text-xs text-muted-foreground hover:bg-transparent">
                    <TableHead className="text-left font-medium">{t("builds.col_time")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_format")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_filename")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.status")}</TableHead>
                    <TableHead className="text-left font-medium">{t("builds.col_user")}</TableHead>
                    <TableHead className="text-right font-medium">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {builds.slice(0, 20).map((b) => (
                    <TableRow key={b.id} className="transition-colors hover:bg-muted/40">
                      <TableCell className="whitespace-nowrap text-xs text-muted-foreground">{formatTime(b.created_at)}</TableCell>
                      <TableCell>
                        <span className="inline-flex items-center gap-x-1.5 rounded-md bg-muted/60 px-1.5 py-0.5 text-xs font-medium text-muted-foreground ring-1 ring-border/30">
                          <span>{FORMAT_ICONS[b.format]}</span>
                          <span className="uppercase">{b.format}</span>
                        </span>
                      </TableCell>
                      <TableCell className="max-w-[220px] truncate font-mono text-xs text-muted-foreground" title={b.filename}>{b.filename || "\u2014"}</TableCell>
                      <TableCell>
                        {b.status === "success" ? (
                          <Badge variant="success" className="gap-1 rounded-full text-xs"><CheckCircle2 className="size-3.5" /> {t("builds.success")}</Badge>
                        ) : (
                          <Tooltip>
                            <TooltipTrigger>
                              <Badge variant="destructive" className="gap-1 rounded-full text-xs"><XCircle className="size-3.5" /> {t("builds.error")}</Badge>
                            </TooltipTrigger>
                            <TooltipContent>{b.error}</TooltipContent>
                          </Tooltip>
                        )}
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{b.user || "system"}</TableCell>
                      <TableCell className="text-right">
                        <div className="inline-flex items-center gap-1">
                          {(b.status === "success" || b.output_path) && (
                            <Tooltip>
                              <TooltipTrigger render={<Button type="button" variant="ghost" size="icon-xs" onClick={() => void handleDownload(b)} aria-label={t("builds.download_artifact")} className="rounded-lg hover:bg-primary/10 hover:text-primary">
                                <Download className="size-3.5" />
                              </Button>} />
                              <TooltipContent>{t("builds.download_artifact")}</TooltipContent>
                            </Tooltip>
                          )}
                          <Tooltip>
                            <TooltipTrigger render={<Button type="button" variant="ghost" size="icon-xs" render={<Link href={rebuildHref(b)} />} aria-label={t("generate.rebuild")} className="rounded-lg hover:bg-muted">
                              <RefreshCw className="size-3.5" />
                            </Button>} />
                            <TooltipContent>{t("generate.rebuild")}</TooltipContent>
                          </Tooltip>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              </div>
            </Card>
          )}
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
