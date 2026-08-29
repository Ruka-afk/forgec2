"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { fetchAIStatus } from "@/app/(main)/agents/[id]/_components/AISuggestCard";
import { MessageSquareText, Search, Sparkles } from "lucide-react";

interface NLFilter {
  keywords?: string[];
  agent_hostname?: string;
  task_type?: string;
  status?: string;
  since_days?: number;
}

interface NLTaskRow {
  id: number;
  agent_id: string;
  type: string;
  command: string;
  status: string;
  result: string;
  created_at: string;
}

// Ask-the-history dialog: natural-language question -> LLM extracts a filter
// -> deterministic whitelist query. Shows the parsed filter as chips so the
// operator can see exactly how their question was interpreted.
function NLQueryDialog() {
  const { t } = useI18n();
  const [aiReady, setAiReady] = useState<boolean | null>(null);
  const [open, setOpen] = useState(false);
  const [question, setQuestion] = useState("");
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState<NLFilter | null>(null);
  const [rows, setRows] = useState<NLTaskRow[]>([]);

  if (aiReady === null) {
    void fetchAIStatus().then((st) => setAiReady(st.enabled));
    return null;
  }
  if (!aiReady) return null;

  const ask = async () => {
    const q = question.trim();
    if (!q || loading) return;
    setLoading(true);
    try {
      const d = await api.postJson<{ filter?: NLFilter; tasks?: NLTaskRow[] }>(paths.ai.nlQuery, { question: q });
      setFilter(d.filter || {});
      setRows(d.tasks || []);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.assist_failed"));
    } finally {
      setLoading(false);
    }
  };

  const chips: string[] = [];
  if (filter) {
    for (const kw of filter.keywords || []) chips.push(`"${kw}"`);
    if (filter.agent_hostname) chips.push(t("ai.nl_chip_host", { host: filter.agent_hostname }));
    if (filter.task_type) chips.push(`${t("tasks.col_type")}: ${filter.task_type}`);
    if (filter.status) chips.push(`${t("tasks.col_status")}: ${filter.status}`);
    chips.push(`${t("ai.nl_chip_days")}: ${filter.since_days ?? 30}`);
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) { setQuestion(""); setFilter(null); setRows([]); } }}>
      <Button variant="secondary" size="sm" onClick={() => setOpen(true)} className="gap-1.5">
        <MessageSquareText className="size-4 text-primary" />{t("ai.nl_button")}
      </Button>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="size-4 text-primary" />{t("ai.nl_title")}
          </DialogTitle>
        </DialogHeader>
        <div className="flex gap-2">
          <Input
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") ask(); }}
            placeholder={t("ai.nl_placeholder")}
            autoFocus
          />
          <Button onClick={ask} disabled={loading || !question.trim()} className="shrink-0 gap-1.5">
            {loading ? <Spinner size="xs" /> : <Search className="size-4" />}
          </Button>
        </div>

        {filter && (
          <div className="flex flex-wrap items-center gap-1.5 mt-2">
            <span className="text-xs text-muted-foreground">{t("ai.nl_parsed")}</span>
            {chips.map((c, i) => (
              <Badge key={i} variant="secondary" className="text-(--fs-micro-sm)">{c}</Badge>
            ))}
          </div>
        )}

        <div className="flex-1 overflow-y-auto mt-3 rounded-lg border border-border divide-y divide-border min-h-[120px]">
          {loading ? (
            <div className="py-10 text-center"><Spinner /></div>
          ) : rows.length === 0 ? (
            <p className="py-10 text-center text-sm text-muted-foreground">{filter ? t("ai.nl_no_results") : t("ai.nl_hint")}</p>
          ) : (
            rows.map((r) => (
              <div key={r.id} className="px-3 py-2 text-sm hover:bg-muted/50 transition-colors">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-mono text-xs text-muted-foreground">#{r.id}</span>
                  <Badge variant="outline" className="text-(--fs-micro-sm)">{r.type}</Badge>
                  <Badge variant="secondary" className="text-(--fs-micro-sm)">{r.status}</Badge>
                  <span className="text-xs text-muted-foreground truncate">{r.agent_id.slice(0, 12)}</span>
                </div>
                <code className="block truncate text-xs font-mono text-muted-foreground mt-0.5" title={r.command}>{r.command}</code>
              </div>
            ))
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>{t("common.close")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default NLQueryDialog;
