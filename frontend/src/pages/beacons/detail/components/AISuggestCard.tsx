import { useEffect, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Lightbulb, Copy, Play, Sparkles } from "lucide-react";
import { useAIStatus } from "@/lib/hooks/useAIStatus";
import { useMutation } from "@/lib/hooks/useMutation";
import { useErrorToast } from "@/lib/hooks/useErrorToast";

interface Suggestion {
  action: string;
  reason: string;
  risk: "low" | "medium" | "high";
  command_hint: string;
}

const RISK_VARIANT: Record<string, "success" | "warning" | "destructive"> = {
  low: "success",
  medium: "warning",
  high: "destructive",
};

export function AISuggestCard({ agentId, online }: { agentId: string; online: boolean }) {
  const { t } = useI18n();
  const { status } = useAIStatus();
  const { toastError, warnError } = useErrorToast();
  const [suggestions, setSuggestions] = useState<Suggestion[] | null>(null);
  const [executingIdx, setExecutingIdx] = useState<number>(-1);

  const { mutate: loadSuggestions, isPending: loading, reset: resetLoad } = useMutation({
    fn: () => api.postJson<{ suggestions?: Suggestion[] }>(paths.ai.suggestNextSteps, {
      agent_id: agentId,
    }),
    onSuccess: (d) => {
      setSuggestions(d.suggestions || []);
      if (!d.suggestions?.length) toast.info(t("ai.suggest_empty"));
    },
    onError: (e) => toastError(e, "ai.toast.assist_failed"),
  });

  const { mutate: runCommand, reset: resetRun } = useMutation({
    fn: (cmd: string) => api.postJson(paths.agents.command(agentId), { command: cmd }),
    onSuccess: () => toast.success(t("ai.suggest_dispatched")),
    onError: (e) => {
      // 409: the command layer enforces its own approval semantics; that is
      // an outcome, not a retryable failure (same treatment as the header
      // quick-action in BeaconDetailPage). `warnError` keeps the warning tone
      // while resolving the text through the shared error rules.
      if (e instanceof ApiError && e.status === 409) warnError(e, "ai.toast.assist_failed");
      else toastError(e, "ai.toast.assist_failed");
    },
  });

  // Route reuse: /beacons/A → /beacons/B keeps this card mounted. Suggestions,
  // the per-row spinner and in-flight completions must not carry over.
  useEffect(() => {
    setSuggestions(null);
    setExecutingIdx(-1);
    resetLoad();
    resetRun();
  }, [agentId, resetLoad, resetRun]);

  if (status === null) return null;
  // A genuinely disabled AI subsystem hides this card; a failed status query
  // must not, so the feature stays visible with an explicit unknown state.
  if (status.known && !status.enabled) return null;

  const handleRun = async (idx: number, cmd: string) => {
    setExecutingIdx(idx);
    try {
      await runCommand(cmd);
    } finally {
      setExecutingIdx(-1);
    }
  };

  return (
    <Card className="p-4 mb-4">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-semibold text-foreground flex items-center gap-2">
          <Sparkles className="size-4 text-primary" />{t("ai.suggest_title")}
        </span>
        <Button size="sm" variant="secondary" onClick={loadSuggestions} disabled={loading}>
          {loading ? <Spinner size="xs" /> : <Lightbulb className="size-4" />}
          {suggestions ? t("ai.suggest_refresh") : t("ai.suggest_load")}
        </Button>
      </div>

      {suggestions && suggestions.length > 0 && (
        <ul className="mt-3 space-y-2">
          {suggestions.map((sg, i) => (
            <li key={`${i}-${sg.action}`} className="rounded-lg border border-border bg-muted/40 p-3">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Badge variant={RISK_VARIANT[sg.risk] || "warning"} className="shrink-0">
                      {t(`ai.risk_${sg.risk}`)}
                    </Badge>
                    <span className="text-sm font-medium text-foreground">{sg.action}</span>
                  </div>
                  <p className="mt-1 text-xs text-muted-foreground">{sg.reason}</p>
                  {sg.command_hint && (
                    <code className="mt-1.5 block truncate rounded bg-background px-1.5 py-1 text-micro-sm font-mono text-muted-foreground" title={sg.command_hint}>
                      {sg.command_hint}
                    </code>
                  )}
                </div>
                <div className="flex shrink-0 flex-col gap-1">
                  {sg.command_hint && (
                    <>
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label={t("common.copy")}
                        onClick={() => { navigator.clipboard.writeText(sg.command_hint); toast.success(t("ai.copied")); }}
                      >
                        <Copy className="size-3.5" />
                      </Button>
                      {online && (
                        <Button
                          variant="ghost"
                          size="icon-xs"
                          aria-label={t("ai.suggest_run")}
                          disabled={executingIdx === i}
                          onClick={() => handleRun(i, sg.command_hint)}
                          className="text-primary"
                        >
                          {executingIdx === i ? <Spinner size="xs" /> : <Play className="size-3.5" />}
                        </Button>
                      )}
                    </>
                  )}
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {suggestions !== null && suggestions.length === 0 && !loading && (
        <p className="mt-3 text-xs text-muted-foreground">{t("ai.suggest_empty")}</p>
      )}
    </Card>
  );
}

export default AISuggestCard;
