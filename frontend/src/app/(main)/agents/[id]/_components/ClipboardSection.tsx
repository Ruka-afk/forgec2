"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { CopyButton } from "@/components/ui/copy-button";
import { Clipboard, ClipboardPaste, Send } from "lucide-react";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import CollectCard from "./CollectCard";
import { useCollectTask } from "./useCollectTask";

interface ClipboardSectionProps {
  agentId: string;
  online: boolean;
}

interface ClipEntry {
  text: string;
  time: string;
}

const HISTORY_CAP = 20;

export default memo(function ClipboardSection({ agentId, online }: ClipboardSectionProps) {
  const { t } = useI18n();
  const { busy, collect, fire } = useCollectTask(agentId);
  const [draft, setDraft] = useState("");
  const [history, setHistory] = useState<ClipEntry[]>([]);

  const handleGet = async () => {
    const text = await collect("get", paths.agents.clipboardGet(agentId), {
      storeResult: false,
      successText: t("agents.clipboard_got"),
      errorText: t("agents.clipboard_failed"),
    });
    if (text !== null) {
      setHistory((prev) => [{ text, time: new Date().toLocaleString() }, ...prev].slice(0, HISTORY_CAP));
    }
  };

  const handleSet = async () => {
    if (!draft.trim()) return;
    const ok = await fire("set", paths.agents.clipboardSet(agentId), { data: draft }, t("agents.clipboard_set_done"));
    if (ok) setDraft("");
  };

  return (
    <CollectCard
      title={t("agents.clipboard_title")}
      icon={<Clipboard className="size-3.5" />}
      headerRight={
        <Button size="sm" disabled={!online || busy !== null} onClick={() => void handleGet()}>
          {busy === "get" ? (
            <>
              <Spinner size="xs" /> {t("agents.clipboard_getting")}
            </>
          ) : (
            <>
              <ClipboardPaste className="size-4" /> {t("agents.clipboard_get")}
            </>
          )}
        </Button>
      }
      emptyIcon={Clipboard}
      emptyTitle={t("agents.clipboard_empty")}
      emptyHint={t("agents.clipboard_empty_hint")}
      result={null}
      resultOverride={
        history.length === 0 ? (
          <EmptyState icon={Clipboard} title={t("agents.clipboard_empty")} message={t("agents.clipboard_empty_hint")} />
        ) : (
          <div className="max-h-64 space-y-2 overflow-auto">
            {history.map((h, i) => (
              <div key={`${h.time}-${i}`} className="rounded-lg border border-border bg-muted/50 p-2.5">
                <div className="mb-1 flex items-center justify-between gap-2">
                  <span className="font-mono text-xs text-muted-foreground">{h.time}</span>
                  <CopyButton text={h.text} label={t("agents.clipboard_title")} size="xs" />
                </div>
                <pre className="whitespace-pre-wrap break-words font-mono text-xs">{h.text || t("agents.clipboard_empty_text")}</pre>
              </div>
            ))}
          </div>
        )
      }
    >
      <div className="flex gap-2">
        <Textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          placeholder={t("agents.clipboard_set_placeholder")}
          aria-label={t("agents.clipboard_set_placeholder")}
          className="min-h-16 flex-1 font-mono text-xs"
        />
        <Button
          size="sm"
          variant="outline"
          disabled={!online || busy !== null || !draft.trim()}
          onClick={() => void handleSet()}
          className="shrink-0 self-end"
        >
          {busy === "set" ? <Spinner size="xs" /> : <Send className="size-4" />}
          {t("agents.clipboard_set")}
        </Button>
      </div>
    </CollectCard>
  );
});
