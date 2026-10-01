
import type { Execution } from "./types";
import { getStatusColor } from "./types";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { DataError } from "@/components/ui/data-state";
import { Terminal } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";

interface BOFExecutionsTabProps {
  executions: Execution[];
  loading: boolean;
  error?: string | null;
  onRetry?: () => void;
}

export default function BOFExecutionsTab({ executions, loading, error, onRetry }: BOFExecutionsTabProps) {
  const { t } = useI18n();
  if (loading) {
    return (
      <Card className="overflow-hidden">
        <div className="text-center py-12 text-muted-foreground">
          <Spinner />
        </div>
      </Card>
    );
  }

  return (
    <Card className="overflow-hidden">
      {executions.length > 0 ? (
        <div className="divide-y divide-border">
          {executions.map((ex: Execution, i: number) => (
            <div key={ex.id || i} className="px-5 py-4">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-3">
                  <div className="size-8 bg-success/15 rounded-lg flex items-center justify-center text-success">
                    <Terminal className="size-4" />
                  </div>
                  <div>
                    <div className="text-sm font-medium text-foreground">{ex.bof_name || ex.args?.split(/\s+/)[0] || t("bof.unknown")}</div>
                    <div className="text-xs text-muted-foreground">
                      {ex.agent_name || ex.agent_hostname || t("bof.unknown")} {ex.args ? `· args: ${ex.args}` : ""}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary" className={`text-(--fs-micro-sm) px-2 py-0.5 rounded-full ${getStatusColor(ex.status || "")}`}>
                    {ex.status || "pending"}
                  </Badge>
                  <span className="text-xs text-muted-foreground">{ex.created_at || ""}</span>
                </div>
              </div>
              {(ex.result) && (
                <pre className="text-xs font-mono text-chart-1 bg-card rounded-lg p-3 mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap border border-border">
                  {ex.result}
                </pre>
              )}
            </div>
          ))}
        </div>
      ) : error ? (
        // A failed refresh with no runs on screen: the "no executions"
        // line would read as a fact instead of about the request.
        <DataError message={t("bof.executions_unreadable", { message: error })} onRetry={onRetry} />
      ) : (
        <EmptyState icon={Terminal} title={t("bof.no_executions")} />
      )}
    </Card>
  );
}

