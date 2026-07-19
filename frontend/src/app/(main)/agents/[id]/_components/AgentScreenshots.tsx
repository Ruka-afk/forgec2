"use client";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import Link from "next/link";
import { Camera, ChevronLeft, ChevronRight, Maximize2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export interface AgentScreenshotsProps {
  screenshots: string[];
  agentId: string;
  lightboxIdx: number | null;
  onOpenLightbox: (idx: number) => void;
  onCloseLightbox: () => void;
  onPrevLightbox: () => void;
  onNextLightbox: () => void;
}

export default function AgentScreenshots({
  screenshots, agentId, lightboxIdx, onOpenLightbox,
  onCloseLightbox, onPrevLightbox, onNextLightbox,
}: AgentScreenshotsProps) {
  const { t } = useI18n();
  if (screenshots.length === 0) return null;

  const lightboxOpen = lightboxIdx !== null;

  return (
    <>
      <Card className="mb-4 gap-0">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h3 className="text-sm font-semibold text-foreground"><Camera className="w-4 h-4" />{t("agents.screenshots_title")} <span className="ml-1.5 text-muted-foreground/70 font-normal">({screenshots.length})</span></h3>
          <Link href={`/screenshots?agent_id=${agentId}`} className="text-xs text-indigo-600 dark:text-indigo-400 hover:underline">{t("agents.screenshots_view_all")} &rarr;</Link>
        </div>
        <div className="p-3"><div className="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-2">{screenshots.slice(0, 12).map((fn, i) => (
          <Button key={fn} variant="ghost" onClick={() => onOpenLightbox(i)} className="relative aspect-video rounded-lg overflow-hidden bg-secondary group cursor-pointer p-0 h-auto">
            <img src={`/screenshots/${agentId}/${fn}`} alt={fn} className="w-full h-full object-cover" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
            <div className="absolute inset-0 bg-black/0 group-hover:bg-black/30 transition-colors flex items-center justify-center"><Maximize2 className="w-4 h-4" /></div>
          </Button>
        ))}</div></div>
      </Card>

      {lightboxOpen && (
        <Dialog open={true} onOpenChange={onCloseLightbox}>
          <DialogContent className="max-w-4xl bg-transparent border-0 p-0" showCloseButton={false}>
            <Button variant="ghost" size="icon-lg" onClick={onPrevLightbox} className="absolute left-4 top-1/2 -translate-y-1/2 rounded-full bg-black/50 hover:bg-black/70 text-white" disabled={lightboxIdx === 0} aria-label={t("agents.screenshots_prev")}><ChevronLeft className="w-4 h-4" /></Button>
            <div className="max-w-[90vw] max-h-[85vh]">
              <img src={`/screenshots/${agentId}/${screenshots[lightboxIdx!]}`} alt={`Screenshot ${lightboxIdx! + 1} of ${screenshots.length}`} className="max-w-full max-h-[80vh] object-contain rounded-lg" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
              <div className="text-center text-white/70 text-xs mt-2">{lightboxIdx! + 1} / {screenshots.length}</div>
            </div>
            <Button variant="ghost" size="icon-lg" onClick={onNextLightbox} className="absolute right-4 top-1/2 -translate-y-1/2 rounded-full bg-black/50 hover:bg-black/70 text-white" disabled={lightboxIdx! >= screenshots.length - 1} aria-label={t("agents.screenshots_next")}><ChevronRight className="w-4 h-4" /></Button>
            <Button variant="ghost" size="icon" onClick={onCloseLightbox} className="absolute top-4 right-4 rounded-full bg-black/50 hover:bg-black/70 text-white" aria-label={t("agents.screenshots_close")}><X className="w-4 h-4" /></Button>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
