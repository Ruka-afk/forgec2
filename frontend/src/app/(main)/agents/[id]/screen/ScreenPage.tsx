"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useWS } from "@/lib/wsContext";
import { api } from "@/lib/api";
import { EmptyState, Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { toast } from "sonner";
import { ArrowLeft, Camera, Clock, Download, ImageIcon, Images, Maximize2, Monitor, Play, RotateCw, Square, TriangleAlert, X, Zap } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface ScreenshotItem {
  id?: string;
  data?: string;
  timestamp?: string;
  width?: number;
  height?: number;
  window_name?: string;
}export default function ScreenPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
  const [monitoring, setMonitoring] = useState(false);  const [monitoringStatus, setMonitoringStatus] = useState<"connected" | "offline" | "capturing" | "waiting">("waiting");
  const [screenshot, setScreenshot] = useState<string | null>(null);
  const [screenshotGallery, setScreenshotGallery] = useState<ScreenshotItem[]>([]);
  const [lastUpdate, setLastUpdate] = useState<string>("-");  const [interval, setInterval_] = useState(3);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [modalImage, setModalImage] = useState<string>("");  const [resolution, setResolution] = useState<{ width: number; height: number } | null>(null);  const [status, setStatus] = useState<"waiting" | "capturing" | "error">("waiting");
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const galleryIdRef = useRef(0);
  const [wsLive, setWsLive] = useState(false);
  const { subscribe } = useWS();

  const captureScreenshot = useCallback(async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      const data = await api.get<{image?: string; data?: string; screenshot?: string; width?: number; height?: number; window_name?: string}>(`/agents/${id}/screenshot`);
      const imgData = data.image || data.data || data.screenshot || "";
      const width = data.width || 0;
      const height = data.height || 0;
      if (imgData) {
        const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;
        setScreenshot(fullData);
        setLastUpdate(new Date().toLocaleTimeString());
        setStatus("waiting");
        setMonitoringStatus("connected");
        if (width && height) setResolution({ width, height });
        const galleryItem: ScreenshotItem = {
          id: String(++galleryIdRef.current),
          data: fullData,
          timestamp: new Date().toLocaleTimeString(),
          width,
          height,
          window_name: data.window_name || "Desktop",
        };
        setScreenshotGallery(prev => [galleryItem, ...prev].slice(0, 50));
      } else {
        setStatus("error");
        setMonitoringStatus("offline");
      }
    } catch {
      setStatus("error");
      setMonitoringStatus("offline");
    }
  }, [id]);  const startMonitoring = async () => {
    if (!id) return;
    setMonitoring(true);
    setMonitoringStatus("capturing");
    try {      const body = new URLSearchParams();      body.append("interval", (interval * 1000).toString());
      await api.post(`/agents/${id}/screen/start`, { interval: (interval * 1000).toString() });      toast.success(t("agents.screen_started"));
      } catch {
      toast.error(t("agents.screen_start_failed"));
    }
    await captureScreenshot();    if (autoRefresh) {
      if (timerRef.current) clearInterval(timerRef.current);
      timerRef.current = setInterval(captureScreenshot, interval * 1000);    }
  };

  const stopMonitoring = async () => {
    setMonitoring(false);
    setMonitoringStatus("waiting");
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }    try {
      await api.post(`/agents/${id}/screen/stop`, {});
      toast.success(t("agents.screen_stopped"));
    } catch { toast.error(t("agents.screen_stop_failed")); }
  };

  const handleManualCapture = async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      await api.post<{ success?: boolean }>(`/agents/${id}/screenshot`, {});
      toast.success(t("agents.screen_capture"));
      await new Promise((r) => setTimeout(r, 900));
      await captureScreenshot();
    } catch (err) {
      setStatus("error");
      setMonitoringStatus("offline");
      toast.error(String(err));
    }
  };

  const handleWindowScreenshot = async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      await api.post<{ success?: boolean }>(`/agents/${id}/screenshot_window`, {});
      toast.success(t("agents.screen_window_capture"));
      await new Promise((r) => setTimeout(r, 900));
      await captureScreenshot();
    } catch (err) {
      setStatus("error");
      setMonitoringStatus("offline");
      toast.error(String(err));
    }
  };

  const handleDownloadScreenshot = (imgSrc?: string, filename?: string) => {
    const target = imgSrc || screenshot;
    if (!target) return;
    const a = document.createElement("a");
    a.href = target;
    a.download = filename || `screenshot_${id}_${Date.now()}.png`;
    a.click();
  };

  const openModal = (imgSrc: string) => {
    setModalImage(imgSrc);
    setShowModal(true);
  };

  useEffect(() => {
    if (monitoring && autoRefresh) {
      if (timerRef.current) clearInterval(timerRef.current);
      timerRef.current = setInterval(captureScreenshot, interval * 1000);
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [monitoring, autoRefresh, interval, captureScreenshot]);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  const applyFrame = useCallback((imgData: string, width = 0, height = 0, windowName = "Desktop") => {
    const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;
    setScreenshot(fullData);
    setLastUpdate(new Date().toLocaleTimeString());
    setStatus("waiting");
    setMonitoringStatus("connected");
    if (width && height) setResolution({ width, height });
    const galleryItem: ScreenshotItem = {
      id: String(++galleryIdRef.current),
      data: fullData,
      timestamp: new Date().toLocaleTimeString(),
      width,
      height,
      window_name: windowName,
    };
    setScreenshotGallery((prev) => [galleryItem, ...prev].slice(0, 50));
  }, []);

  useEffect(() => {
    if (!id) return;
    return subscribe((msg) => {
      if (msg.type === "screenshot" && String(msg.agent_id) === id && msg.data) {
        setWsLive(true);
        applyFrame(String(msg.data));
      }
    });
  }, [subscribe, id, applyFrame]);

  const statusIndicator = () => {
    if (status === "capturing") return { color: "bg-amber-400 animate-pulse", text: "Capturing...", icon: <Spinner size="xs" /> };
    if (status === "error") return { color: "bg-destructive", text: "Error", icon: <TriangleAlert className="w-3 h-3" /> };
    if (monitoring && monitoringStatus === "connected") return { color: "bg-emerald-400 animate-pulse", text: t("agents.rdp_connected"), icon: <span className="w-1.5 h-1.5 rounded-full bg-current" /> };
    return { color: "bg-muted-foreground", text: t("agents.rdp_standby"), icon: <span className="w-1.5 h-1.5 rounded-full bg-current" /> };
  };

  const indicator = statusIndicator();

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex flex-col h-[calc(100vh-4rem)]">        <div className="flex items-center justify-between mb-3 gap-3 flex-wrap">
          <div className="flex items-center gap-3">
            <h1 className="text-lg font-bold text-foreground flex items-center gap-2">
              <Monitor className="w-4 h-4" />
              {t("agents.screen_title")}
            </h1>
            <Badge variant="secondary" className="text-xs font-mono">{id}</Badge>
            {resolution && (
              <Badge variant="outline" className="text-xs flex items-center gap-1">
                <Maximize2 className="w-4 h-4" />{resolution.width} x {resolution.height}
              </Badge>
            )}          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {!monitoring ? (              <Button onClick={startMonitoring}
                className="bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
                <Play className="w-4 h-4" />
                {t("agents.screen_start")}
              </Button>
            ) : (
              <Button onClick={stopMonitoring}
                className="bg-destructive hover:bg-destructive/90 text-destructive-foreground text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
                <Square className="w-4 h-4" />                {t("agents.screen_stop")}
              </Button>
            )}
            <Button onClick={handleManualCapture} disabled={status === "capturing"}
              className="bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <Camera className="w-4 h-4" />
              {t("agents.screen_capture")}
            </Button>
            <Button onClick={handleWindowScreenshot} disabled={status === "capturing"}
              className="bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <Maximize2 className="w-4 h-4" />              {t("agents.screen_window_capture")}            </Button>
            <Button onClick={() => screenshot && handleDownloadScreenshot()} disabled={!screenshot}
              className="bg-muted hover:bg-border disabled:opacity-50 disabled:cursor-not-allowed text-foreground text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <Download className="w-4 h-4" />              {t("agents.screen_download")}
            </Button>          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-4 flex-1 min-h-0">          <div className="lg:col-span-3 flex flex-col min-h-0">
            <Card className="overflow-hidden flex-1 flex flex-col min-h-0">
              <div className="bg-muted px-4 py-2.5 flex items-center justify-between border-b border-border shrink-0">                <div className="flex items-center gap-2">
                  <Monitor className="w-4 h-4" />
                  <span className="text-sm font-medium text-foreground">{t("agents.screen_live_view")}</span>                </div>
                <div className="flex items-center gap-3 text-xs text-muted-foreground">
                  <span className={`flex items-center gap-1.5 ${status === "error" ? "text-destructive" : ""}`}>
                    <span className={`w-2 h-2 rounded-full ${indicator.color}`}></span>
                    {indicator.icon}
                    {indicator.text}                  </span>
                   <span className="hidden sm:inline text-muted-foreground/70">
                     <Clock className="w-4 h-4" />
                     {lastUpdate}
                   </span>
                  {monitoring && (
                    <Tooltip>
                      <TooltipTrigger>
                        <span className="ml-1 w-2 h-2 bg-emerald-500 rounded-full animate-pulse"></span>
                      </TooltipTrigger>
                      <TooltipContent>Monitoring active</TooltipContent>
                    </Tooltip>
                  )}
                  {wsLive && (
                    <Tooltip>
                      <TooltipTrigger>
                        <span className="text-(--font-size-micro-sm) text-emerald-400 flex items-center gap-1">
                          <Zap className="w-4 h-4" /> WS
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>WebSocket live stream</TooltipContent>
                    </Tooltip>
                  )}
                </div>
              </div>              <div className="relative bg-background flex-1 flex items-center justify-center cursor-pointer overflow-hidden"
                onClick={() => screenshot && openModal(screenshot)}>
                {screenshot ? (
                  <img src={screenshot} alt="Screenshot" className="max-w-full max-h-full object-contain" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
                ) : (
                  <div className="text-center text-muted-foreground/70 py-20">
                    <EmptyState icon={Monitor} title={t("agents.screen_no_screenshots")} message={`${t("agents.screen_start")} / ${t("agents.screen_capture")}`} />
                  </div>
                )}
                {status === "capturing" && (
                  <div className="absolute inset-0 flex items-center justify-center bg-black/40 backdrop-blur-sm">
                    <div className="flex flex-col items-center gap-3">
                      <Spinner size="xl" color="white" />
                      <span className="text-white text-sm font-medium">Capturing...</span>
                    </div>
                  </div>                )}
              </div>
            </Card>
          </div>          <div className="lg:col-span-1 flex flex-col gap-3 min-h-0 overflow-y-auto">
            <Card className="p-4 shrink-0">
              <div className="text-xs text-muted-foreground mb-3 font-semibold uppercase tracking-wider flex items-center justify-between">
                <span>{t("agents.screen_controls")}</span>
                 <span className={`flex items-center gap-1.5 text-(--font-size-micro-sm) font-normal ${monitoring ? "text-emerald-500" : "text-muted-foreground/70"}`}>                  <span className={`w-1.5 h-1.5 rounded-full ${monitoring ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground"}`}></span>                  {monitoring ? "LIVE" : "OFF"}
                </span>
              </div>
              <div className="space-y-3">
                <div>
                  <span className="block text-xs text-muted-foreground mb-1.5 flex items-center gap-1">
                    <Clock className="w-4 h-4" />
                    {t("agents.screen_interval")}
                  </span>
                  <Select value={String(interval)} onValueChange={(v) => { if (v !== null) setInterval_(Number(v)); }}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">1s (High CPU)</SelectItem>
                      <SelectItem value="3">3s</SelectItem>
                      <SelectItem value="5">5s</SelectItem>
                      <SelectItem value="10">10s</SelectItem>
                    </SelectContent>
                  </Select>
                </div>                <div className="flex items-center justify-between">
                  <span className="text-xs text-muted-foreground flex items-center gap-1">
                    <RotateCw className="w-4 h-4" />
                    {t("agents.screen_auto_refresh")}
                  </span>
                  <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
                </div>
                <div className="border-t border-border pt-3">
                  <span className="block text-xs text-muted-foreground mb-2 flex items-center gap-1">
                    <ImageIcon className="w-4 h-4" aria-hidden="true" /> Quick Capture                  </span>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">                    <Button onClick={handleManualCapture} disabled={status === "capturing"}
                      className="px-2 py-1.5 bg-primary/10 hover:bg-primary/20 text-primary rounded-lg text-xs transition-colors flex items-center justify-center gap-1 disabled:opacity-50">
                      <Camera className="w-4 h-4" /> {t("agents.screen_capture")}                    </Button>
                    <Button onClick={handleWindowScreenshot} disabled={status === "capturing"}
                      className="px-2 py-1.5 bg-primary/10 hover:bg-primary/20 text-primary rounded-lg text-xs transition-colors flex items-center justify-center gap-1 disabled:opacity-50">
                      <Maximize2 className="w-4 h-4" /> {t("agents.screen_window_capture")}
                    </Button>
                  </div>
                </div>
                {resolution && (
                  <div className="border-t border-border pt-3">
                    <span className="block text-xs text-muted-foreground mb-1.5 flex items-center gap-1">
                      <Maximize2 className="w-4 h-4" /> Resolution                    </span>
                     <div className="bg-muted rounded-lg px-3 py-2 text-sm font-mono text-foreground">
                      {resolution.width} &times; {resolution.height}
                    </div>
                  </div>
                )}
              </div>
            </Card>

            <Card className="p-4 flex-1 min-h-0 flex flex-col">
              <div className="flex items-center justify-between mb-3 shrink-0">
                <div className="text-xs text-muted-foreground font-semibold uppercase tracking-wider flex items-center gap-1">
                  <Images className="w-4 h-4" aria-hidden="true" /> {t("agents.screen_gallery")}
                </div>
                <Badge variant="secondary" className="text-(--font-size-micro-sm)">{screenshotGallery.length}</Badge>
              </div>
              {screenshotGallery.length > 0 ? (
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3 overflow-y-auto flex-1 max-h-64 lg:max-h-full">
                  {screenshotGallery.map((item) => (
                    <div key={item.id} className="relative group cursor-pointer rounded-xl overflow-hidden border-2 border-transparent hover:border-primary transition-colors bg-muted"
                      onClick={() => openModal(item.data || "")}>
                      <img src={item.data} alt="Thumbnail" className="w-full h-auto aspect-video object-cover" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />                      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent px-1.5 py-1 text-(--font-size-micro) text-white opacity-0 group-hover:opacity-100 transition-opacity">
                        {item.timestamp}
                      </div>
                      <div className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <Button
                          onClick={(e) => { e.stopPropagation(); handleDownloadScreenshot(item.data, `screen_${id}_${item.timestamp?.replace(/[:\s/]/g, "_")}.png`); }}
                          className="bg-black/60 hover:bg-black/80 text-white p-1 rounded text-(--font-size-micro) transition-colors h-auto"
                        >
                          <Download className="w-4 h-4" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>              ) : (
                <div className="text-center py-6 text-muted-foreground/70 text-xs flex-1 flex flex-col items-center justify-center">                  <ImageIcon className="w-4 h-4" aria-hidden="true" />                  <p>{t("agents.screen_no_screenshots")}</p>
                </div>
              )}            </Card>
          </div>        </div>
      </div>      <Dialog open={showModal} onOpenChange={setShowModal}>
        <DialogContent showCloseButton={false} className="sm:max-w-[95vw] p-0 bg-transparent border-0 ring-0 max-h-[95vh] overflow-visible">
          <div className="relative">
            <div className="absolute -top-12 left-0 flex items-center gap-3 z-10">
              <Tooltip>
                <TooltipTrigger render={<Button
                    onClick={() => handleDownloadScreenshot(modalImage)}
                    className="text-white/80 hover:text-white p-2 rounded-full bg-black/30 hover:bg-black/50 transition-colors h-auto"
                  />}>
                  <Download className="w-4 h-4" />
                </TooltipTrigger>
                <TooltipContent>Download</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger render={<Button
                    onClick={() => setShowModal(false)}
                    className="text-white/80 hover:text-white p-2 rounded-full bg-black/30 hover:bg-black/50 transition-colors h-auto"
                  />}>
                  <X className="w-4 h-4" />
                </TooltipTrigger>
                <TooltipContent>Close</TooltipContent>
              </Tooltip>
            </div>
            <img src={modalImage} alt="Full size" className="max-w-full max-h-[90vh] rounded-lg shadow-2xl" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}
