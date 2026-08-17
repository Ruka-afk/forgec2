"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import { useParams } from "next/navigation";
import { useWS } from "@/lib/wsContext";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import { Clock, Compass, Expand, Keyboard, Maximize2, Mouse, Play, Square, TriangleAlert, Users, Wifi, Zap } from "lucide-react";
import { IconBadge } from "@/components/ui/icon-badge";
import { useI18n } from "@/lib/i18n";
import { safeImageSrc } from "@/lib/safeUrl";
import { isExperimentalDesktop } from "../_components/session-quality";
import { implantBlocksDest } from "../../_components/implant-version";

interface ResolutionOption {
  value: string;
  label: string;
  interval: number;
}

const INTERVAL_BY_RES: Record<string, number> = {
  low: 2000,
  medium: 1000,
  high: 500,
  ultra: 250,
};

export default function RemoteDesktopPage() {
  const { t } = useI18n();
  const params = useParams();
  const id = params.id as string;

  // Experimental: screenshot stream + remote_input tasks — not a full RDP stack.

  const RESOLUTIONS: ResolutionOption[] = useMemo(
    () => [
      { value: "low", label: t("agents.rdp_low"), interval: INTERVAL_BY_RES.low },
      { value: "medium", label: t("agents.rdp_medium"), interval: INTERVAL_BY_RES.medium },
      { value: "high", label: t("agents.rdp_high"), interval: INTERVAL_BY_RES.high },
      { value: "ultra", label: t("agents.rdp_ultra"), interval: INTERVAL_BY_RES.ultra },
    ],
    [t],
  );

  const [monitoring, setMonitoring] = useState(false);
  const monitoringRef = useRef(false);
  const [status, setStatus] = useState<"waiting" | "capturing" | "connected" | "error">("waiting");
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
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastFrameRef = useRef<string | null>(null);
  const frameRafRef = useRef<number | null>(null);
  const pendingFrameRef = useRef<{ data: string; width?: number; height?: number } | null>(null);
  const cursorRafRef = useRef<number | null>(null);
  const pendingCursorRef = useRef<{ x: number; y: number } | null>(null);
  const captureBusyRef = useRef(false);
  const { subscribe } = useWS();

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
      setLastUpdate(new Date().toLocaleTimeString());
      setStatus("connected");
      setScreenData(p.data);
      if (p.width) setNativeWidth(p.width);
      if (p.height) setNativeHeight(p.height);
    });
  }, []);

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
      toast.error(t("agents.rdp_capture_failed"));
    } finally {
      captureBusyRef.current = false;
    }
  }, [id, t, commitFrame]);

  const startMonitoring = async () => {
    if (!id) return;
    if (versionBlocked) {
      toast.error(t("agents.version_unknown_dest"));
      return;
    }
    setMonitoring(true);
    monitoringRef.current = true;
    setStatus("capturing");
    try {
      await api.post(paths.agents.screenStart(id), { interval: String(pollInterval) });
      await captureFrame();
      if (timerRef.current) clearInterval(timerRef.current);
      timerRef.current = setInterval(() => { if (shouldCapture()) captureFrame(); }, pollInterval);
    } catch {
      setStatus("error");
      setMonitoring(false);
      toast.error(t("agents.rdp_start_failed"));
    }
  };

  const stopMonitoring = async () => {
    if (!id) return;
    setMonitoring(false);
    monitoringRef.current = false;
    setStatus("waiting");
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }
    try {
      await api.post(paths.agents.screenStop(id));
    } catch {
      toast.error(t("agents.rdp_stop_failed"));
    }
  };

  useEffect(() => {
    if (!monitoring) return;
    if (timerRef.current) clearInterval(timerRef.current);
    timerRef.current = setInterval(() => { if (shouldCapture()) captureFrame(); }, pollInterval);
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, [resolution, monitoring, captureFrame, pollInterval, shouldCapture]);

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
      api.post(paths.agents.screenStop(id)).catch((e) => { console.error("RemoteDesktop stop failed:", e); });
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

  const handleClick = async (e: React.MouseEvent<HTMLDivElement>) => {
    if (!monitoring || !id) return;
    const { x, y } = getRelativeCoords(e);
    try {
      await api.postJson(paths.agents.remoteInput(id), { type: "click", x, y });
    } catch {
      toast.error(t("agents.rdp_capture_failed"));
    }
  };

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

  const toggleFullscreen = async () => {
    if (!containerRef.current) return;
    if (!document.fullscreenElement) {
      await containerRef.current.requestFullscreen();
    } else {
      await document.exitFullscreen();
    }
  };

  useEffect(() => {
    const handler = () => setIsFullscreen(!!document.fullscreenElement);
    document.addEventListener("fullscreenchange", handler);
    return () => document.removeEventListener("fullscreenchange", handler);
  }, []);

  const statusConfig = {
    waiting: {
      color: "bg-muted-foreground dark:bg-muted-foreground",
      text: t("agents.rdp_standby"),
      icon: <span className="w-1.5 h-1.5 rounded-full bg-current" />,
    },
    capturing: {
      color: "bg-warning animate-pulse",
      text: t("agents.rdp_starting"),
      icon: <Spinner size="xs" />,
    },
    connected: {
      color: "bg-success/60 animate-pulse",
      text: t("agents.rdp_connected"),
      icon: <span className="w-1.5 h-1.5 rounded-full bg-current" />,
    },
    error: {
      color: "bg-destructive",
      text: t("agents.rdp_error"),
      icon: <TriangleAlert className="w-3 h-3" />,
    },
  };

  const indicator = statusConfig[status];

  return (
    <PageContainer className="space-y-4">
      <Card className="p-3 border-warning/40 bg-warning/10 text-sm text-warning-foreground flex items-start gap-2">
        <TriangleAlert className="w-4 h-4 mt-0.5 shrink-0" />
        <div>
          <div className="font-semibold">{t("agents.rdp_experimental_title")}</div>
          <div className="text-xs text-muted-foreground">{t("agents.rdp_experimental_desc")}</div>
        </div>
      </Card>
      <div className="flex flex-col h-[calc(100vh-4rem)]">
      <div className="flex items-center justify-between mb-3 gap-3 flex-wrap shrink-0">
        <div className="flex items-center gap-3">
            <h1 className="text-lg font-bold text-foreground flex items-center gap-2">
            <Users className="w-4 h-4" />
            {t("agents.rdp_title")}
            {isExperimentalDesktop("remote-desktop") ? (
              <span className="text-(--fs-micro) font-normal text-warning">{t("generate.quality_experimental")}</span>
            ) : null}
          </h1>
          <Badge variant="secondary" className="text-xs text-muted-foreground font-mono bg-muted/50 px-2 py-0.5 rounded-lg">
            {id}
          </Badge>
          {nativeWidth > 0 && nativeHeight > 0 && (
            <Badge variant="secondary" className="text-xs bg-muted/50 text-muted-foreground px-2.5 py-1 rounded-lg flex items-center gap-1">
              <Maximize2 className="w-4 h-4" />
              {nativeWidth} x {nativeHeight}
            </Badge>
          )}
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={resolution} onValueChange={(v) => { if (v !== null) setResolution(v); }} disabled={monitoring}>
            <SelectTrigger className="w-[180px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {RESOLUTIONS.map((r) => (
                <SelectItem key={r.value} value={r.value}>
                  {r.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Tooltip>
            <TooltipTrigger render={<Button
                variant="outline"
                size="icon-lg"
                onClick={toggleFullscreen}
                aria-label={isFullscreen ? t("agents.rdp_exit_fullscreen") : t("agents.rdp_enter_fullscreen")}
              />}>
              {isFullscreen ? <Compass className="w-4 h-4" /> : <Expand className="w-4 h-4" />}
            </TooltipTrigger>
            <TooltipContent>{isFullscreen ? t("agents.rdp_exit_fullscreen") : t("agents.rdp_fullscreen")}</TooltipContent>
          </Tooltip>
          {!monitoring ? (
            <Button
              onClick={() => void startMonitoring()}
              disabled={versionBlocked}
              title={versionBlocked ? t("agents.version_unknown_dest") : undefined}
            >
              <Play className="w-4 h-4" />
              {t("agents.rdp_start")}
            </Button>
          ) : (
            <Button
              variant="destructive"
              onClick={stopMonitoring}
            >
              <Square className="w-4 h-4" />
              {t("agents.rdp_stop")}
            </Button>
          )}
        </div>
      </div>

      <div className="flex-1 min-h-0 flex flex-col">
        <Card
          className="overflow-hidden shadow-sm flex-1 flex flex-col min-h-0 rounded-lg"
        >
          <div className="bg-muted/50 px-4 py-2.5 flex items-center justify-between border-b border-border shrink-0">
            <div className="flex items-center gap-2">
              <Users className="w-4 h-4" />
                <span className="text-sm font-medium text-foreground">
                {t("agents.rdp_interactive")}
              </span>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <span
                className={`flex items-center gap-1.5 ${status === "error" ? "text-destructive" : ""}`}
              >
                <span className={`w-2 h-2 rounded-full ${indicator.color}`}></span>
                {indicator.icon}
                {indicator.text}
              </span>
                <span className="hidden sm:inline text-muted-foreground/70">
                  <Clock className="w-4 h-4" />
                {lastUpdate}
              </span>
              {monitoring && (
                <Tooltip>
                  <TooltipTrigger>
                    <span
                      className="ml-1 w-2 h-2 bg-success rounded-full animate-pulse"
                    ></span>
                  </TooltipTrigger>
                  <TooltipContent>{t("agents.rdp_monitoring_active")}</TooltipContent>
                </Tooltip>
              )}
              {wsLive && (
                <Tooltip>
                  <TooltipTrigger>
                    <span
                      className="text-(--fs-micro-sm) text-chart-1 flex items-center gap-1"
                    >
                      <Zap className="w-4 h-4" /> WS
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>{t("agents.rdp_ws_live")}</TooltipContent>
                </Tooltip>
              )}
              {monitoring && (
                <span className="text-(--fs-micro-sm) text-primary flex items-center gap-1">
                  <Mouse className="w-4 h-4" /> {t("agents.rdp_interactive_label")}
                </span>
              )}
            </div>
          </div>

          <div
            className="relative bg-card dark:bg-card flex-1 flex items-center justify-center overflow-hidden select-none"
            onClick={handleClick}
            onMouseMove={handleMouseMove}
            onMouseLeave={() => setShowCursor(false)}
          >
            {screenData ? (
              <>
                <img
                  ref={imgRef}
                  src={safeImageSrc(screenData)}
                  alt={t("agents.rdp_title")}
                  className="max-w-full max-h-full object-contain"
                  draggable={false}
                  loading="lazy"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }}
                  onLoad={(e) => {
                    const img = e.currentTarget;
                    setNativeWidth(img.naturalWidth || nativeWidth);
                    setNativeHeight(img.naturalHeight || nativeHeight);
                  }}
                />
                {showCursor && monitoring && (
                  <div
                    className="absolute pointer-events-none"
                    style={{
                      left: mouseX,
                      top: mouseY,
                      width: 16,
                      height: 16,
                      transform: "translate(-2px, -2px)",
                    }}
                  >
                    <svg
                      width="16"
                      height="16"
                      viewBox="0 0 16 16"
                      fill="none"
                      className="drop-shadow-lg"
                    >
                      <path
                        d="M1 1L6 15L8 9L14 8L1 1Z"
                        fill="white"
                        stroke="black"
                        strokeWidth="1.5"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                )}
                {monitoring && (
                  <div className="absolute bottom-3 left-3 flex items-center gap-2 text-(--fs-xs-sm) text-white/70 bg-black/50 backdrop-blur-sm px-3 py-1.5 rounded-lg pointer-events-none">
                    <Mouse className="w-4 h-4" />
                    {t("agents.rdp_click_to_interact")}
                    <span className="text-white/40 mx-1">|</span>
                    <Keyboard className="w-4 h-4" />
                    {t("agents.rdp_keyboard_active")}
                  </div>
                )}
              </>
            ) : (
                <div className="text-center text-muted-foreground/70 py-20">
                  <EmptyState icon={Users} title={t("agents.rdp_standby")} message={t("agents.rdp_start_hint")} />
              </div>
            )}

            {status === "capturing" && (
              <div className="absolute inset-0 flex items-center justify-center bg-black/40 backdrop-blur-sm">
                <div className="flex flex-col items-center gap-3">
                  <Spinner size="xl" color="white" />
                  <span className="text-white text-sm font-medium">{t("agents.rdp_starting_session")}</span>
                </div>
              </div>
            )}
          </div>
        </Card>

        <div className="mt-3 grid grid-cols-1 sm:grid-cols-3 gap-3 shrink-0">
          <Card className="px-4 py-3 flex flex-row items-center gap-3 rounded-2xl">
            <IconBadge icon={Mouse} color="primary" size="md" />
            <div>
              <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
                {t("agents.rdp_mouse")}
              </div>
              <div className="text-sm text-foreground">
                {monitoring ? t("agents.rdp_click_move_active") : t("agents.rdp_inactive")}
              </div>
            </div>
          </Card>
          <Card className="px-4 py-3 flex flex-row items-center gap-3 rounded-2xl">
            <IconBadge icon={Keyboard} color="primary" size="md" />
            <div>
              <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
                {t("agents.rdp_keyboard")}
              </div>
              <div className="text-sm text-foreground">
                {monitoring ? t("agents.rdp_active") : t("agents.rdp_inactive")}
              </div>
            </div>
          </Card>
          <Card className="px-4 py-3 flex flex-row items-center gap-3 rounded-2xl">
            <IconBadge icon={Wifi} color="primary" size="md" />
            <div>
              <div className="text-(--fs-xs-sm) text-muted-foreground uppercase tracking-wider font-semibold">
                {t("agents.rdp_connection")}
              </div>
              <div className="text-sm text-foreground">
                <span
                   className={`inline-flex items-center gap-1 ${monitoring ? "text-success" : "text-muted-foreground/70"}`}
                >
                  <span
                    className={`w-1.5 h-1.5 rounded-full ${monitoring ? "bg-success animate-pulse" : "bg-muted-foreground/50"}`}
                  ></span>
                  {monitoring ? t("agents.rdp_session_active") : t("agents.rdp_disconnected")}
                </span>
              </div>
            </div>
          </Card>
        </div>
      </div>
      </div>
    </PageContainer>
  );
}

