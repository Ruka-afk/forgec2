"use client";

import { memo } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import Link from "next/link";
import { Camera, ChevronLeft, ChevronRight, Maximize2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface AgentScreenshotsProps {
  screenshots: string[];
  newScreenshots?: string[];
  agentId: string;
  lightboxIdx: number | null;
  onOpenLightbox: (idx: number) => void;
  onCloseLightbox: () => void;
  onPrevLightbox: () => void;
  onNextLightbox: () => void;
}

export default memo(function AgentScreenshots({
  screenshots, newScreenshots = [], agentId, lightboxIdx, onOpenLightbox,
  onCloseLightbox, onPrevLightbox, onNextLightbox,
}: AgentScreenshotsProps) {
  const { t } = useI18n();
  if (screenshots.length === 0) return null;

  const lightboxOpen = lightboxIdx !== null;
  const freshSet = new Set(newScreenshots);

  return (
    <>
      <Card className="mb-4 overflow-hidden border-border/70 bg-card/90 shadow-sm">
        <div className="h-1 w-full bg-gradient-to-r from-primary via-chart-2 to-chart-1" />
        <div className="px-4 py-3 border-b border-border/70 flex items-center justify-between">
          <h3 className="text-sm font-semibold text-foreground flex items-center gap-2"><Camera className="w-3.5 h-3.5 text-primary" />{t("agents.screenshots_title")} <span className="rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 text-(--fs-micro-sm) font-normal text-muted-foreground/70">({screenshots.length})</span></h3>
          <Link href={`/loot?tab=screenshots&agent_id=${agentId}`} className="text-xs text-primary hover:underline">{t("agents.screenshots_view_all")} &rarr;</Link>
        </div>
        <div className="p-3">
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6 gap-2">
            {screenshots.slice(0, 12).map((fn, i) => (
<Button key={fn} variant="ghost" onClick={() => onOpenLightbox(i)} className="group relative aspect-video rounded-lg border border-border/70 bg-muted/30 p-0 h-auto overflow-hidden shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:shadow-md">
                <img src={`/screenshots/${agentId}/${fn}`} alt={fn} className="w-full h-full object-cover transition-transform duration-300 group-hover:scale-105" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }} />
                {freshSet.has(fn) && (
                  <span className="absolute top-1.5 left-1.5"><Badge className="px-1.5 py-0 text-(--fs-micro-sm)">{t("agents.screenshots_new")}</Badge></span>
                )}
                <div className="absolute inset-0 bg-gradient-to-t from-black/45 via-black/0 to-transparent opacity-0 transition-opacity group-hover:opacity-100" />
                <div className="absolute inset-0 flex items-center justify-center opacity-0 transition-opacity group-hover:opacity-100">
                  <span className="flex h-8 w-8 items-center justify-center rounded-full bg-black/40 text-white backdrop-blur-sm">
                    <Maximize2 className="w-4 h-4" />
                  </span>
                </div>
              </Button>
            ))}
          </div>
        </div>
      </Card>

      {lightboxOpen && (
        <Dialog open={true} onOpenChange={onCloseLightbox}>
          <DialogContent className="max-w-5xl bg-transparent border-0 p-0 shadow-none" showCloseButton={false}>
            <div className="relative mx-auto max-w-[92vw] max-h-[88vh]">
              <Button variant="ghost" size="icon-lg" onClick={onPrevLightbox} className="absolute left-3 top-1/2 -translate-y-1/2 rounded-full bg-black/50 hover:bg-black/70 text-white backdrop-blur-sm" disabled={lightboxIdx === 0} aria-label={t("agents.screenshots_prev")}><ChevronLeft className="w-4 h-4" /></Button>
              <img src={`/screenshots/${agentId}/${screenshots[lightboxIdx!]}`} alt={`Screenshot ${lightboxIdx! + 1} of ${screenshots.length}`} className="max-w-full max-h-[82vh] object-contain rounded-2xl border border-white/10 shadow-2xl" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }} />
              <div className="absolute left-1/2 bottom-3 -translate-x-1/2 rounded-full bg-black/55 px-3 py-1 text-(--fs-xs-sm) text-white/80 backdrop-blur-sm">
                {lightboxIdx! + 1} / {screenshots.length}
              </div>
              <Button variant="ghost" size="icon-lg" onClick={onNextLightbox} className="absolute right-3 top-1/2 -translate-y-1/2 rounded-full bg-black/50 hover:bg-black/70 text-white backdrop-blur-sm" disabled={lightboxIdx! >= screenshots.length - 1} aria-label={t("agents.screenshots_next")}><ChevronRight className="w-4 h-4" /></Button>
              <Button variant="ghost" size="icon" onClick={onCloseLightbox} className="absolute top-3 right-3 rounded-full bg-black/50 hover:bg-black/70 text-white backdrop-blur-sm" aria-label={t("agents.screenshots_close")}><X className="w-4 h-4" /></Button>
            </div>
          </DialogContent>
        </Dialog>
)}
    </>
  );
})
