import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { RefreshCw, RotateCcw, Zap } from "lucide-react";
import { toast } from "sonner";

interface ReloadGroup {
  token: string;
  mode: "hot" | "static";
  mechanism: "hook" | "live-read" | "restart";
}

interface ReloadOutcome {
  at: string;
  changed: string[] | null;
  applied: string[] | null;
  failed: Record<string, string> | null;
  rejected_static: string[] | null;
}

interface ReloadStatus {
  groups: ReloadGroup[];
  last_reload: ReloadOutcome | null;
}

// ReloadStatusCard shows the hot/static matrix for config reload: which
// edits apply live, which need a restart, and what the last reload did.
export default function ReloadStatusCard() {
  const { t } = useI18n();
  const [data, setData] = useState<ReloadStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const d = await api.get<ReloadStatus>(paths.settings.reloadStatus);
      setData(d);
    } catch {
      toast.error(t("settings.reload_status_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { void load(); }, [load]);

  const last = data?.last_reload;
  const staticCount = data?.groups.filter((g) => g.mode === "static").length ?? 0;

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Zap} tone="primary" accent={false}
        title={t("settings.reload_status_title")}
        description={t("settings.reload_status_subtitle")} />
      <div className="p-(--card-spacing) space-y-3">
        {loading ? (
          <div className="py-6 text-center"><Spinner /></div>
        ) : !data ? (
          <p className="text-xs text-muted-foreground text-center py-4">{t("settings.reload_status_failed")}</p>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="success">{t("settings.reload_hot_count", { count: data.groups.length - staticCount })}</Badge>
              <Badge variant="warning"><RotateCcw className="size-3" />{t("settings.reload_static_count", { count: staticCount })}</Badge>
              <Button variant="ghost" size="sm" onClick={() => void load()} className="ml-auto gap-1.5">
                <RefreshCw className="size-3.5" />{t("common.refresh")}
              </Button>
            </div>
            {last ? (
              <div className="rounded-lg border border-border/60 bg-muted/30 p-3 text-xs space-y-1.5">
                <div className="text-muted-foreground">{t("settings.reload_last_at", { at: new Date(last.at).toLocaleString() })}</div>
                {(last.applied?.length ?? 0) > 0 && (
                  <div><span className="font-medium text-success">{t("settings.reload_applied")}: </span>
                    <span className="font-mono text-muted-foreground">{last.applied!.join(", ")}</span></div>
                )}
                {last.failed && Object.keys(last.failed).length > 0 && (
                  <div><span className="font-medium text-destructive">{t("settings.reload_failed")}: </span>
                    <span className="font-mono text-muted-foreground">
                      {Object.entries(last.failed).map(([k, v]) => `${k} (${v})`).join(", ")}
                    </span></div>
                )}
                {(last.rejected_static?.length ?? 0) > 0 && (
                  <div><span className="font-medium text-warning">{t("settings.reload_needs_restart")}: </span>
                    <span className="font-mono text-muted-foreground">{last.rejected_static!.join(", ")}</span></div>
                )}
                {(last.applied?.length ?? 0) === 0 && (!last.failed || Object.keys(last.failed).length === 0) && (
                  <div className="text-muted-foreground">{t("settings.reload_nothing")}</div>
                )}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">{t("settings.reload_never")}</p>
            )}
            <details className="text-xs">
              <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
                {t("settings.reload_matrix_toggle")}
              </summary>
              <div className="mt-2 max-h-56 overflow-y-auto rounded-lg border border-border/60">
                <table className="w-full text-xs">
                  <tbody>
                    {data.groups.map((g) => (
                      <tr key={g.token} className="border-b border-border/40 last:border-0">
                        <td className="px-2.5 py-1 font-mono text-muted-foreground">{g.token}</td>
                        <td className="px-2.5 py-1 text-right">
                          <Badge variant={g.mode === "hot" ? "success" : "secondary"}>
                            {g.mode === "hot" ? t("settings.reload_hot") : t("settings.reload_static")}
                          </Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </details>
          </>
        )}
      </div>
    </Card>
  );
}
