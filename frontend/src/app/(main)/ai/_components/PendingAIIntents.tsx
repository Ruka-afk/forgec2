"use client";

import { useCallback, useState } from "react";
import { AlertTriangle, Check, Clock, ShieldCheck, X } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";

interface AIExecutionIntent {
  id: string;
  session_id: number;
  tool_name: string;
  risk: "low_risk_write" | "write" | "destructive" | "sensitive";
  required_permission?: string;
  target_summary: string;
  arguments_summary: string;
  status: string;
  created_at: string;
}

export function PendingAIIntents({ activeSessionId }: { activeSessionId: number | null }) {
  const { t } = useI18n();
  const [acting, setActing] = useState<Set<string>>(() => new Set());
  const fetchIntents = useCallback(async (signal?: AbortSignal) => {
    const payload = await api.get<unknown>(`${paths.ai.intents}?status=pending`, { signal });
    return normalizeListEnvelope(payload, ["intents", "data"]) as AIExecutionIntent[];
  }, []);
	const { data, refresh } = useApiResource({ fetcher: fetchIntents, pollMs: 5000 });
	const allIntents = data ?? [];
	const intents = activeSessionId == null ? allIntents : allIntents.filter((intent) => intent.session_id === activeSessionId);

  const decide = async (intent: AIExecutionIntent, approve: boolean) => {
    setActing((current) => new Set(current).add(intent.id));
    try {
      await api.postJson(approve ? paths.ai.intentApprove(intent.id) : paths.ai.intentReject(intent.id), {});
      toast.success(approve ? t("ai.intent_approved") : t("ai.intent_rejected"));
      await refresh();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("ai.intent_action_failed"));
    } finally {
      setActing((current) => {
        const next = new Set(current);
        next.delete(intent.id);
        return next;
      });
    }
  };

  if (intents.length === 0) return null;
  return (
    <Card className="border-warning/30 bg-warning/5 p-3" aria-live="polite">
      <div className="mb-2 flex items-center gap-2">
        <ShieldCheck className="size-4 text-warning" />
        <span className="text-sm font-semibold">{t("ai.intent_manual_approval")}</span>
        <Badge variant="warning" className="ml-auto">{intents.length}</Badge>
      </div>
      <div className="max-h-72 space-y-2 overflow-y-auto">
        {intents.map((intent) => (
          <div key={intent.id} className="rounded-xl border border-border bg-card p-3 shadow-xs">
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <Badge variant={intent.risk === "destructive" || intent.risk === "sensitive" ? "destructive" : "warning"}>
                <AlertTriangle className="size-3" /> {intent.risk}
              </Badge>
              <span className="font-mono font-medium">{intent.tool_name}</span>
              <span className="ml-auto flex items-center gap-1 text-muted-foreground"><Clock className="size-3" />{new Date(intent.created_at).toLocaleTimeString()}</span>
            </div>
            <pre className="mt-2 max-h-24 overflow-auto whitespace-pre-wrap break-all rounded-lg bg-muted/55 p-2 text-xs">{intent.arguments_summary || intent.target_summary}</pre>
            {intent.required_permission && <p className="mt-1 text-xs text-muted-foreground">{t("ai.intent_permission", { permission: intent.required_permission })}</p>}
            <div className="mt-2 grid grid-cols-2 gap-2">
              <Button size="xs" disabled={acting.has(intent.id)} onClick={() => void decide(intent, true)}><Check className="size-3" />{t("ai.intent_approve_execute")}</Button>
              <Button size="xs" variant="outline" disabled={acting.has(intent.id)} onClick={() => void decide(intent, false)}><X className="size-3" />{t("ai.reject")}</Button>
            </div>
          </div>
        ))}
      </div>
    </Card>
  );
}
