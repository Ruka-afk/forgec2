"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { Lightbulb, Copy, Play, Sparkles } from "lucide-react";

// Module-level AI status cache: one probe per browser session is enough and
// keeps every assist card cheap to mount.
let aiStatusCache: { enabled: boolean; hasApiKey: boolean } | null = null;

export async function fetchAIStatus(): Promise<{ enabled: boolean; hasApiKey: boolean }> {
  if (aiStatusCache) return aiStatusCache;
  try {
    const d = await api.get<{ enabled?: boolean; has_api_key?: boolean }>(paths.ai.status);
    aiStatusCache = { enabled: !!d.enabled && !!d.has_api_key, hasApiKey: !!d.has_api_key };
  } catch {
    aiStatusCache = { enabled: false, hasApiKey: false };
  }
  return aiStatusCache;
}

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
  const [aiReady, setAiReady] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  const [suggestions, setSuggestions] = useState<Suggestion[] | null>(null);
  const [executingIdx, setExecutingIdx] = useState<number>(-1);

  // Probe once; hide entirely when the AI subsystem is off (graceful degrade).
  useEffect(() => {
    let cancelled = false;
    fetchAIStatus().then((st) => { if (!cancelled) setAiReady(st.enabled); });
    return () => { cancelled = true; };
  }, []);
  if (aiReady === null) return null;
  if (!aiReady) return null;

  const loadSuggestions = async () => {
    setLoading(true);
    try {
      const d = await api.postJson<{ suggestions?: Suggestion[] }>(paths.ai.suggestNextSteps, {
        agent_id: agentId,
      });
      setSuggestions(d.suggestions || []);
      if (!d.suggestions?.length) toast.info(t("ai.suggest_empty"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.assist_failed"));
    } finally {
      setLoading(false);
    }
  };

  const runCommand = async (idx: number, cmd: string) => {
    setExecutingIdx(idx);
    try {
      await api.postJson(paths.agents.command(agentId), { command: cmd });
      toast.success(t("ai.suggest_dispatched"));
    } catch (e) {
      // 409 etc.: the command layer enforces its own approval semantics.
      toast.info(e instanceof Error ? e.message : t("ai.suggest_dispatched"));
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
                    <code className="mt-1.5 block truncate rounded bg-background px-1.5 py-1 text-(--fs-micro-sm) font-mono text-muted-foreground" title={sg.command_hint}>
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
                          onClick={() => runCommand(i, sg.command_hint)}
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
