import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { safeImageSrc } from "@/lib/safeUrl";
import { Button } from "@/components/ui/button";
import { AlertTriangle } from "lucide-react";
import { screenshotDataUrl } from "./dock-shot";

export function AgentDockShot({ agentId, refreshKey }: { agentId: string; refreshKey: number }) {
  const { t } = useI18n();
  const [src, setSrc] = useState("");
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!agentId) return;
    const ac = new AbortController();
    api.get(paths.agents.screenshot(agentId), { signal: ac.signal })
      .then((data) => {
        if (ac.signal.aborted) return;
        // A successful read that yields no usable image is a real "no
        // screenshot", which stays hidden. A rejected read is tracked
        // separately so it is not mistaken for the same thing.
        setSrc(screenshotDataUrl(data));
        setFailed(false);
      })
      .catch(() => {
        // Keep the last good frame: clearing it left a blank gap that looked
        // like a deliberate choice rather than a failed request.
        if (!ac.signal.aborted) setFailed(true);
      });
    return () => ac.abort();
  }, [agentId, refreshKey, attempt]);

  if (!src) {
    if (!failed) return null;
    return (
      <div role="alert" className="flex flex-wrap items-center gap-2 border-b border-border bg-warning/10 px-2 py-1.5 text-(--fs-micro-sm) text-warning-foreground">
        <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="min-w-0 flex-1">{t("agents.dock_shot_failed")}</span>
        <Button onClick={() => setAttempt((n) => n + 1)} size="xs" variant="outline">{t("common.try_again")}</Button>
      </div>
    );
  }

  return (
    <div className="border-b border-border bg-muted/30 px-2 py-1">
      {failed && (
        <div role="alert" className="mb-1 flex items-center gap-1.5 text-(--fs-micro-sm) text-warning-foreground">
          <AlertTriangle className="size-3 shrink-0" aria-hidden="true" />
          <span>{t("agents.dock_shot_stale")}</span>
        </div>
      )}
      <img
        src={safeImageSrc(src) ?? ""}
        alt={t("agents.dock_shot_alt")}
        className="max-h-28 max-w-full rounded object-contain"
      />
    </div>
  );
}
