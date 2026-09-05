"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { nowTime } from "@/lib/utils";
import { useWS } from "@/lib/wsContext";
import { dataUrlToBlob, dataUrlToBlobUrl, revokeBlobUrl } from "@/lib/screenImage";

type TKey = (key: string, params?: Record<string, string | number>) => string;

export type MonitorStatus = "connected" | "offline" | "capturing" | "waiting";
export type CaptureStatus = "waiting" | "capturing" | "error";
export type ScreenQuality = "low" | "medium" | "high";
export type BusyAction = "start" | "stop" | "capture" | "window" | null;

export interface ScreenshotItem {
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

export interface ScreenMonitorOpts {
  interval: number;
  quality: ScreenQuality;
  autoRefresh: boolean;
  t: TKey;
}

/**
 * Screen capture state machine: live monitor start/stop, on-demand captures,
 * WS frame ingest with rAF coalescing, capped gallery, title trigger, and
 * lightbox/download helpers. Settings (interval/quality/autoRefresh) stay
 * with the caller and are read live via a ref.
 */
export function useScreenMonitor(agentId: string, opts: ScreenMonitorOpts) {
  const { t } = opts;
  const optsRef = useRef(opts);
  optsRef.current = opts;

  const { subscribe } = useWS();

  const [monitoring, setMonitoring] = useState(false);
  const [monitoringStatus, setMonitoringStatus] = useState<MonitorStatus>("waiting");
  const [status, setStatus] = useState<CaptureStatus>("waiting");
  const [busyAction, setBusyAction] = useState<BusyAction>(null);
  const [screenshot, setScreenshot] = useState<string | null>(null);
  const [screenshotGallery, setScreenshotGallery] = useState<ScreenshotItem[]>([]);
  const [lastUpdate, setLastUpdate] = useState("-");
  const [triggerOn, setTriggerOn] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [modalImage, setModalImage] = useState("");
  const [resolution, setResolution] = useState<{ width: number; height: number } | null>(null);
  const [wsLive, setWsLive] = useState(false);

  const monitoringRef = useRef(false);
  const mountedRef = useRef(true);
  const prevIdRef = useRef(agentId);
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
      const sharedBlob = dataUrlToBlob(next.data);
      const blobUrl = sharedBlob ? URL.createObjectURL(sharedBlob) : dataUrlToBlobUrl(next.data);
      const galleryBlobUrl = sharedBlob ? URL.createObjectURL(sharedBlob) : dataUrlToBlobUrl(next.data);
      setScreenshot((prev) => { if (prev) revokeBlobUrl(prev); return blobUrl; });
      if (next.width && next.height) setResolution({ width: next.width, height: next.height });
      setScreenshotGallery((previous) => {
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
    const id = agentId;
    if (!id || captureBusyRef.current) return false;
    // WS main link: skip HTTP when WS is live and recent (staleAfter = max(5s, interval*2.5))
    const staleAfter = Math.max(5000, optsRef.current.interval * 2500);
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
  }, [agentId, recordFrame]);

  const waitForTask = useCallback(async (taskId: number | string) => {
    const deadline = Date.now() + TASK_TIMEOUT_MS;
    while (mountedRef.current && Date.now() < deadline) {
      const task = await api.get<TaskStatusResponse>(paths.agents.task(agentId, taskId));
      const taskStatus = String(task.status || "").toLowerCase();
      if (["completed", "success", "done"].includes(taskStatus)) return;
      if (["failed", "error", "cancelled"].includes(taskStatus)) {
        throw new Error(task.error || task.result || t("common.task_failed"));
      }
      await delay(1_000);
    }
    throw new Error(t("screen.capture_timeout"));
  }, [agentId, t]);

  const requestFreshCapture = useCallback(async (kind: "capture" | "window") => {
    if (!agentId || busyAction) return;
    const previousSequence = frameSequenceRef.current;
    setBusyAction(kind);
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      const endpoint = kind === "window" ? paths.agents.screenshotWindow(agentId) : paths.agents.screenshotTask(agentId);
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
  }, [busyAction, captureScreenshot, agentId, t, waitForTask]);

  const startMonitoring = useCallback(async () => {
    const { interval, quality } = optsRef.current;
    if (!agentId || busyAction) return;
    setBusyAction("start");
    setMonitoringStatus("capturing");
    setStatus("waiting");
    try {
      await api.post(paths.agents.screenStart(agentId), { interval: String(interval), quality });
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
  }, [agentId, busyAction, captureScreenshot, t]);

  const stopMonitoring = useCallback(async () => {
    if (!agentId || busyAction) return;
    setBusyAction("stop");
    try {
      await api.post(paths.agents.screenStop(agentId), {});
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
  }, [agentId, busyAction, t]);

  const startTitleTrigger = useCallback(async (matchRaw: string) => {
    const { interval } = optsRef.current;
    if (!agentId) return;
    const match = matchRaw.trim();
    if (!match) {
      toast.error(t("agents.screen_trigger_need_match"));
      return;
    }
    try {
      await api.post(paths.agents.screenTriggerStart(agentId), { match, interval: String(interval) });
      setTriggerOn(true);
      toast.success(t("agents.screen_trigger_started"));
    } catch (error) {
      toast.error(formatThrownError(error));
    }
  }, [agentId, t]);

  const stopTitleTrigger = useCallback(async () => {
    if (!agentId) return;
    try {
      await api.post(paths.agents.screenTriggerStop(agentId), {});
      setTriggerOn(false);
      toast.success(t("agents.screen_trigger_stopped"));
    } catch (error) {
      toast.error(formatThrownError(error));
    }
  }, [agentId, t]);

  const downloadScreenshot = useCallback((image?: string, filename?: string) => {
    const target = image || screenshot;
    if (!target) return;
    const link = document.createElement("a");
    link.href = target;
    link.download = filename || `screenshot_${agentId}_${Date.now()}.png`;
    link.click();
  }, [screenshot, agentId]);

  const openModal = useCallback((image: string) => {
    if (!image) return;
    setModalImage(image);
    setShowModal(true);
  }, []);

  const closeModal = useCallback(() => setShowModal(false), []);

  const activatePreview = useCallback((event: React.KeyboardEvent<HTMLElement>, image: string) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openModal(image);
    }
  }, [openModal]);

  useVisibleInterval(() => {
    const staleAfter = Math.max(5_000, optsRef.current.interval * 2_500);
    if (Date.now() - lastWsFrameAtRef.current < staleAfter) return;
    setWsLive(false);
    void captureScreenshot(false);
  }, monitoring && opts.autoRefresh ? Math.max(3_000, opts.interval * 1_000) : 0);

  useEffect(() => {
    lastCaptureIdRef.current = null;
    void captureScreenshot(false);
  }, [captureScreenshot]);

  useEffect(() => {
    if (prevIdRef.current === agentId) return;
    const previousId = prevIdRef.current;
    if (monitoringRef.current && previousId) {
      monitoringRef.current = false;
      void api.post(paths.agents.screenStop(previousId), {}).catch(() => {});
    }
    setMonitoring(false);
    setMonitoringStatus("waiting");
    setWsLive(false);
    setTriggerOn(false);
    setScreenshot((prev) => { if (prev) revokeBlobUrl(prev); return null; });
    setScreenshotGallery((prev) => { prev.forEach((item) => revokeBlobUrl(item.data)); return []; });
    setResolution(null);
    lastFrameRef.current = null;
    lastCaptureIdRef.current = null;
    prevIdRef.current = agentId;
  }, [agentId]);

  useEffect(() => {
    if (!agentId) return;
    return subscribe((message) => {
      if (String(message.agent_id || "") !== agentId) return;
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
  }, [agentId, recordFrame, subscribe, t]);

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

  return {
    monitoring, monitoringStatus, status, busyAction,
    screenshot, screenshotGallery, lastUpdate, resolution, wsLive,
    triggerOn, showModal, modalImage,
    captureScreenshot, requestFreshCapture,
    startMonitoring, stopMonitoring,
    startTitleTrigger, stopTitleTrigger,
    downloadScreenshot, openModal, closeModal, activatePreview,
  };
}
