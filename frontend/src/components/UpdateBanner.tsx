"use client";

import { useEffect, useState } from "react";
import { useWS } from "@/lib/wsContext";

const DISMISS_KEY = "forgec2_update_dismissed";

export default function UpdateBanner({ currentVersion }: { currentVersion?: string }) {
  const { subscribe } = useWS();
  const [info, setInfo] = useState<{ latest: string; downloadUrl: string } | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    try {
      const d = localStorage.getItem(DISMISS_KEY);
      if (d) setDismissed(true);
    } catch {
      /* ignore */
    }
  }, []);

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
      /* ignore */
    }
  };

  return (
    <div className="bg-gradient-to-r from-sky-500 to-indigo-600 text-white text-xs flex items-center justify-between px-6 py-2 shrink-0">
      <div className="flex items-center gap-2">
        <i className="fa-solid fa-circle-up text-sm"></i>
        <span>
          Update available: <strong>{info.latest}</strong>
          {currentVersion && <span className="text-white/80"> (current: {currentVersion})</span>}
        </span>
      </div>
      <div className="flex items-center gap-3">
        <a
          href={info.downloadUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-white/90 hover:text-white underline"
        >
          Download
        </a>
        <button onClick={dismiss} className="text-white/70 hover:text-white" aria-label="Dismiss">
          <i className="fa-solid fa-xmark"></i>
        </button>
      </div>
    </div>
  );
}