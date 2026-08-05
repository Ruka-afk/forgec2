"use client";

import { memo } from "react";
import { Check, Download } from "lucide-react";
import type { Screenshot } from "@/types/screenshot";
import { useI18n } from "@/lib/i18n";

interface ScreenshotCardProps {
  screenshot: Screenshot;
  isSelected: boolean;
  index: number;
  onToggleSelect: (id: string) => void;
  onOpen: (index: number) => void;
  onResolution?: (id: string, w: number, h: number) => void;
}

function ScreenshotCardInner({
  screenshot: s,
  isSelected,
  index,
  onToggleSelect,
  onOpen,
  onResolution,
}: ScreenshotCardProps) {
  const { t } = useI18n();
  return (
    <div
      className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted transition-all hover:shadow-lg dark:hover:shadow-black/30 ${
        isSelected
          ? "border-primary ring-2 ring-primary/25 dark:ring-primary/40"
          : "border-border"
      }`}
    >
      <button
        type="button"
        onClick={() => onOpen(index)}
        aria-label={`Open ${s.filename}`}
        className="block w-full focus-visible:outline-none"
      >
        <img
          src={`/screenshots/${s.path}`}
          alt={s.filename}
          className="w-full h-28 object-contain bg-card"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
          onLoad={e => {
            const img = e.currentTarget;
            if (img.naturalWidth && onResolution) onResolution(s.id, img.naturalWidth, img.naturalHeight);
          }}
        />
      </button>
      <div className="absolute top-1.5 left-1.5 z-10">
        <button
          type="button"
          onClick={() => onToggleSelect(s.id)}
          aria-label={`Select ${s.filename}`}
          aria-pressed={isSelected}
          className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-colors ${
            isSelected
              ? "bg-primary/100 border-primary"
              : "bg-secondary/90 border-border"
          }`}
        >
          {isSelected && <Check className="w-4 h-4" aria-hidden="true" />}
        </button>
      </div>
      <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-(--fs-micro-sm) text-white px-2 py-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition flex justify-between items-center">
        <span className="truncate">{s.agent_id.substring(0, 8)}</span>
        <a href={`/screenshots/${s.path}`} download aria-label={t("common.download")} className="hover:text-emerald-300 px-1 transition-colors">
          <Download aria-hidden="true" className="w-4 h-4" />
        </a>
      </div>
    </div>
  );
}

export const ScreenshotCard = memo(ScreenshotCardInner);
