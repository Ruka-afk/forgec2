"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { formatTime } from "@/lib/utils";
import { EmptyState, PageHeader, Pagination, ConfirmModal } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Check, ChevronLeft, ChevronRight, Download, Images, Minus, Pause, Play, Plus, Trash2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface Screenshot {
  id: string;
  agent_id: string;
  filename: string;
  path: string;
  created_at: string;
}

interface Resolution {
  w: number;
  h: number;
}

const PAGE_SIZE = 24;

export default function ScreenshotsPage() {
  const { t } = useI18n();
  const [screenshots, setScreenshots] = useState<Screenshot[]>([]);
  const [loading, setLoading] = useState(true);
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

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const result = await api.get("/loot");
      setScreenshots((result.screenshots || []) as Screenshot[]);
    } catch {
      setScreenshots([]);
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

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

  const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

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
      msg: `Delete ${selectedIds.size} screenshot(s)?`,
      cb: async () => {
        try {
          await api.postJson("/loot/bulk-delete", { ids: [...selectedIds] });
          setSelectedIds(new Set());
          loadData();
        } catch { toast.error(t("screenshots.delete_failed")); }
        setCfm(null);
      },
    });
  };

  const handleResolution = (id: string, w: number, h: number) => {
    setResolutions(prev => {
      if (prev[id]) return prev;
      return { ...prev, [id]: { w, h } };
    });
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
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
              <Trash2 className="w-4 h-4" /><span>{t("screenshots.delete")} ({selectedIds.size})</span>
            </Button>
          )}
        </div>
      </PageHeader>

      {loading ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
          {Array.from({ length: 12 }).map((_, i) => (
            <Skeleton key={i} className="rounded-xl border border-border h-28" />
          ))}
        </div>
      ) : currentItems.length > 0 ? (
        <>
          <div className="flex items-center justify-between mb-3">
            {currentItems.length > 0 && (
              <Label className="flex items-center gap-x-2 text-xs text-muted-foreground cursor-pointer">
                <Checkbox aria-label="Select all screenshots" checked={currentItems.length > 0 && currentItems.every(s => selectedIds.has(s.id))} onCheckedChange={toggleSelectAll} />
                {t("screenshots.select_all")}
              </Label>
            )}
            <Badge variant="secondary">{filtered.length} {t("screenshots.screenshots_count")}</Badge>
          </div>

          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
            {currentItems.map(s => {
              const globalIdx = filtered.indexOf(s);
              return (
                <div
                  key={s.id}
                  className={`group relative rounded-xl overflow-hidden border-2 cursor-pointer bg-muted transition-all hover:shadow-lg dark:hover:shadow-black/30 ${
                    selectedIds.has(s.id)
                      ? "border-indigo-500 ring-2 ring-indigo-200 dark:ring-indigo-800"
                      : "border-border"
                  }`}
                  onClick={() => {
                    setLbIndex(globalIdx);
                    setZoom(1);
                    setSlideshow(false);
                    setLbOpen(true);
                  }}
                >
                  <div className="absolute top-1.5 left-1.5 z-10" onClick={e => { e.stopPropagation(); toggleSelect(s.id); }}>
                    <div className={`w-5 h-5 rounded border-2 flex items-center justify-center transition-colors ${
                      selectedIds.has(s.id)
                        ? "bg-indigo-500 border-indigo-500"
                        : "bg-secondary/90 border-border"
                    }`}>
                      {selectedIds.has(s.id) && <Check className="w-4 h-4" />}
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
                      if (img.naturalWidth) handleResolution(s.id, img.naturalWidth, img.naturalHeight);
                    }}
                  />
                  <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-[10px] text-white px-2 py-1 opacity-0 group-hover:opacity-100 transition flex justify-between items-center">
                    <span className="truncate">{s.agent_id.substring(0, 8)}</span>
                    <a href={`/screenshots/${s.path}`} download onClick={e => e.stopPropagation()} className="hover:text-emerald-300 px-1 transition-colors">
                      <Download className="w-4 h-4" />
                    </a>
                  </div>
                </div>
              );
            })}
          </div>

          <div className="mt-4">
            <Pagination page={page} pageSize={PAGE_SIZE} total={filtered.length} onPageChange={setPage} />
          </div>
        </>
      ) : (
        <div className="text-center py-16 sm:py-20">
          <EmptyState icon={Images} title={t("screenshots.empty_title")} message={t("screenshots.empty_message")} />
        </div>
      )}

      {lbOpen && current && (
        <Dialog open={true} onOpenChange={() => setLbOpen(false)}>
          <DialogContent className="max-w-[100vw] w-full h-full bg-black/95 border-0 rounded-none p-0 gap-0 flex flex-col" showCloseButton={false}>
            <div className="flex items-center justify-between px-4 py-2 bg-black/80 border-b border-white/10 shrink-0">
              <div className="flex items-center gap-3 text-xs text-white/70">
                <span className="font-medium text-white">{lbIndex + 1} / {lbImages.length}</span>
                <span className="hidden sm:inline">{current.filename}</span>
              </div>
              <div className="flex items-center gap-1">
                <Button variant="ghost" size="icon-sm" onClick={() => setZoom(z => Math.max(0.25, z - 0.25))} className="text-white/70 hover:text-white hover:bg-white/10" title="Zoom out" aria-label="Zoom out">
                  <Minus className="w-4 h-4" />
                </Button>
                <span className="text-xs text-white/60 w-10 text-center">{Math.round(zoom * 100)}%</span>
                <Button variant="ghost" size="icon-sm" onClick={() => setZoom(z => Math.min(5, z + 0.25))} className="text-white/70 hover:text-white hover:bg-white/10" title="Zoom in" aria-label="Zoom in">
                  <Plus className="w-4 h-4" />
                </Button>
                <div className="w-px h-5 bg-secondary/70 mx-1" />
                <Button variant="ghost" size="icon-sm" onClick={() => setSlideshow(s => !s)} className={`${slideshow ? "text-emerald-400" : "text-white/70 hover:text-white hover:bg-white/10"}`} title="Slideshow" aria-label="Slideshow">
                  {slideshow ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
                </Button>
                <a href={current.url} download className="inline-flex size-7 items-center justify-center rounded-lg text-white/70 hover:text-white hover:bg-white/10 transition-colors text-sm" title="Download">
                  <Download className="w-4 h-4" />
                </a>
                <div className="w-px h-5 bg-secondary/70 mx-1" />
                <Button variant="ghost" size="icon-sm" onClick={() => setLbOpen(false)} className="text-white/70 hover:text-white hover:bg-white/10" title="Close (Esc)" aria-label="Close">
                  <X className="w-4 h-4" />
                </Button>
              </div>
            </div>

            <div className="flex-1 flex items-center justify-center relative overflow-hidden">
              {lbIndex > 0 && (
                <Button variant="ghost" size="icon" onClick={goPrev} className="absolute left-2 sm:left-4 top-1/2 -translate-y-1/2 z-10 text-white/80 hover:text-white hover:bg-white/10 rounded-full" aria-label="Previous">
                  <ChevronLeft className="w-4 h-4" />
                </Button>
              )}
              {lbIndex < lbImages.length - 1 && (
                <Button variant="ghost" size="icon" onClick={goNext} className="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 z-10 text-white/80 hover:text-white hover:bg-white/10 rounded-full" aria-label="Next">
                  <ChevronRight className="w-4 h-4" />
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
              <span><span className="text-white/40">Agent:</span> {current.agent_id.substring(0, 12)}</span>
              <span><span className="text-white/40">File:</span> {current.filename}</span>
              <span><span className="text-white/40">Time:</span> {formatTime(current.created_at)}</span>
              {resolutions[current.id] && (
                <span><span className="text-white/40">Size:</span> {resolutions[current.id].w}&times;{resolutions[current.id].h}</span>
              )}
            </div>
          </DialogContent>
        </Dialog>
      )}

      <ConfirmModal
        open={!!cfm}
        title="Confirm"
        message={cfm?.msg || ""}
        confirmText="Delete"
        cancelText="Cancel"
        danger
        onConfirm={() => { cfm?.cb(); }}
        onCancel={() => setCfm(null)}
      />
    </div>
  );
}

