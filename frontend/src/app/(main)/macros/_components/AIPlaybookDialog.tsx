"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentListCached } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { fetchAIStatus } from "@/app/(main)/agents/[id]/_components/AISuggestCard";
import { Sparkles, Wand2, Save } from "lucide-react";

interface PBStep {
  command: string;
  shell: string;
  rationale?: string;
}

// AI playbook drafter: goal (+optional target agent) -> ordered macro steps.
// Draft only; saving creates a CommandMacro owned by "ai" which then runs
// through the ordinary macro runner and its approval semantics.
export function AIPlaybookDialog({ onSaved }: { onSaved: () => void }) {
  const { t } = useI18n();
  const [aiReady, setAiReady] = useState<boolean | null>(null);
  const [open, setOpen] = useState(false);
  const [goal, setGoal] = useState("");
  const [agentId, setAgentId] = useState("");
  const [agents, setAgents] = useState<Array<{ id: string; hostname?: string }>>([]);
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [draft, setDraft] = useState<{ name: string; description: string; steps: PBStep[] } | null>(null);

  if (aiReady === null) {
    void fetchAIStatus().then((st) => setAiReady(st.enabled));
    return null;
  }
  if (!aiReady) return null;

  const openDialog = async () => {
    setOpen(true);
    try {
      const list = await fetchAgentListCached();
      setAgents(list.map((a) => ({ id: String(a.id || ""), hostname: a.hostname })).slice(0, 100));
    } catch {
      setAgents([]);
    }
  };

  const generate = async () => {
    if (!goal.trim() || generating) return;
    setGenerating(true);
    setDraft(null);
    try {
      const d = await api.postJson<{ name: string; description: string; steps: PBStep[] }>(
        paths.ai.generatePlaybook,
        { goal: goal.trim(), agent_id: agentId },
      );
      if (d.steps?.length) {
        setDraft(d);
      } else {
        toast.error(t("ai.pb_generate_failed"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pb_generate_failed"));
    } finally {
      setGenerating(false);
    }
  };

  const save = async () => {
    if (!draft || saving) return;
    setSaving(true);
    try {
      const d = await api.postJson<{ id: number; name: string; steps: number }>(paths.ai.savePlaybook, draft);
      toast.success(t("ai.pb_saved", { name: d.name }));
      setOpen(false);
      setDraft(null);
      setGoal("");
      onSaved();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.pb_save_failed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Button variant="secondary" onClick={openDialog} className="gap-1.5">
        <Sparkles className="size-4 text-primary" />{t("ai.pb_button")}
      </Button>

      <Dialog open={open} onOpenChange={(o) => { setOpen(o); if (!o) { setDraft(null); setGoal(""); setGenerating(false); } }}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Wand2 className="size-4 text-primary" />{t("ai.pb_title")}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-3">
          <div>
            <Label className="text-xs text-muted-foreground">{t("ai.pb_goal")}</Label>
            <Input
              value={goal}
              onChange={(e) => setGoal(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter" && !draft) generate(); }}
              placeholder={t("ai.pb_goal_placeholder")}
              autoFocus
            />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">{t("ai.pb_agent")}</Label>
            <select
              value={agentId}
              onChange={(e) => setAgentId(e.target.value)}
              className="mt-1 w-full h-9 rounded-lg border border-border bg-transparent px-2 text-sm"
            >
              <option value="">{t("ai.pb_agent_none")}</option>
              {agents.map((a) => (
                <option key={a.id} value={a.id}>{a.hostname || a.id.slice(0, 12)}</option>
              ))}
            </select>
          </div>

          {!draft && (
            <Button onClick={generate} disabled={generating || !goal.trim()} className="w-full gap-1.5">
              {generating ? <Spinner size="xs" /> : <Sparkles className="size-4" />}
              {generating ? t("ai.pb_generating") : t("ai.pb_generate_btn")}
            </Button>
          )}

          {draft && (
            <div className="rounded-lg border border-primary/20 bg-primary/5 p-3 space-y-2 max-h-[40vh] overflow-y-auto">
              <div className="flex items-center justify-between">
                <span className="text-sm font-semibold text-foreground">{draft.name}</span>
                <Badge variant="secondary">{draft.steps.length} {t("ai.pb_steps_unit")}</Badge>
              </div>
              {draft.description && (
                <p className="text-xs text-muted-foreground">{draft.description}</p>
              )}
              <ol className="space-y-1.5 mt-1">
                {draft.steps.map((st, i) => (
                  <li key={i} className="rounded-md border border-border bg-background px-2 py-1.5">
                    <div className="flex items-center gap-2">
                      <span className="size-5 shrink-0 rounded-full bg-primary/15 text-(--fs-micro-sm) font-semibold text-primary flex items-center justify-center">{i + 1}</span>
                      <code className="flex-1 text-xs font-mono text-foreground break-all">{st.command}</code>
                      <Badge variant="outline" className="shrink-0 text-(--fs-micro-sm)">{st.shell}</Badge>
                    </div>
                    {st.rationale && (
                      <p className="mt-1 ml-7 text-(--fs-micro-sm) text-muted-foreground">{st.rationale}</p>
                    )}
                  </li>
                ))}
              </ol>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>{t("common.close")}</Button>
          {draft && (
            <Button onClick={save} disabled={saving} className="gap-1.5">
              {saving ? <Spinner size="xs" /> : <Save className="size-4" />}
              {t("ai.pb_save_btn")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
    </>
  );
}

export default AIPlaybookDialog;
