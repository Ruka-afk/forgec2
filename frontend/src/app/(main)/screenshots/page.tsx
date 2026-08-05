"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { formatTime } from "@/lib/utils";
import { PageHeader, Pagination, ConfirmModal } from "@/components/UI";
import { DataState } from "@/components/ui/data-state";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { ChevronLeft, ChevronRight, Download, Images, Minus, Pause, Play, Plus, Trash2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { Screenshot, Resolution } from "@/types/screenshot";
import { ScreenshotCard } from "./_components/ScreenshotCard";

const PAGE_SIZE = 24;

export default function ScreenshotsPage() {
  const { t } = useI18n();
  const [screenshots, setScreenshots] = useState<Screenshot[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [agentFilter, setAgentFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [cfm, setCfm] = useState<{ msg: string; cb: () => void } | null>(null);

  const [lbOpen, setLbOpen] = useState(false);
  const [lbIndex, setLbIndex] = useState(0);
  const [zoom, setZoom] = useState(1);
  const [slideshow, setSlideshow] = useState(false);
  const slideshowRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [resolutions, setResolutions] = useState<Record<string, Resolution>>({});

  const loadData = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      // Dual-use /loot — same as loot page (not /api/loot)
      const result = await api.get(paths.loot.page, { signal });
      if (signal?.aborted) return;
      setScreenshots((result.screenshots || []) as Screenshot[]);
    } catch (e) {
      if (signal?.aborted) return;
      setScreenshots([]);
      const msg = e instanceof Error ? e.message : t("screenshots.load_failed");
      setError(msg);
      toast.error(msg);
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const ac = new AbortController();
    void loadData(ac.signal);
    return () => ac.abort();
  }, [loadData]);

  useEffect(() => {
    return () => {
      if (slideshowRef.current) clearInterval(slideshowRef.current);
    };
  }, []);

  const filtered = screenshots.filter(s => !agentFilter || s.agent_id === agentFilter);
  const currentItems = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE);
  const allAgents = [...new Set(screenshots.map(s => s.agent_id))];
  const lbImages = filtered.map(s => ({ ...s, url: `/screenshots/${s.path}` }));
  const current = lbImages[lbIndex];

  useEffect(() => { setPage(1); }, [agentFilter]);

  const lbImagesLenRef = useRef(0);
  lbImagesLenRef.current = lbImages.length;

  const goNext = useCallback(() => {
    setLbIndex(i => Math.min(i + 1, lbImagesLenRef.current - 1));
  }, []);

  const goPrev = useCallback(() => {
    setLbIndex(i => Math.max(i - 1, 0));
  }, []);

  useEffect(() => {
    if (!lbOpen) return;
    const handler = (e: KeyboardEvent) => {
      switch (e.key) {
        case "Escape": setLbOpen(false); break;
        case "ArrowLeft": goPrev(); break;
        case "ArrowRight": goNext(); break;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [lbOpen, goNext, goPrev]);

  useEffect(() => {
    if (slideshow) {
      slideshowRef.current = setInterval(() => {
        setLbIndex(i => (i < lbImages.length - 1 ? i + 1 : 0));
      }, 3000);
    } else {
      if (slideshowRef.current) {
        clearInterval(slideshowRef.current);
        slideshowRef.current = null;
      }
    }
    return () => {
      if (slideshowRef.current) clearInterval(slideshowRef.current);
    };
  }, [slideshow, lbImages.length]);

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleSelectAll = () => {
    const allSelected = currentItems.every(s => selectedIds.has(s.id));
    setSelectedIds(prev => {
      const next = new Set(prev);
      for (const s of currentItems) {
        if (allSelected) next.delete(s.id);
        else next.add(s.id);
      }
      return next;
    });
  };

  const deleteSelected = () => {
    if (selectedIds.size === 0) return;
    setCfm({
      msg: t("screenshots.confirm_delete", { count: selectedIds.size }),
      cb: async () => {
        try {
          await api.postJson(paths.loot.bulkDelete, { ids: [...selectedIds] });
          setSelectedIds(new Set());
          void loadData();
        } catch { toast.error(t("screenshots.delete_failed")); }
        setCfm(null);
      },
    });
  };

  const handleResolution = useCallback((id: string, w: number, h: number) => {
    setResolutions(prev => {
      if (prev[id]) return prev;
      return { ...prev, [id]: { w, h } };
    });
  }, []);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader
        title={<>{t("screenshots.title")} <span className="text-primary">{t("screenshots.gallery")}</span></>}
        subtitle={t("screenshots.subtitle")}
      >
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={agentFilter} onValueChange={(v) => { setAgentFilter(v ?? ""); setSelectedIds(new Set()); }}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("screenshots.all_agents")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("screenshots.all_agents")}</SelectItem>
              {allAgents.map(a => (
                <SelectItem key={a} value={a}>{a.substring(0, 12)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          {selectedIds.size > 0 && (
            <Button
              variant="destructive"
              size="lg"
              onClick={deleteSelected}
              className="gap-2"
            >
              <Trash2 aria-hidden="true" className="w-4 h-4" /><span>{t("screenshots.delete")} ({selectedIds.size})</span>
            </Button>
          )}
        </div>
      </PageHeader>

      <DataState
        loading={loading}
        error={error}
        onRetry={() => void loadData()}
        empty={!loading && !error && currentItems.length === 0}
        emptyIcon={Images}
        emptyTitle={t("screenshots.empty_title")}
        emptyMessage={t("screenshots.empty_message")}
        loadingSkeleton={
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
            {Array.from({ length: 12 }).map((_, i) => (
              <Skeleton key={i} className="rounded-xl border border-border h-28" />
            ))}
          </div>
        }
      >
        <>
          <div className="flex items-center justify-between mb-3">
            {currentItems.length > 0 && (
              <Label className="flex items-center gap-x-2 text-xs text-muted-foreground cursor-pointer">
                <Checkbox aria-label={t("loot.select_all_screenshots")} checked={currentItems.length > 0 && currentItems.every(s => selectedIds.has(s.id))} onCheckedChange={toggleSelectAll} />
                {t("screenshots.select_all")}
              </Label>
            )}
            <Badge variant="secondary">{filtered.length} {t("screenshots.screenshots_count")}</Badge>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
            {currentItems.map(s => {
              const globalIdx = filtered.indexOf(s);
              return (
                <ScreenshotCard
                  key={s.id}
                  screenshot={s}
                  isSelected={selectedIds.has(s.id)}
                  index={globalIdx}
                  onToggleSelect={toggleSelect}
                  onOpen={(idx) => { setLbIndex(idx); setZoom(1); setSlideshow(false); setLbOpen(true); }}
                  onResolution={handleResolution}
                />
              );
            })}
          </div>

          <div className="mt-4">
            <Pagination page={page} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
          </div>
        </>
      </DataState>

      {lbOpen && current && (
        <Dialog open={true} onOpenChange={() => setLbOpen(false)}>
          <DialogContent className="max-w-[100vw] w-full h-full bg-black/95 border-0 rounded-none p-0 gap-0 flex flex-col" showCloseButton={false}>
            <DialogTitle className="sr-only">{current.filename}</DialogTitle>
            <div className="flex items-center justify-between px-4 py-2 bg-black/80 border-b border-white/10 shrink-0">
              <div className="flex items-center gap-3 text-xs text-white/70">
                <span className="font-medium text-white">{lbIndex + 1} / {lbImages.length}</span>
                <span className="hidden sm:inline">{current.filename}</span>
              </div>
              <div className="flex items-center gap-1">
                <Tooltip>
                  <TooltipTrigger render={                    <Button variant="ghost" size="icon-sm" onClick={() => setZoom(z => Math.max(0.25, z - 0.25))} className="text-white/70 hover:text-white hover:bg-white/10" aria-label={t("screenshots.lightbox_zoom_out")} />}>
                    <Minus aria-hidden="true" className="w-4 h-4" />
                  </TooltipTrigger>
                  <TooltipContent>{t("screenshots.lightbox_zoom_out")}</TooltipContent>
                </Tooltip>
                <span className="text-xs text-white/60 w-10 text-center">{Math.round(zoom * 100)}%</span>
                <Tooltip>
                  <TooltipTrigger render={                    <Button variant="ghost" size="icon-sm" onClick={() => setZoom(z => Math.min(5, z + 0.25))} className="text-white/70 hover:text-white hover:bg-white/10" aria-label={t("screenshots.lightbox_zoom_in")} />}>
                    <Plus aria-hidden="true" className="w-4 h-4" />
                  </TooltipTrigger>
                  <TooltipContent>{t("screenshots.lightbox_zoom_in")}</TooltipContent>
                </Tooltip>
                <div className="w-px h-5 bg-secondary/70 mx-1" />
                <Tooltip>
                  <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => setSlideshow(s => !s)} className={`${slideshow ? "text-emerald-400" : "text-white/70 hover:text-white hover:bg-white/10"}`} aria-label={t("screenshots.lightbox_slideshow")} />}>
                    {slideshow ? <Pause aria-hidden="true" className="w-4 h-4" /> : <Play aria-hidden="true" className="w-4 h-4" />}
                  </TooltipTrigger>
                  <TooltipContent>{t("screenshots.lightbox_slideshow")}</TooltipContent>
                </Tooltip>
                <Tooltip>
                  <TooltipTrigger render={<a href={current.url} download aria-label={t("screenshots.lightbox_download")} className="inline-flex size-7 items-center justify-center rounded-lg text-white/70 hover:text-white hover:bg-white/10 transition-colors text-sm" />}>
                    <Download aria-hidden="true" className="w-4 h-4" />
                  </TooltipTrigger>
                  <TooltipContent>{t("screenshots.lightbox_download")}</TooltipContent>
                </Tooltip>
                <div className="w-px h-5 bg-secondary/70 mx-1" />
                <Tooltip>
                  <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={() => setLbOpen(false)} className="text-white/70 hover:text-white hover:bg-white/10" aria-label={t("common.close")} />}>
                    <X aria-hidden="true" className="w-4 h-4" />
                  </TooltipTrigger>
                  <TooltipContent>{t("screenshots.lightbox_close")}</TooltipContent>
                </Tooltip>
              </div>
            </div>

            <div className="flex-1 flex items-center justify-center relative overflow-hidden">
              {lbIndex > 0 && (
                <Button variant="ghost" size="icon" onClick={goPrev} className="absolute left-2 sm:left-4 top-1/2 -translate-y-1/2 z-10 text-white/80 hover:text-white hover:bg-white/10 rounded-full" aria-label={t("screenshots.lightbox_previous")}>
                  <ChevronLeft aria-hidden="true" className="w-4 h-4" />
                </Button>
              )}
              {lbIndex < lbImages.length - 1 && (
                <Button variant="ghost" size="icon" onClick={goNext} className="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 z-10 text-white/80 hover:text-white hover:bg-white/10 rounded-full" aria-label={t("screenshots.lightbox_next")}>
                  <ChevronRight aria-hidden="true" className="w-4 h-4" />
                </Button>
              )}
              <img
                src={current.url}
                alt={current.filename}
                className="max-w-[95vw] max-h-[85vh] object-contain transition-transform duration-200 select-none"
                style={{ transform: `scale(${zoom})` }}
                draggable={false}
                loading="lazy"
                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                onLoad={e => {
                  const img = e.currentTarget;
                  if (img.naturalWidth) handleResolution(current.id, img.naturalWidth, img.naturalHeight);
                }}
              />
            </div>

            <div className="flex items-center justify-center gap-4 sm:gap-6 px-4 py-2 bg-black/80 border-t border-white/10 text-xs text-white/60 shrink-0 flex-wrap">
              <span><span className="text-white/40">{t("screenshots.info_agent")}</span> {current.agent_id.substring(0, 12)}</span>
              <span><span className="text-white/40">{t("screenshots.info_file")}</span> {current.filename}</span>
              <span><span className="text-white/40">{t("screenshots.info_time")}</span> {formatTime(current.created_at)}</span>
              {resolutions[current.id] && (
                <span><span className="text-white/40">{t("screenshots.info_size")}</span> {resolutions[current.id].w}&times;{resolutions[current.id].h}</span>
              )}
            </div>
          </DialogContent>
        </Dialog>
      )}

      <ConfirmModal
        open={!!cfm}
        title={t("common.confirm")}
        message={cfm?.msg || ""}
        confirmText={t("common.delete")}
        cancelText={t("common.cancel")}
        danger
        onConfirm={() => { cfm?.cb(); }}
        onCancel={() => setCfm(null)}
      />
    </div>
  );
}

