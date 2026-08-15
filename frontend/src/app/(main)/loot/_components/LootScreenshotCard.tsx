"use client";

import { memo } from "react";
import { Checkbox } from "@/components/ui/checkbox";
import { Button } from "@/components/ui/button";
import { Download } from "lucide-react";
import type { Screenshot } from "@/types/screenshot";
import { useI18n } from "@/lib/i18n";

interface LootScreenshotCardProps {
  screenshot: Screenshot;
  index: number;
  isSelected: boolean;
  onToggleSelect: (id: string) => void;
  onOpen: (index: number) => void;
}

function LootScreenshotCardInner({
  screenshot: s,
  index,
  isSelected,
  onToggleSelect,
  onOpen,
}: LootScreenshotCardProps) {
  const { t } = useI18n();
  return (
    <div className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted/50 ${isSelected ? "border-primary ring-2 ring-primary/25 dark:ring-primary/40" : "border-border"}`}>
      <Button
        type="button"
        variant="ghost"
        onClick={() => onOpen(index)}
        aria-label={`Open ${s.filename}`}
        className="block h-auto w-full rounded-none p-0"
      >
        <img src={`/screenshots/${s.path}`} alt={s.filename} className="w-full h-24 object-contain bg-background" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
      </Button>
      <div className="absolute top-1.5 left-1.5 z-10">
        <Checkbox checked={isSelected} onCheckedChange={() => onToggleSelect(s.id)} aria-label={`Select screenshot ${s.filename}`} className="bg-secondary/90" />
      </div>
      <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-(--fs-micro-sm) text-white px-2 py-1 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition flex justify-between items-center">
        <span className="truncate">{s.agent_id.substring(0, 8)}</span>
        <a href={`/screenshots/${s.path}`} download aria-label={t("common.download")} className="hover:text-primary dark:hover:text-chart-1 px-1 transition-colors"><Download aria-hidden="true" className="w-4 h-4" /></a>
      </div>
    </div>
  );
}

export const LootScreenshotCard = memo(LootScreenshotCardInner);
