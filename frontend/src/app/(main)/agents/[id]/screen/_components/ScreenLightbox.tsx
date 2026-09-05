"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { SafeImg } from "@/components/ui/safe-img";
import { Download, X } from "lucide-react";
import { safeImageSrc } from "@/lib/safeUrl";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface ScreenLightboxProps {
  t: TKey;
  open: boolean;
  image: string;
  onClose: () => void;
  onDownload: () => void;
}

/** Fullscreen capture viewer. */
export default memo(function ScreenLightbox({ t, open, image, onClose, onDownload }: ScreenLightboxProps) {
  return (
    <Dialog open={open} onOpenChange={(v) => { if (!v) onClose(); }}>
      <DialogContent showCloseButton={false} className="max-h-[95vh] border-0 bg-transparent p-0 ring-0 sm:max-w-[95vw]">
        <div className="relative flex max-h-[95vh] items-center justify-center">
          <div className="absolute right-2 top-2 z-10 flex items-center gap-2">
            <Button size="icon" variant="secondary" aria-label={t("agents.screen_download")} onClick={onDownload}>
              <Download className="size-4" />
            </Button>
            <Button size="icon" variant="secondary" aria-label={t("common.close")} onClick={onClose}>
              <X className="size-4" />
            </Button>
          </div>
          <SafeImg src={safeImageSrc(image)} alt={t("agents.screen_alt_full")} className="max-h-[95vh] max-w-full rounded-lg object-contain shadow-2xl" loading="eager" decoding="async" />
        </div>
      </DialogContent>
    </Dialog>
  );
});
