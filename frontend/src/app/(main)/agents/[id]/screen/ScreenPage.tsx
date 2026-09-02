"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { Camera, Clock, Download, ImageIcon, Images, Maximize2, Monitor, Play, RotateCw, Square, TriangleAlert, X, Zap } from "lucide-react";
import { toast } from "sonner";

import { api, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useI18n } from "@/lib/i18n";
import { safeImageSrc } from "@/lib/safeUrl";
import { nowTime } from "@/lib/utils";
import { useWS } from "@/lib/wsContext";
import { dataUrlToBlobUrl, revokeBlobUrl } from "@/lib/screenImage";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { EmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { PageContainer } from "@/components/ui/page-container";
import { SafeImg } from "@/components/ui/safe-img";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { StatusDot } from "@/components/ui/status-dot";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

type MonitorStatus = "connected" | "offline" | "capturing" | "waiting";
type CaptureStatus = "waiting" | "capturing" | "error";
type ScreenQuality = "low" | "medium" | "high";
type BusyAction = "start" | "stop" | "capture" | "window" | null;

interface ScreenshotItem {
  id: string;
  data: string;
  timestamp: string;
  width?: number;
  height?: number;
  window_name: string;
}

interface ScreenshotResponse {
  image?: string;
  data?: string;
  screenshot?: string;
  width?: number;
  height?: number;
  window_name?: string;
  capture_id?: string;
  captured_at?: string;
}

interface TaskResponse { task_id?: number | string }
interface TaskStatusResponse { status?: string; error?: string; result?: string }

const GALLERY_CAP = 48;
const TASK_TIMEOUT_MS = 90_000;

function delay(ms: number) {
  return new Promise<void>((resolve) => window.setTimeout(resolve, ms));
}

function frameDataUrl(value: string, mime = "image/png") {
  if (value.startsWith("data:")) return value;
  const detectedMime = value.startsWith("/9j/") ? "image/jpeg" : mime;
  return `data:${detectedMime};base64,${value}`;
}

export default function ScreenPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
  const { subscribe } = useWS();

  const [monitoring, setMonitoring] = useState(false);
  const [monitoringStatus, setMonitoringStatus] = useState<MonitorStatus>("waiting");
  const [status, setStatus] = useState<CaptureStatus>("waiting");
  const [busyAction, setBusyAction] = useState<BusyAction>(null);
  const [screenshot, setScreenshot] = useState<string | null>(null);
  const [screenshotGallery, setScreenshotGallery] = useState<ScreenshotItem[]>([]);
  const [lastUpdate, setLastUpdate] = useState("-");
  const [interval, setInterval_] = useState(3);
  const [quality, setQuality] = useState<ScreenQuality>("medium");
  const [triggerMatch, setTriggerMatch] = useState("");
  const [triggerOn, setTriggerOn] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [modalImage, setModalImage] = useState("");
  const [resolution, setResolution] = useState<{ width: number; height: number } | null>(null);
  const [wsLive, setWsLive] = useState(false);

  const monitoringRef = useRef(false);
  const mountedRef = useRef(true);
  const prevIdRef = useRef(id);
  const galleryIdRef = useRef(0);
  const frameSequenceRef = useRef(0);
  const lastFrameRef = useRef<string | null>(null);
  const lastCaptureIdRef = useRef<string | null>(null);
  const lastWsFrameAtRef = useRef(0);
  const captureBusyRef = useRef(false);
  const frameRafRef = useRef<number | null>(null);
  const pendingFrameRef = useRef<{ data: string; width?: number; height?: number; windowName: string } | null>(null);

  const commitFrame = useCallback((fullData: string, extras: { width?: number; height?: number; windowName: string }) => {
    pendingFrameRef.current = { data: fullData, ...extras };
    if (frameRafRef.current !== null) return;
    frameRafRef.current = requestAnimationFrame(() => {
      frameRafRef.current = null;
      const next = pendingFrameRef.current;
      pendingFrameRef.current = null;
      if (!next || next.data === lastFrameRef.current) return;

      lastFrameRef.current = next.data;
      const blobUrl = dataUrlToBlobUrl(next.data);
      setScreenshot((prev) => { if (prev) revokeBlobUrl(prev); return blobUrl; });
      if (next.width && next.height) setResolution({ width: next.width, height: next.height });
      setScreenshotGallery((previous) => {
        // Reuse same blob URL for gallery to avoid duplicate object URLs; clone via same conversion would create new URL
        // We create a separate URL for gallery to allow independent revocation, but share the underlying data
        const galleryBlobUrl = dataUrlToBlobUrl(next.data);
        const nextGallery = [
          {
            id: String(++galleryIdRef.current),
            data: galleryBlobUrl,
            timestamp: nowTime(),
            width: next.width,
            height: next.height,
            window_name: next.windowName,
          },
          ...previous,
        ].slice(0, GALLERY_CAP);
        // Revoke evicted blob URLs to free memory
        if (previous.length >= GALLERY_CAP) {
          const evicted = previous.slice(GALLERY_CAP - 1);
          evicted.forEach((item) => revokeBlobUrl(item.data));
        }
        return nextGallery;
      });
    });
  }, []);

  const recordFrame = useCallback((fullData: string, extras: { width?: number; height?: number; windowName: string }, live: boolean) => {
    frameSequenceRef.current += 1;
    setLastUpdate(nowTime());
    setStatus("waiting");
    if (live) {
      lastWsFrameAtRef.current = Date.now();
      if (monitoringRef.current) setMonitoringStatus("connected");
      setWsLive(true);
    }
    commitFrame(fullData, extras);
  }, [commitFrame]);

  const captureScreenshot = useCallback(async (showStatus = true): Promise<boolean> => {
    if (!id || captureBusyRef.current) return false;
    // WS main link: skip HTTP when WS is live and recent (staleAfter = max(5s, interval*2.5))
    const staleAfter = Math.max(5000, interval * 2500);
    if (monitoringRef.current && Date.now() - lastWsFrameAtRef.current < staleAfter) {
      if (showStatus) setStatus("waiting");
      return false;
    }
    captureBusyRef.current = true;
    if (showStatus) {
      setStatus("capturing");
      setMonitoringStatus("capturing");
    }
    try {
      const data = await api.get<ScreenshotResponse>(paths.agents.screenshot(id, ""));
      const image = data.image || data.data || data.screenshot || "";
      if (!image) {
        if (showStatus) setStatus("error");
        return false;
      }

      const fullData = frameDataUrl(image);
      const captureId = data.capture_id || data.captured_at || fullData;
      if (captureId === lastCaptureIdRef.current) {
        if (showStatus) setStatus("waiting");
        return false;
      }
      lastCaptureIdRef.current = captureId;
      if (monitoringRef.current && captureId.startsWith("live-")) {
        // The HTTP fallback exposes the server's in-memory live frame. It is
        // proof that the stream is healthy, but not that WebSocket is healthy.
        setMonitoringStatus("connected");
      }
      recordFrame(fullData, {
        width: data.width,
        height: data.height,
        windowName: data.window_name || "Desktop",
      }, false);
      return true;
    } catch {
      if (showStatus) {
        setStatus("error");
        if (monitoringRef.current) setMonitoringStatus("offline");
      }
      return false;
    } finally {
      captureBusyRef.current = false;
    }
  }, [id, recordFrame, interval]);

  const waitForTask = useCallback(async (taskId: number | string) => {
    const deadline = Date.now() + TASK_TIMEOUT_MS;
    while (mountedRef.current && Date.now() < deadline) {
      const task = await api.get<TaskStatusResponse>(paths.agents.task(id, taskId));
      const taskStatus = String(task.status || "").toLowerCase();
      if (["completed", "success", "done"].includes(taskStatus)) return;
      if (["failed", "error", "cancelled"].includes(taskStatus)) {
        throw new Error(task.error || task.result || t("common.task_failed"));
      }
      await delay(1_000);
    }
    throw new Error(t("screen.capture_timeout"));
  }, [id, t]);

  const requestFreshCapture = useCallback(async (kind: "capture" | "window") => {
    if (!id || busyAction) return;
    const previousSequence = frameSequenceRef.current;
    setBusyAction(kind);
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      const endpoint = kind === "window" ? paths.agents.screenshotWindow(id) : paths.agents.screenshotTask(id);
      const queued = await api.post<TaskResponse>(endpoint, {});
      if (queued.task_id != null) await waitForTask(queued.task_id);

      for (let attempt = 0; mountedRef.current && attempt < 12; attempt += 1) {
        if (frameSequenceRef.current > previousSequence) break;
        await captureScreenshot(false);
        if (frameSequenceRef.current > previousSequence) break;
        await delay(500);
      }
      if (frameSequenceRef.current <= previousSequence) throw new Error(t("screen.capture_timeout"));
      setMonitoringStatus(monitoringRef.current ? "connected" : "waiting");
      toast.success(kind === "window" ? t("screen.window_capture_complete") : t("screen.capture_complete"));
    } catch (error) {
      setStatus("error");
      setMonitoringStatus(monitoringRef.current ? "connected" : "waiting");
      toast.error(formatThrownError(error));
    } finally {
      if (mountedRef.current) setBusyAction(null);
    }
  }, [busyAction, captureScreenshot, id, t, waitForTask]);

  const startMonitoring = async () => {
    if (!id || busyAction) return;
    setBusyAction("start");
    setMonitoringStatus("capturing");
    setStatus("waiting");
    try {
      await api.post(paths.agents.screenStart(id), { interval: String(interval), quality });
      monitoringRef.current = true;
      setMonitoring(true);
      setMonitoringStatus("waiting");
      toast.success(t("agents.screen_started"));
      void captureScreenshot(false);
    } catch (error) {
      monitoringRef.current = false;
      setMonitoring(false);
      setStatus("error");
      setMonitoringStatus("offline");
      toast.error(`${t("agents.screen_start_failed")}: ${formatThrownError(error)}`);
    } finally {
      if (mountedRef.current) setBusyAction(null);
    }
  };

  const stopMonitoring = async () => {
    if (!id || busyAction) return;
    setBusyAction("stop");
    try {
      await api.post(paths.agents.screenStop(id), {});
      monitoringRef.current = false;
      setMonitoring(false);
      setWsLive(false);
      setMonitoringStatus("waiting");
      setStatus("waiting");
      toast.success(t("agents.screen_stopped"));
    } catch (error) {
      toast.error(`${t("agents.screen_stop_failed")}: ${formatThrownError(error)}`);
    } finally {
      if (mountedRef.current) setBusyAction(null);
    }
  };

  const startTitleTrigger = async () => {
    if (!id) return;
    const match = triggerMatch.trim();
    if (!match) {
      toast.error(t("agents.screen_trigger_need_match"));
      return;
    }
    try {
      await api.post(paths.agents.screenTriggerStart(id), { match, interval: String(interval) });
      setTriggerOn(true);
      toast.success(t("agents.screen_trigger_started"));
    } catch (error) {
      toast.error(formatThrownError(error));
    }
  };

  const stopTitleTrigger = async () => {
    if (!id) return;
    try {
      await api.post(paths.agents.screenTriggerStop(id), {});
      setTriggerOn(false);
      toast.success(t("agents.screen_trigger_stopped"));
    } catch (error) {
      toast.error(formatThrownError(error));
    }
  };

  const handleDownloadScreenshot = (image?: string, filename?: string) => {
    const target = image || screenshot;
    if (!target) return;
    const link = document.createElement("a");
    link.href = target;
    link.download = filename || `screenshot_${id}_${Date.now()}.png`;
    link.click();
  };

  const openModal = (image: string) => {
    if (!image) return;
    setModalImage(image);
    setShowModal(true);
  };

  const activatePreview = (event: React.KeyboardEvent<HTMLDivElement>, image: string) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openModal(image);
    }
  };

  useVisibleInterval(() => {
    const staleAfter = Math.max(5_000, interval * 2_500);
    if (Date.now() - lastWsFrameAtRef.current < staleAfter) return;
    setWsLive(false);
    void captureScreenshot(false);
  }, monitoring && autoRefresh ? Math.max(3_000, interval * 1_000) : 0);

  useEffect(() => {
    lastCaptureIdRef.current = null;
    void captureScreenshot(false);
  }, [captureScreenshot]);

  useEffect(() => {
    if (prevIdRef.current === id) return;
    const previousId = prevIdRef.current;
    if (monitoringRef.current && previousId) {
      monitoringRef.current = false;
      void api.post(paths.agents.screenStop(previousId), {}).catch(() => {});
    }
    setMonitoring(false);
    setMonitoringStatus("waiting");
    setWsLive(false);
    setScreenshot((prev) => { if (prev) revokeBlobUrl(prev); return null; });
    setScreenshotGallery((prev) => { prev.forEach((item) => revokeBlobUrl(item.data)); return []; });
    setResolution(null);
    lastFrameRef.current = null;
    lastCaptureIdRef.current = null;
    prevIdRef.current = id;
  }, [id]);

  useEffect(() => {
    if (!id) return;
    return subscribe((message) => {
      if (String(message.agent_id || "") !== id) return;
      if (message.type === "screenshot" && monitoringRef.current && message.data) {
        const fullData = frameDataUrl(String(message.data), String(message.mime || "image/png"));
        recordFrame(fullData, { windowName: "Desktop" }, true);
        return;
      }
      if (message.type === "screen_stream_error" && monitoringRef.current) {
        monitoringRef.current = false;
        setMonitoring(false);
        setWsLive(false);
        setStatus("error");
        setMonitoringStatus("offline");
        toast.error(String(message.error || t("agents.screen_start_failed")));
      }
    });
  }, [id, recordFrame, subscribe, t]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      if (frameRafRef.current !== null) cancelAnimationFrame(frameRafRef.current);
      if (monitoringRef.current && prevIdRef.current) {
        monitoringRef.current = false;
        void api.post(paths.agents.screenStop(prevIdRef.current), {}).catch(() => {});
      }
      setScreenshot((prev) => { if (prev) revokeBlobUrl(prev); return prev; });
      setScreenshotGallery((prev) => { prev.forEach((item) => revokeBlobUrl(item.data)); return prev; });
    };
  }, []);

  const indicator = (() => {
    if (status === "capturing") return { color: "bg-warning", text: t("screen.capturing"), icon: <Spinner size="xs" /> };
    if (status === "error") return { color: "bg-destructive", text: t("screen.error"), icon: <TriangleAlert className="size-3" /> };
    if (monitoring && monitoringStatus === "connected") {
      return { color: "bg-success", text: t("agents.rdp_connected"), icon: <span className="size-1.5 rounded-full bg-current" /> };
    }
    return { color: "bg-muted-foreground", text: t("agents.rdp_standby"), icon: <span className="size-1.5 rounded-full bg-current" /> };
  })();
  const capturing = busyAction === "capture" || busyAction === "window";

  return (
    <PageContainer variant="workspace" className="h-full px-4 py-3 sm:px-6">
      <div className="flex h-full min-h-0 flex-col">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 flex-wrap items-center gap-2.5">
            <h1 className="flex items-center gap-2 text-lg font-bold text-foreground">
              <Monitor className="size-4" aria-hidden="true" />
              {t("agents.screen_title")}
            </h1>
            <Badge variant="secondary" className="max-w-48 truncate font-mono text-xs">{id}</Badge>
            {resolution && (
              <Badge variant="outline" className="flex items-center gap-1 text-xs">
                <Maximize2 className="size-3.5" aria-hidden="true" />
                {resolution.width} × {resolution.height}
              </Badge>
            )}
          </div>

          <div className="flex flex-wrap items-center gap-2">
            {!monitoring ? (
              <Button onClick={() => void startMonitoring()} disabled={busyAction !== null}>
                {busyAction === "start" ? <Spinner size="xs" /> : <Play className="size-4" />}
                {t("agents.screen_start")}
              </Button>
            ) : (
              <Button variant="destructive" onClick={() => void stopMonitoring()} disabled={busyAction !== null}>
                {busyAction === "stop" ? <Spinner size="xs" /> : <Square className="size-4" />}
                {t("agents.screen_stop")}
              </Button>
            )}
            <Button variant="outline" onClick={() => void requestFreshCapture("capture")} disabled={busyAction !== null}>
              <Camera className="size-4" />
              {t("agents.screen_capture")}
            </Button>
            <Button variant="outline" onClick={() => void requestFreshCapture("window")} disabled={busyAction !== null}>
              <Maximize2 className="size-4" />
              {t("agents.screen_window_capture")}
            </Button>
            <Button variant="secondary" onClick={() => handleDownloadScreenshot()} disabled={!screenshot}>
              <Download className="size-4" />
              {t("agents.screen_download")}
            </Button>
          </div>
        </div>

        <div className="grid min-h-0 flex-1 grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_19rem]">
          <Card className="flex min-h-[28rem] min-w-0 flex-col overflow-hidden lg:min-h-0">
            <div className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-muted/70 px-4 py-2.5">
              <div className="flex items-center gap-2">
                <Monitor className="size-4" aria-hidden="true" />
                <span className="text-sm font-medium text-foreground">{t("agents.screen_live_view")}</span>
              </div>
              <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                <span className={`flex items-center gap-1.5 ${status === "error" ? "text-destructive" : ""}`}>
                  <span className={`size-2 rounded-full ${indicator.color}`} aria-hidden="true" />
                  {indicator.icon}
                  {indicator.text}
                </span>
                <span className="hidden items-center gap-1 sm:flex">
                  <Clock className="size-3.5" aria-hidden="true" />
                  {lastUpdate}
                </span>
                {wsLive && (
                  <Tooltip>
                    <TooltipTrigger render={<span className="flex items-center gap-1 text-chart-1" />}>
                      <Zap className="size-3.5" aria-hidden="true" /> WS
                    </TooltipTrigger>
                    <TooltipContent>{t("agents.screen_ws_live")}</TooltipContent>
                  </Tooltip>
                )}
              </div>
            </div>

            <div
              className="relative flex flex-1 items-center justify-center overflow-hidden bg-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              role={screenshot ? "button" : undefined}
              tabIndex={screenshot ? 0 : undefined}
              aria-label={screenshot ? t("agents.screen_alt_full") : undefined}
              onClick={() => screenshot && openModal(screenshot)}
              onKeyDown={(event) => screenshot && activatePreview(event, screenshot)}
            >
              {screenshot ? (
                <SafeImg
                  src={safeImageSrc(screenshot)}
                  alt={t("agents.screenshot")}
                  width={resolution?.width || undefined}
                  height={resolution?.height || undefined}
                  style={{ aspectRatio: resolution ? `${resolution.width} / ${resolution.height}` : undefined }}
                  className="max-h-full max-w-full object-contain"
                  loading="eager"
                  decoding="async"
                />
              ) : (
                <EmptyState icon={Monitor} title={t("agents.screen_no_screenshots")} message={`${t("agents.screen_start")} / ${t("agents.screen_capture")}`} />
              )}
              {capturing && (
                <div className="absolute inset-0 flex items-center justify-center bg-background/75 backdrop-blur-sm">
                  <div className="flex flex-col items-center gap-3 rounded-xl border border-border bg-card px-6 py-5 shadow-lg">
                    <Spinner size="xl" />
                    <span className="text-sm font-medium text-foreground">{t("screen.waiting_for_agent")}</span>
                  </div>
                </div>
              )}
            </div>
          </Card>

          <div className="flex min-h-0 flex-col gap-3 overflow-y-auto">
            <Card className="shrink-0 p-4">
              <div className="mb-3 flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <span>{t("agents.screen_controls")}</span>
                <span className={`flex items-center gap-1.5 font-normal normal-case ${monitoring ? "text-success" : "text-muted-foreground"}`}>
                  <StatusDot tone={monitoring ? "success" : "muted"} size="xs" pulse={monitoring} />
                  {monitoring ? t("screen.live") : t("screen.off")}
                </span>
              </div>

              <div className="space-y-3">
                <label className="block">
                  <span className="mb-1.5 flex items-center gap-1 text-xs text-muted-foreground">
                    <Clock className="size-3.5" aria-hidden="true" />
                    {t("agents.screen_interval")}
                  </span>
                  <Select value={String(interval)} onValueChange={(value) => value !== null && setInterval_(Number(value))} disabled={monitoring || busyAction === "start"}>
                    <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="1">1s · {t("screen.high_cpu")}</SelectItem>
                      <SelectItem value="3">3s</SelectItem>
                      <SelectItem value="5">5s</SelectItem>
                      <SelectItem value="10">10s</SelectItem>
                    </SelectContent>
                  </Select>
                </label>

                <label className="block">
                  <span className="mb-1.5 block text-xs text-muted-foreground">{t("screen.quality")}</span>
                  <Select value={quality} onValueChange={(value) => value !== null && setQuality(value as ScreenQuality)} disabled={monitoring || busyAction === "start"}>
                    <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="low">{t("screen.quality_low")}</SelectItem>
                      <SelectItem value="medium">{t("screen.quality_medium")}</SelectItem>
                      <SelectItem value="high">{t("screen.quality_high")}</SelectItem>
                    </SelectContent>
                  </Select>
                </label>

                <div className="flex items-center justify-between gap-3 rounded-lg bg-muted/60 px-3 py-2.5">
                  <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                    <RotateCw className="size-3.5" aria-hidden="true" />
                    {t("agents.screen_auto_refresh")}
                  </span>
                  <Switch checked={autoRefresh} onCheckedChange={setAutoRefresh} />
                </div>

                <div className="border-t border-border pt-3">
                  <span className="mb-1.5 block text-xs text-muted-foreground">{t("agents.screen_trigger")}</span>
                  <Input value={triggerMatch} onChange={(event) => setTriggerMatch(event.target.value)} placeholder={t("agents.screen_trigger_placeholder")} className="mb-2" />
                  {!triggerOn ? (
                    <Button size="sm" variant="outline" className="w-full" onClick={() => void startTitleTrigger()}>{t("agents.screen_trigger_start")}</Button>
                  ) : (
                    <Button size="sm" variant="destructive" className="w-full" onClick={() => void stopTitleTrigger()}>{t("agents.screen_trigger_stop")}</Button>
                  )}
                </div>

                {resolution && (
                  <div className="border-t border-border pt-3">
                    <span className="mb-1.5 flex items-center gap-1 text-xs text-muted-foreground">
                      <Maximize2 className="size-3.5" aria-hidden="true" />
                      {t("screen.resolution")}
                    </span>
                    <div className="rounded-lg bg-muted px-3 py-2 font-mono text-sm text-foreground">{resolution.width} × {resolution.height}</div>
                  </div>
                )}
              </div>
            </Card>

            <Card className="flex min-h-48 flex-1 flex-col p-4">
              <div className="mb-3 flex shrink-0 items-center justify-between">
                <div className="flex items-center gap-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  <Images className="size-3.5" aria-hidden="true" />
                  {t("agents.screen_gallery")}
                </div>
                <Badge variant="secondary" className="text-xs">{screenshotGallery.length}</Badge>
              </div>
              {screenshotGallery.length > 0 ? (
                <div className="grid flex-1 grid-cols-2 content-start gap-2 overflow-y-auto pr-1">
                  {screenshotGallery.map((item) => (
                    <div
                      key={item.id}
                      className="group relative cursor-pointer overflow-hidden rounded-lg border border-border bg-muted outline-none transition-colors hover:border-primary focus-visible:ring-2 focus-visible:ring-ring"
                      role="button"
                      tabIndex={0}
                      aria-label={`${t("agents.screen_alt_thumb")} ${item.timestamp}`}
                      onClick={() => openModal(item.data)}
                      onKeyDown={(event) => activatePreview(event, item.data)}
                    >
                      <SafeImg src={safeImageSrc(item.data)} alt={t("agents.screen_alt_thumb")} className="aspect-video h-auto w-full object-cover" loading="lazy" decoding="async" />
                      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-1.5 py-1 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus:opacity-100">{item.timestamp}</div>
                      <Button
                        size="icon-xs"
                        variant="secondary"
                        aria-label={t("agents.screen_download")}
                        onClick={(event) => {
                          event.stopPropagation();
                          handleDownloadScreenshot(item.data, `screen_${id}_${item.id}.png`);
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
          </div>
        </div>
      </div>

      <Dialog open={showModal} onOpenChange={setShowModal}>
        <DialogContent showCloseButton={false} className="max-h-[95vh] border-0 bg-transparent p-0 ring-0 sm:max-w-[95vw]">
          <div className="relative flex max-h-[95vh] items-center justify-center">
            <div className="absolute right-2 top-2 z-10 flex items-center gap-2">
              <Button size="icon" variant="secondary" aria-label={t("agents.screen_download")} onClick={() => handleDownloadScreenshot(modalImage)}>
                <Download className="size-4" />
              </Button>
              <Button size="icon" variant="secondary" aria-label={t("common.close")} onClick={() => setShowModal(false)}>
                <X className="size-4" />
              </Button>
            </div>
            <SafeImg src={safeImageSrc(modalImage)} alt={t("agents.screen_alt_full")} className="max-h-[95vh] max-w-full rounded-lg object-contain shadow-2xl" loading="eager" decoding="async" />
          </div>
        </DialogContent>
      </Dialog>
    </PageContainer>
  );
}
