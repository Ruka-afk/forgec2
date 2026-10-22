
import { memo } from "react";
import { SectionCard } from "@/components/ui/section-card";
import { timeAgo } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import type { LogEntry } from "./agent-detail-utils";
import { Circle, History } from "lucide-react";

interface ConnectionLogSectionProps {
  logs: LogEntry[];
}

export default memo(function ConnectionLogSection({ logs }: ConnectionLogSectionProps) {
  const { t } = useI18n();
  if (logs.length === 0) return null;

  return (
    <SectionCard className="mb-(--page-gap)" title={t("agents.detail_connection_log")} icon={History} tone="muted">
      <div className="max-h-64 overflow-y-auto">
        <div className="divide-y divide-border">
          {logs.map((log, i) => (
            <div key={log.id || i} className="px-4 py-2 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <Circle className={`size-1.5 fill-current ${log.type === "online" ? "text-success-ink" : log.type === "offline" ? "text-destructive-ink" : "text-primary"}`} />
                <span className="text-xs text-foreground">{log.user || t("agents.detail_log_system")}</span>
                {log.message && <span className="text-xs text-muted-foreground">{log.message}</span>}
              </div>
              <span className="text-micro-sm text-muted-foreground whitespace-nowrap">{(log.created_at) ? timeAgo(String(log.created_at), t) : ""}</span>
            </div>
          ))}
        </div>
      </div>
    </SectionCard>
  );
});