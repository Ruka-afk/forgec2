"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Play, Pause, Maximize2, Circle } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { cn } from "@/lib/utils";

interface ScreenVideoPlayerProps {
  src: string | null;
  wsLive: boolean;
  fps?: number;
  className?: string;
  onFullscreen?: () => void;
}

export function ScreenVideoPlayer({ src, wsLive, fps = 5, className, onFullscreen }: ScreenVideoPlayerProps) {
  const { t } = useI18n();
  const imgRef = useRef<HTMLImageElement>(null);
  const [isPlaying, setIsPlaying] = useState(true);
  const [frameCount, setFrameCount] = useState(0);
  const [lastFps, setLastFps] = useState(0);
  const fpsRef = useRef<{ count: number; last: number } | null>(null);
  if (fpsRef.current === null) {
    fpsRef.current = { count: 0, last: 0 };
  }

  useEffect(() => {
    if (!src) return;
    if (!isPlaying) return;
    if (fpsRef.current === null) fpsRef.current = { count: 0, last: Date.now() };
    setFrameCount((c) => c + 1);
    fpsRef.current.count++;
    const now = Date.now();
    if (fpsRef.current.last === 0) fpsRef.current.last = now;
    if (now - fpsRef.current.last >= 1000) {
      setLastFps(fpsRef.current.count);
      fpsRef.current.count = 0;
      fpsRef.current.last = now;
    }
  }, [src, isPlaying]);

  if (!src) {
    return (
      <div className={cn("flex aspect-video w-full items-center justify-center rounded-xl border border-dashed border-border/70 bg-muted/20", className)}>
        <span className="text-sm text-muted-foreground">No video — start monitoring</span>
      </div>
    );
  }

  return (
    <div className={cn("group relative overflow-hidden rounded-xl border border-border bg-black", className)}>
      <img
        ref={imgRef}
        src={src}
        alt={t("agents.screen_alt_full")}
        className="h-full w-full object-contain"
        style={{ display: isPlaying ? "block" : "none" }}
        draggable={false}
      />
      {!isPlaying && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/60">
          <span className="text-sm text-white">Paused</span>
        </div>
      )}
      {/* Video-like overlay controls */}
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent p-3 opacity-0 transition-opacity group-hover:opacity-100">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Button size="icon-xs" variant="ghost" className="text-white hover:bg-white/20" onClick={() => setIsPlaying(!isPlaying)}>
              {isPlaying ? <Pause className="size-3.5" /> : <Play className="size-3.5" />}
            </Button>
            <Badge variant="secondary" className="gap-1 bg-black/40 text-white text-xs">
              <Circle className={cn("size-2", wsLive ? "fill-success text-success animate-pulse" : "fill-muted text-muted")} />
              {wsLive ? "LIVE" : "POLL"} · {lastFps || fps} fps
            </Badge>
            <span className="text-xs text-white/80">#{frameCount}</span>
          </div>
          <Button size="icon-xs" variant="ghost" className="text-white hover:bg-white/20" onClick={onFullscreen}>
            <Maximize2 className="size-3.5" />
          </Button>
        </div>
        <div className="mt-1 h-0.5 w-full bg-white/20 rounded-full overflow-hidden">
          <div className="h-full bg-primary transition-all" style={{ width: wsLive ? "100%" : "45%" }} />
        </div>
      </div>
      <div className="absolute top-2 left-2 flex items-center gap-1.5">
        <span className="inline-flex items-center gap-1 rounded-full bg-destructive/90 px-2 py-0.5 text-xs font-medium text-white">
          <span className="size-1.5 rounded-full bg-white animate-pulse" /> REC
        </span>
        {wsLive && <span className="rounded-full bg-success/90 px-2 py-0.5 text-xs font-medium text-white">WS</span>}
      </div>
    </div>
  );
}
