"use client";

import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { SafeImg } from "@/components/ui/safe-img";
import { Download, ImageIcon, Images } from "lucide-react";
import { safeImageSrc } from "@/lib/safeUrl";
import type { ScreenshotItem } from "./useScreenMonitor";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface ScreenGalleryProps {
  t: TKey;
  agentId: string;
  gallery: ScreenshotItem[];
  onOpen: (image: string) => void;
  onActivate: (event: React.KeyboardEvent<HTMLDivElement>, image: string) => void;
  onDownload: (image: string, filename: string) => void;
}

/** Still-mode thumbnail gallery (hidden in video mode). */
export default memo(function ScreenGallery({ t, agentId, gallery, onOpen, onActivate, onDownload }: ScreenGalleryProps) {
  return (
    <Card className="flex min-h-48 flex-1 flex-col p-4">
      <div className="mb-3 flex shrink-0 items-center justify-between">
        <div className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          <Images className="size-3.5" aria-hidden="true" />
          {t("agents.screen_gallery")}
        </div>
        <Badge variant="secondary" className="text-xs">{gallery.length}</Badge>
      </div>
      {gallery.length > 0 ? (
        <div className="grid flex-1 grid-cols-2 content-start gap-2 overflow-y-auto pr-1">
          {gallery.map((item) => (
            <div
              key={item.id}
              className="group relative cursor-pointer overflow-hidden rounded-lg border border-border bg-muted outline-none transition-colors hover:border-primary focus-visible:ring-2 focus-visible:ring-ring"
              role="button"
              tabIndex={0}
              aria-label={`${t("agents.screen_alt_thumb")} ${item.timestamp}`}
              onClick={() => onOpen(item.data)}
              onKeyDown={(event) => onActivate(event, item.data)}
            >
              <SafeImg src={safeImageSrc(item.data)} alt={t("agents.screen_alt_thumb")} className="aspect-video h-auto w-full object-cover" loading="lazy" decoding="async" />
              <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-1.5 py-1 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus:opacity-100">{item.timestamp}</div>
              <Button
                size="icon-xs"
                variant="secondary"
                aria-label={t("agents.screen_download")}
                onClick={(event) => {
                  event.stopPropagation();
                  onDownload(item.data, `screen_${agentId}_${item.id}.png`);
                }}
                className="absolute right-1 top-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus:opacity-100"
              >
                <Download className="size-3" />
              </Button>
            </div>
          ))}
        </div>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 py-6 text-center text-xs text-muted-foreground">
          <ImageIcon className="size-5" aria-hidden="true" />
          <p>{t("agents.screen_no_screenshots")}</p>
        </div>
      )}
    </Card>
  );
});
