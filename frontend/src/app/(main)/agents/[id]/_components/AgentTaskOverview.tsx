"use client";

import { Card } from "@/components/ui/card";
import { useI18n } from "@/lib/i18n";
import {
  CheckCircle,
  Clock,
  ListChecks,
  XCircle,
} from "lucide-react";

interface AgentTaskOverviewProps {
  totalTasks: number;
  completedTasks: number;
  pendingTasks: number;
  failedTasks: number;
}

export default function AgentTaskOverview({
  totalTasks,
  completedTasks,
  pendingTasks,
  failedTasks,
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
            <CheckCircle className="w-3.5 h-3.5 text-success" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_completed")}</span>
            <span className="text-sm font-bold text-success">{completedTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5 text-warning" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_pending")}</span>
            <span className="text-sm font-bold text-warning">{pendingTasks}</span>
          </div>
          <div className="w-px h-4 bg-border" />
          <div className="flex items-center gap-1.5">
            <XCircle className="w-3.5 h-3.5 text-destructive" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground/70">{t("agents.detail_failed")}</span>
            <span className="text-sm font-bold text-destructive">{failedTasks}</span>
          </div>
        </div>
      </Card>
    </>
  );
}
