"use client";

import { Card } from "@/components/ui/card";
import { useI18n } from "@/lib/i18n";
import {
  BarChart,
  CheckCircle,
  Clock,
  ListChecks,
  PieChart,
  XCircle,
  Zap,
} from "lucide-react";

interface BreakdownItem {
  label: string;
  count: number;
  color: string;
}

interface SparkPoint {
  x: number;
  y: number;
}

interface AgentTaskOverviewProps {
  totalTasks: number;
  completedTasks: number;
  pendingTasks: number;
  failedTasks: number;
  successRate: number;
  avgResponseTime: string;
  typeBreakdown: BreakdownItem[];
  otherTasks: number;
  sparklinePoints: SparkPoint[];
}

export default function AgentTaskOverview({
  totalTasks,
  completedTasks,
  pendingTasks,
  failedTasks,
  successRate,
  avgResponseTime,
  typeBreakdown,
  otherTasks,
  sparklinePoints,
}: AgentTaskOverviewProps) {
  const { t } = useI18n();

  return (
    <>
      <Card className="p-3.5 sm:p-5 mb-5">
        <div className="flex items-center gap-2.5 sm:gap-5 flex-wrap">
          <div className="flex items-center gap-1.5">
            <ListChecks className="w-3.5 h-3.5 text-primary" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_total_tasks")}</span>
            <span className="text-sm font-bold text-foreground">{totalTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <CheckCircle className="w-3.5 h-3.5 text-emerald-500" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_completed")}</span>
            <span className="text-sm font-bold text-emerald-600 dark:text-emerald-400">{completedTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5 text-amber-500" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_pending")}</span>
            <span className="text-sm font-bold text-amber-600 dark:text-amber-400">{pendingTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <XCircle className="w-3.5 h-3.5 text-red-500" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_failed")}</span>
            <span className="text-sm font-bold text-red-600 dark:text-red-400">{failedTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <PieChart className="w-3.5 h-3.5 text-cyan-500" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_success_rate")}</span>
            <span className="text-sm font-bold text-foreground">{successRate}%</span>
          </div>
          <div className="w-px h-4 bg-border hidden sm:block" />
          <div className="hidden sm:flex items-center gap-1.5">
            <Zap className="w-3.5 h-3.5 text-purple-500" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_avg_response")}</span>
            <span className="text-sm font-bold text-foreground">{avgResponseTime}</span>
          </div>
        </div>
      </Card>

      {totalTasks > 0 && typeBreakdown.length > 0 && (
        <Card className="p-5 sm:p-6 mb-5">
          <div className="flex items-center gap-4 flex-wrap">
            <span className="text-(--fs-micro-sm) font-semibold uppercase tracking-wider text-muted-foreground/70 shrink-0">
              <BarChart className="w-3.5 h-3.5 inline mr-1" aria-hidden="true" />
              {t("agents.detail_task_breakdown")}
            </span>
            <div className="flex-1 flex items-center gap-0.5 h-2 rounded-full overflow-hidden bg-secondary min-w-[100px]">
              {typeBreakdown.map((item) => (
                <div
                  key={item.label}
                  className={`${item.color} h-full transition-all`}
                  style={{ width: `${(item.count / totalTasks) * 100}%` }}
                  title={`${item.label}: ${item.count}`}
                />
              ))}
              {otherTasks > 0 && (
                <div
                  className="bg-muted-foreground/50 h-full"
                  style={{ width: `${(otherTasks / totalTasks) * 100}%` }}
                  title={`${t("agents.detail_other")}: ${otherTasks}`}
                />
              )}
            </div>
            <div className="flex items-center gap-3 flex-wrap">
              {typeBreakdown.map((item) => (
                <span key={item.label} className="flex items-center gap-1.5 text-(--fs-micro-sm) text-muted-foreground">
                  <span className={`w-2 h-2 rounded-full ${item.color}`} />
                  {item.label}: {item.count}
                </span>
              ))}
              {otherTasks > 0 && (
                <span className="flex items-center gap-1.5 text-(--fs-micro-sm) text-muted-foreground">
                  <span className="w-2 h-2 rounded-full bg-muted-foreground" />
                  {t("agents.detail_other")}: {otherTasks}
                </span>
              )}
            </div>
            {sparklinePoints.length > 1 && (
              <div
                className="ml-2 shrink-0"
                title={t("agents.detail_response_times").replace("{count}", String(sparklinePoints.length))}
              >
                <svg
                  viewBox="-2 -2 106 28"
                  className="w-20 h-5"
                  role="img"
                  aria-label={t("agents.detail_response_times").replace("{count}", String(sparklinePoints.length))}
                >
                  <polyline
                    points={sparklinePoints.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(" ")}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    className="text-primary"
                    strokeLinejoin="round"
                    strokeLinecap="round"
                  />
                  {sparklinePoints.map((p, idx) => (
                    <circle
                      key={idx}
                      cx={p.x}
                      cy={p.y}
                      r="1.5"
                      className="fill-primary"
                    />
                  ))}
                </svg>
              </div>
            )}
          </div>
        </Card>
      )}
    </>
  );
}
