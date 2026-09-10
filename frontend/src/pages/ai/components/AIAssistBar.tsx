import { useEffect, useState } from "react";
import { BookOpen, ClipboardList, LoaderCircle } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { normalizeListEnvelope } from "@/lib/envelope";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

interface PlaybookMacro {
  id: number;
  name: string;
  description?: string;
}

interface RunReview {
  run_id: string;
  run_status: string;
  summary: string;
  what_worked: string[];
  gaps: string[];
  next_steps: string[];
}

interface AIAssistBarProps {
  sessionId: number | null;
  disabled?: boolean;
  onInsertPrompt: (prompt: string) => void;
}

// Assist bar above the composer: pick a saved playbook template into the
// input, or debrief the session's latest finished run.
export function AIAssistBar({ sessionId, disabled, onInsertPrompt }: AIAssistBarProps) {
  const { t } = useI18n();
  const [macros, setMacros] = useState<PlaybookMacro[]>([]);
  const [review, setReview] = useState<RunReview | null>(null);
  const [reviewOpen, setReviewOpen] = useState(false);
  const [reviewing, setReviewing] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const payload = await api.get<unknown>(paths.macros.list);
        const list = normalizeListEnvelope(payload, ["macros", "data"]) as PlaybookMacro[];
        if (!cancelled) setMacros(Array.isArray(list) ? list : []);
      } catch {
        if (!cancelled) setMacros([]);
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const handleReview = async () => {
    if (sessionId == null) return;
    setReviewing(true);
    try {
      const data = await api.postJson<{ review?: RunReview }>(paths.ai.runReview, { session_id: sessionId });
      if (!data.review) throw new Error("empty review");
      setReview(data.review);
      setReviewOpen(true);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("ai.review_failed"));
    } finally {
      setReviewing(false);
    }
  };

  return (
    <>
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Select
          disabled={disabled || macros.length === 0}
          onValueChange={(v) => {
            const macro = macros.find((m) => String(m.id) === v);
            if (macro) onInsertPrompt(t("ai.playbook_prompt", { name: macro.name, id: macro.id }));
          }}
        >
          <SelectTrigger className="h-8 w-52 text-xs" aria-label={t("ai.playbook_picker")}>
            <BookOpen className="size-3.5" />
            <SelectValue placeholder={macros.length === 0 ? t("ai.playbook_empty") : t("ai.playbook_picker")} />
          </SelectTrigger>
          <SelectContent>
            {macros.map((m) => (
              <SelectItem key={m.id} value={String(m.id)}>{m.name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="outline" size="sm" disabled={disabled || sessionId == null || reviewing} onClick={() => void handleReview()} className="gap-1.5">
          {reviewing ? <LoaderCircle className="size-3.5 animate-spin" /> : <ClipboardList className="size-3.5" />}
          {t("ai.review_run")}
        </Button>
      </div>
      <Dialog open={reviewOpen} onOpenChange={setReviewOpen}>
        <DialogContent className="sm:max-w-lg" aria-live="polite">
          <DialogHeader>
            <DialogTitle>{t("ai.review_title")}</DialogTitle>
            <DialogDescription>
              {review ? `${review.run_status} · ${review.run_id.slice(0, 8)}` : ""}
            </DialogDescription>
          </DialogHeader>
          {review && (
            <div className="max-h-96 space-y-3 overflow-y-auto text-sm">
              <p className="leading-6">{review.summary}</p>
              {review.what_worked.length > 0 && (
                <div>
                  <p className="mb-1 font-semibold">{t("ai.review_worked")}</p>
                  <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                    {review.what_worked.map((item, i) => <li key={i}>{item}</li>)}
                  </ul>
                </div>
              )}
              {review.gaps.length > 0 && (
                <div>
                  <p className="mb-1 font-semibold">{t("ai.review_gaps")}</p>
                  <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                    {review.gaps.map((item, i) => <li key={i}>{item}</li>)}
                  </ul>
                </div>
              )}
              {review.next_steps.length > 0 && (
                <div>
                  <p className="mb-1 font-semibold">{t("ai.review_next")}</p>
                  <ul className="list-disc space-y-1 pl-5 text-muted-foreground">
                    {review.next_steps.map((item, i) => <li key={i}>{item}</li>)}
                  </ul>
                </div>
              )}
            </div>
          )}
          <DialogFooter showCloseButton />
        </DialogContent>
      </Dialog>
    </>
  );
}
