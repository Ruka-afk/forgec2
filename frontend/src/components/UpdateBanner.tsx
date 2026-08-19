"use client";

import { useEffect, useState } from "react";
import { useWS } from "@/lib/wsContext";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { ArrowUpCircle, X } from "lucide-react";
import { safeHref } from "@/lib/safeUrl";
import { bannerSurface } from "@/components/ui/banner";
import { cn } from "@/lib/utils";

const DISMISS_KEY = "forgec2_update_dismissed";

export default function UpdateBanner({ currentVersion }: { currentVersion?: string }) {
  const { subscribe } = useWS();
  const { t } = useI18n();
  const [info, setInfo] = useState<{ latest: string; downloadUrl?: string } | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    try {
      const d = localStorage.getItem(DISMISS_KEY);
      if (d) Promise.resolve().then(() => setDismissed(true));
    } catch {
      toast.error(t("update_banner.toast.storage_failed"));
    }
  }, [t]);

  useEffect(() => {
    return subscribe((msg) => {
      if (msg.type === "update_available") {
        const latest = String(msg.latest || "");
        const dismissedVer = localStorage.getItem(DISMISS_KEY);
        if (dismissedVer === latest) return;
        setInfo({
          latest,
          downloadUrl: safeHref(msg.download_url),
        });
        setDismissed(false);
      }
    });
  }, [subscribe]);

  if (!info || dismissed) return null;

  const dismiss = () => {
    setDismissed(true);
    try {
      localStorage.setItem(DISMISS_KEY, info.latest);
    } catch {
      toast.error(t("update_banner.toast.storage_failed"));
    }
  };

  const downloadHref = info.downloadUrl ?? "https://github.com/forgec2/forgec2/releases";

  return (
    <div className={cn(bannerSurface("info"), "flex items-center justify-between gap-3 px-4 sm:px-5 py-2.5 mx-2 sm:mx-3 mb-2 animate-fade-in")} aria-live="polite">
      <div className="flex items-center gap-2 min-w-0">
        <ArrowUpCircle className="w-4 h-4 text-info shrink-0" />
        <span className="mono-cell text-(--fs-compact) text-foreground/90 truncate">
          {t("update_banner.available")}: <strong className="font-mono">{info.latest}</strong>
          {currentVersion && <span className="text-muted-foreground/70"> {t("update_banner.current", { current: currentVersion })}</span>}
        </span>
      </div>
      <div className="flex items-center gap-3 shrink-0 ml-3">
        <a
          href={downloadHref}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-info-foreground font-medium underline-offset-2 hover:underline transition-colors hidden sm:inline"
        >
          {t("update_banner.download")}
        </a>
        <Button variant="ghost" size="icon-sm" onClick={dismiss} className="text-muted-foreground hover:text-foreground" aria-label={t("common.dismiss")}>
          <X className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );
}
