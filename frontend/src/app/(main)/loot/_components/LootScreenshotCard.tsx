"use client";

import { memo } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Download } from "lucide-react";
import type { Screenshot } from "@/types/screenshot";

interface LootScreenshotCardProps {
  screenshot: Screenshot;
  isSelected: boolean;
  onToggleSelect: (id: string) => void;
  onOpen: (path: string) => void;
}

function LootScreenshotCardInner({
  screenshot: s,
  isSelected,
  onToggleSelect,
  onOpen,
}: LootScreenshotCardProps) {
  return (
    <div role="button" tabIndex={0} className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted/50 ${isSelected ? "border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-800" : "border-border"}`}
      onClick={() => onOpen(`/screenshots/${s.path}`)}
      onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onOpen(`/screenshots/${s.path}`); } }}>
      <div role="button" tabIndex={0} className="absolute top-1.5 left-1.5 z-10" onClick={e => { e.stopPropagation(); onToggleSelect(s.id); }} onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); onToggleSelect(s.id); } }}>
        <Checkbox checked={isSelected} aria-label={`Select screenshot ${s.filename}`} className="bg-secondary/90" />
      </div>
      <img src={`/screenshots/${s.path}`} alt={s.filename} className="w-full h-24 object-contain bg-background" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
      <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-(--font-size-micro-sm) text-white px-2 py-1 opacity-0 group-hover:opacity-100 transition flex justify-between items-center">
        <span className="truncate">{s.agent_id.substring(0, 8)}</span>
        <a href={`/screenshots/${s.path}`} download onClick={e => e.stopPropagation()} aria-label="Download" className="hover:text-primary dark:hover:text-emerald-300 px-1 transition-colors"><Download aria-hidden="true" className="w-4 h-4" /></a>
      </div>
    </div>
  );
}

export const LootScreenshotCard = memo(LootScreenshotCardInner);
