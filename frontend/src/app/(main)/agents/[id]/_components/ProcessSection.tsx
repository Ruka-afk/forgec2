"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { SectionCard } from "@/components/ui/section-card";
import { useI18n } from "@/lib/i18n";
import { RefreshCw, Terminal } from "lucide-react";

export interface ProcessSectionProps {
  processList: string | null;
  loading: boolean;
  loadFailed: boolean;
  expanded: boolean;
  /** Collapse/expand toggle from the card header. */
  onToggle: (open: boolean) => void;
  /** Force a fresh process snapshot without toggling the card. */
  onRefresh?: () => void;
}

export default memo(function ProcessSection({ processList, loading, loadFailed, expanded, onToggle, onRefresh }: ProcessSectionProps) {
  const { t } = useI18n();

  return (
    <SectionCard
      className="mb-4"
      title={t("agents.detail_process_list")}
      icon={<Terminal className="w-3.5 h-3.5" />}
      description={t("agents.detail_process_snapshot_hint")}
      collapsible
      defaultOpen={expanded}
      onOpenChange={onToggle}
      action={
        expanded && processList && !loading ? (
          <Button variant="ghost" size="sm" className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline" onClick={onRefresh}>
            <RefreshCw className="w-3 h-3" />{t("agents.detail_refresh")}
          </Button>
        ) : (
          <Button variant="ghost" size="sm" className="text-xs h-auto p-0 text-primary hover:bg-transparent hover:underline" onClick={() => onToggle(!expanded)}>
            {loadFailed ? t("agents.detail_retry") : (expanded ? t("agents.detail_hide") : t("agents.detail_load"))}
          </Button>
        )
      }
    >
      <div className="p-3">
        {loading ? (
          <div className="flex items-center justify-center py-6"><Spinner size="md" /></div>
        ) : (
          <pre className="p-3 bg-muted rounded-lg font-mono text-xs text-foreground whitespace-pre-wrap break-all max-h-64 overflow-y-auto border border-border">
            {processList || t("agents.detail_no_data")}
          </pre>
        )}
      </div>
    </SectionCard>
  );
});