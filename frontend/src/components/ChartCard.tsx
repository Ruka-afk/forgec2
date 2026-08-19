"use client";

import { useRef, ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { exportElementPng } from "@/lib/chartExport";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { IconBadge } from "@/components/ui/icon-badge";
import { Download, RefreshCw, AlertTriangle, type LucideIcon } from "lucide-react";
import { Spinner } from "@/components/ui/spinner";
import { useI18n } from "@/lib/i18n";
import type { Hue } from "@/lib/ui/statusStyles";

interface ChartCardProps {
  title: string;
  icon: LucideIcon;
  iconColor?: Hue;
  children: ReactNode;
  onRefresh?: () => void;
  loading?: boolean;
  error?: boolean;
  exportFilename?: string;
  className?: string;
}

export function ChartCard({ title, icon: Icon, iconColor = "primary", children, onRefresh, loading, error, exportFilename, className }: ChartCardProps) {
  const { t } = useI18n();
  const chartRef = useRef<HTMLDivElement>(null);
  const handleExport = async () => {
    if (!chartRef.current || !exportFilename) return;
    try { await exportElementPng(chartRef.current, exportFilename); } catch (e) { if (process.env.NODE_ENV === "development") console.warn("[ChartCard] export failed:", e); }
  };
  return (
    <Card role="region" aria-label={title} className={`p-5 sm:p-6 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30 ${className || ""}`}>
      <div className="flex items-center justify-between mb-4 pb-4 border-b border-border/50">
        <div className="font-semibold text-foreground flex items-center gap-x-2.5 text-sm">
          <IconBadge icon={Icon} color={iconColor} size="md" />
          <span>{title}</span>
        </div>
        <div className="flex items-center gap-0.5">
          {exportFilename && (
            <Tooltip>
              <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={handleExport} aria-label={t("common.export_png")} className="text-muted-foreground hover:text-foreground" />}>
                <Download className="w-4 h-4" />
              </TooltipTrigger>
              <TooltipContent>{t("common.export_png")}</TooltipContent>
            </Tooltip>
          )}
          {onRefresh && (
            <Tooltip>
              <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={onRefresh} disabled={loading} aria-label={t("common.refresh")} className="text-muted-foreground hover:text-foreground" />}>
                {loading ? <Spinner size="xs" /> : <RefreshCw className="w-4 h-4" />}
              </TooltipTrigger>
              <TooltipContent>{t("common.refresh")}</TooltipContent>
            </Tooltip>
          )}
        </div>
      </div>
      {error ? (
        <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <div className="w-12 h-12 rounded-xl bg-warning/10 flex items-center justify-center mb-3">
            <AlertTriangle className="w-6 h-6 text-warning" />
          </div>
          <span className="text-sm font-medium">{t("chart.load_failed")}</span>
          <span className="text-xs text-muted-foreground/70 mt-1">{t("chart.try_later")}</span>
          {onRefresh && (
            <Button variant="outline" size="sm" onClick={onRefresh} className="mt-3">
              <RefreshCw className="w-3 h-3 mr-1.5" />{t("common.try_again")}
            </Button>
          )}
        </div>
      ) : loading ? (
        <div className="space-y-3 py-2">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-32 w-full" />
          <div className="flex gap-2">
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-3 w-16" />
          </div>
        </div>
      ) : (
        <div ref={chartRef}>{children}</div>
      )}
    </Card>
  );
}
