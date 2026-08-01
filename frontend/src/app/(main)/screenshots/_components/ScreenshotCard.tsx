"use client";

import { memo } from "react";
import { Check, Download } from "lucide-react";
import type { Screenshot } from "@/types/screenshot";

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
  return (
    <div
      role="button" tabIndex={0}
      className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted transition-all hover:shadow-lg dark:hover:shadow-black/30 ${
        isSelected
          ? "border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-800"
          : "border-border"
      }`}
      onClick={() => onOpen(index)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpen(index); } }}
    >
      <div role="button" tabIndex={0} className="absolute top-1.5 left-1.5 z-10" onClick={e => { e.stopPropagation(); onToggleSelect(s.id); }} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); onToggleSelect(s.id); } }}>
        <div className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-colors ${
          isSelected
            ? "bg-indigo-500 border-indigo-500"
            : "bg-secondary/90 border-border"
        }`}>
          {isSelected && <Check className="w-4 h-4" />}
        </div>
      </div>
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
      <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-(--font-size-micro-sm) text-white px-2 py-1 opacity-0 group-hover:opacity-100 transition flex justify-between items-center">
        <span className="truncate">{s.agent_id.substring(0, 8)}</span>
        <a href={`/screenshots/${s.path}`} download onClick={e => e.stopPropagation()} aria-label="Download" className="hover:text-emerald-300 px-1 transition-colors">
          <Download aria-hidden="true" className="w-4 h-4" />
        </a>
      </div>
    </div>
  );
}

export const ScreenshotCard = memo(ScreenshotCardInner);
