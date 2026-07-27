"use client";

import { useEffect, useState } from "react";
import { useWS } from "@/lib/wsContext";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { ArrowUpCircle, X } from "lucide-react";

const DISMISS_KEY = "forgec2_update_dismissed";

export default function UpdateBanner({ currentVersion }: { currentVersion?: string }) {
  const { subscribe } = useWS();
  const { t } = useI18n();
  const [info, setInfo] = useState<{ latest: string; downloadUrl: string } | null>(null);
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
          downloadUrl: String(msg.download_url || "https://github.com/forgec2/forgec2/releases"),
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

  return (
    <div className="bg-gradient-to-r from-sky-500 to-primary text-white text-xs flex items-center justify-between px-4 sm:px-6 py-2 shrink-0 rounded-xl mx-4 sm:mx-6 lg:mx-8 mt-2 overflow-hidden">
      <div className="flex items-center gap-2 min-w-0">
        <ArrowUpCircle className="w-4 h-4 shrink-0" />
        <span className="truncate">
          Update available: <strong>{info.latest}</strong>
          {currentVersion && <span className="text-white/80"> (current: {currentVersion})</span>}
        </span>
      </div>
      <div className="flex items-center gap-3 shrink-0 ml-3">
        <a
          href={info.downloadUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-white/90 hover:text-white underline hidden sm:inline transition-colors"
        >
          Download
        </a>
        <Button variant="ghost" size="icon-sm" onClick={dismiss} className="text-white/70 hover:text-white" aria-label={t("common.dismiss")}>
          <X className="w-4 h-4" />
        </Button>
      </div>
    </div>
  );
}
