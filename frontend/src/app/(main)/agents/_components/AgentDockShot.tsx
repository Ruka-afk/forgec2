"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { safeImageSrc } from "@/lib/safeUrl";
import { screenshotDataUrl } from "./dock-shot";

export function AgentDockShot({ agentId, refreshKey }: { agentId: string; refreshKey: number }) {
  const { t } = useI18n();
  const [src, setSrc] = useState("");

  useEffect(() => {
    if (!agentId) return;
    const ac = new AbortController();
    api.get(paths.agents.screenshot(agentId), { signal: ac.signal })
      .then((data) => {
        if (ac.signal.aborted) return;
        setSrc(screenshotDataUrl(data));
      })
      .catch(() => {
        if (!ac.signal.aborted) setSrc("");
      });
    return () => ac.abort();
  }, [agentId, refreshKey]);

  if (!src) return null;

  return (
    <div className="border-b border-border bg-muted/30 px-2 py-1">
        <img
        src={safeImageSrc(src) ?? ""}
        alt={t("agents.dock_shot_alt")}
        className="max-h-28 max-w-full rounded object-contain"
      />
    </div>
  );
}
