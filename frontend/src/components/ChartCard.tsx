"use client";

import { useRef, ReactNode } from "react";
import { exportElementPng } from "@/lib/chartExport";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Download, RefreshCw, AlertTriangle, type LucideIcon } from "lucide-react";

interface ChartCardProps {
  title: string;
  icon: LucideIcon;
  iconColor: string;
  children: ReactNode;
  onRefresh?: () => void;
  loading?: boolean;
  error?: boolean;
  exportFilename?: string;
  className?: string;
}

export function ChartCard({ title, icon: Icon, iconColor, children, onRefresh, loading, error, exportFilename, className }: ChartCardProps) {
  const chartRef = useRef<HTMLDivElement>(null);
  const handleExport = async () => {
    if (!chartRef.current || !exportFilename) return;
    try { await exportElementPng(chartRef.current, exportFilename); } catch (e) { if (process.env.NODE_ENV === "development") console.warn("[ChartCard] export failed:", e); }
  };
  return (
    <Card className={`p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30 ${className || ""}`}>
      <div className="flex items-center justify-between mb-3 pb-3 border-b border-border/50">
        <div className="font-semibold text-foreground flex items-center gap-x-2 text-sm">
          <div className={`w-9 h-9 rounded-lg flex items-center justify-center ${iconColor.includes("emerald") ? "bg-emerald-500/10" : iconColor.includes("amber") ? "bg-amber-500/10" : iconColor.includes("red") ? "bg-destructive/10" : "bg-primary/10"}`}>
            <Icon className={`w-4 h-4 ${iconColor}`} />
          </div>
          <span>{title}</span>
        </div>
        <div className="flex items-center gap-0.5">
          {exportFilename && (
            <Button variant="ghost" size="icon-sm" onClick={handleExport} title="Export PNG" aria-label="Export PNG" className="text-muted-foreground hover:text-foreground">
              <Download className="w-4 h-4" />
            </Button>
          )}
          {onRefresh && (
            <Button variant="ghost" size="icon-sm" onClick={onRefresh} disabled={loading} title="Refresh" aria-label="Refresh" className="text-muted-foreground hover:text-foreground">
              <RefreshCw className={`w-4 h-4 transition-transform ${loading ? "animate-spin" : ""}`} />
            </Button>
          )}
        </div>
      </div>
      {error ? (
        <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
          <div className="w-12 h-12 rounded-xl bg-amber-500/10 flex items-center justify-center mb-3">
            <AlertTriangle className="w-6 h-6 text-amber-500" />
          </div>
          <span className="text-sm font-medium">Failed to load chart</span>
          <span className="text-xs text-muted-foreground/70 mt-1">Please try again later</span>
          {onRefresh && (
            <Button variant="outline" size="sm" onClick={onRefresh} className="mt-3">
              <RefreshCw className="w-3 h-3 mr-1.5" />Retry
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
