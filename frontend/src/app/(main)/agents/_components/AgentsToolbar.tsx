"use client";

import { memo } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Download, Grip, History, ListChecks, ListOrdered, Pause, Play, Plus, RefreshCw } from "lucide-react";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface AgentsToolbarProps {
  t: TKey;
  bulkMode: boolean;
  onToggleBulk: () => void;
  showResults: boolean;
  onToggleResults: () => void;
  exporting: boolean;
  onExport: () => void;
  autoRefresh: boolean;
  onToggleAutoRefresh: () => void;
  effectiveViewMode: "table" | "grid";
  onToggleView: () => void;
  onRefresh: () => void;
}

/** PageContainer action buttons for the agents list. */
export default memo(function AgentsToolbar({
  t, bulkMode, onToggleBulk, showResults, onToggleResults,
  exporting, onExport, autoRefresh, onToggleAutoRefresh,
  effectiveViewMode, onToggleView, onRefresh,
}: AgentsToolbarProps) {
  return (
    <>
      <Button
        variant="outline"
        onClick={onToggleBulk}
        aria-label={t("agents.bulk_ops")}
        className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
          bulkMode
            ? "bg-primary text-primary-foreground border-primary hover:bg-primary/80"
            : "text-muted-foreground"
        }`}
      >
        <ListChecks className="size-4" />
        <span className="hidden sm:inline text-sm">{t("agents.bulk_ops")}</span>
      </Button>
      <Button
        variant="outline"
        onClick={onToggleResults}
        className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
          showResults
            ? "bg-primary text-primary-foreground border-primary hover:bg-primary/80"
            : "text-muted-foreground"
        }`}
        title={t("agents.bulk_results_title")}
      >
        <History className="size-4" />
        <span className="hidden sm:inline text-sm">{t("agents.results")}</span>
      </Button>
      <Button
        variant="outline"
        onClick={onExport}
        disabled={exporting}
        className="h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem]"
        title={t("agents.export_csv_title")}
      >
        <Download className="size-4" />
        <span className="hidden sm:inline text-foreground text-sm">{t("agents.export")}</span>
      </Button>
      <Button
        variant="outline"
        onClick={onToggleAutoRefresh}
        className={`h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem] transition-all ${
          autoRefresh
            ? "bg-success border-success text-white hover:bg-success/10"
            : "text-muted-foreground"
        }`}
        title={autoRefresh ? t("agents.auto_refresh_on") : t("agents.auto_refresh_off")}
      >
        {autoRefresh ? <Pause className="size-4" /> : <Play className="size-4" />}
        <span className="hidden sm:inline text-sm">{autoRefresh ? t("agents.live") : t("agents.auto")}</span>
      </Button>
      <Button
        variant="outline"
        onClick={onToggleView}
        className="hidden h-9 min-h-[2.75rem] min-w-[2.75rem] gap-2 rounded-lg px-3 sm:inline-flex sm:h-10"
        title={effectiveViewMode === "table" ? t("agents.switch_grid") : t("agents.switch_table")}
      >
        {effectiveViewMode === "table" ? <Grip className="size-4" /> : <ListOrdered className="size-4" />}
      </Button>
      <Button
        variant="outline"
        onClick={onRefresh}
        aria-label={t("agents.refresh")}
        className="h-9 sm:h-10 px-3 rounded-lg gap-2 min-w-[2.75rem] min-h-[2.75rem]"
      >
        <RefreshCw className="size-4" />
        <span className="hidden sm:inline text-foreground text-sm">{t("agents.refresh")}</span>
      </Button>
      <Button render={<Link href="/generate" />}>
        <Plus className="size-4" />
        <span className="hidden sm:inline">{t("agents.generate_implant")}</span>
        <span className="sm:hidden">{t("agents.new")}</span>
      </Button>
    </>
  );
});
