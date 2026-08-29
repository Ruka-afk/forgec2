"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import { fetchAIStatus } from "@/app/(main)/agents/[id]/_components/AISuggestCard";
import { Sparkles, AlertTriangle, ShieldAlert, Globe, FolderOpen, KeyRound, Info, ArrowRight } from "lucide-react";

interface Analysis {
  summary: string;
  highlights: Array<{ kind: string; severity: string; text: string }>;
  next_steps: string[];
}

const KIND_ICON: Record<string, React.ReactNode> = {
  priv: <ShieldAlert className="size-3.5 text-warning" />,
  av: <AlertTriangle className="size-3.5 text-destructive" />,
  network: <Globe className="size-3.5 text-info" />,
  path: <FolderOpen className="size-3.5 text-muted-foreground" />,
  cred: <KeyRound className="size-3.5 text-destructive" />,
  other: <Info className="size-3.5 text-muted-foreground" />,
};

const SEVERITY_CLASS: Record<string, string> = {
  high: "border-destructive/30 bg-destructive/10",
  medium: "border-warning/30 bg-warning/10",
  low: "border-border bg-background",
  info: "border-border bg-background",
};

// Inline AI analysis for one task result. Hidden entirely when the AI
// subsystem is disabled (graceful degradation contract).
function AIAnalysisButton({ taskId }: { taskId: number }) {
  const { t } = useI18n();
  const [aiReady, setAiReady] = useState<boolean | null>(null);
  const [loading, setLoading] = useState(false);
  const [analysis, setAnalysis] = useState<Analysis | null>(null);

  if (aiReady === null) {
    void fetchAIStatus().then((st) => setAiReady(st.enabled));
    return null;
  }
  if (!aiReady) return null;

  const run = async () => {
    setLoading(true);
    try {
      const d = await api.postJson<{ analysis?: Analysis }>(paths.ai.analyzeResult, { task_id: taskId });
      if (d.analysis?.summary) {
        setAnalysis(d.analysis);
      } else {
        toast.error(t("ai.toast.assist_failed"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.assist_failed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mt-2">
      {!analysis && (
        <Button size="xs" variant="secondary" onClick={run} disabled={loading} className="gap-1.5">
          {loading ? <Spinner size="xs" /> : <Sparkles className="size-3.5 text-primary" />}
          {t("ai.analyze_button")}
        </Button>
      )}
      {analysis && (
        <div className="rounded-lg border border-primary/20 bg-primary/5 p-3 space-y-2">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-foreground flex items-center gap-1.5">
              <Sparkles className="size-3.5 text-primary" />{t("ai.analyze_title")}
            </span>
            <Button variant="ghost" size="xs" onClick={run} disabled={loading} className="h-6 px-2 text-xs">
              {loading ? <Spinner size="xs" /> : t("ai.suggest_refresh")}
            </Button>
          </div>
          <p className="text-xs text-foreground whitespace-pre-wrap">{analysis.summary}</p>
          {analysis.highlights?.length > 0 && (
            <ul className="space-y-1.5">
              {analysis.highlights.map((h, i) => (
                <li key={i} className={`flex items-start gap-2 rounded-md border px-2 py-1.5 ${SEVERITY_CLASS[h.severity] || SEVERITY_CLASS.info}`}>
                  <span className="mt-0.5 shrink-0">{KIND_ICON[h.kind] || KIND_ICON.other}</span>
                  <span className="text-xs text-foreground">{h.text}</span>
                  <Badge variant="secondary" className="ml-auto shrink-0 text-(--fs-micro-sm)">{h.severity}</Badge>
                </li>
              ))}
            </ul>
          )}
          {analysis.next_steps?.length > 0 && (
            <div>
              <p className="text-xs font-medium text-muted-foreground mb-1">{t("ai.next_steps")}</p>
              <ul className="space-y-1">
                {analysis.next_steps.map((s, i) => (
                  <li key={i} className="flex items-start gap-1.5 text-xs text-muted-foreground">
                    <ArrowRight className="size-3 mt-0.5 shrink-0 text-primary" />{s}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default AIAnalysisButton;
