"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { useWS } from "@/lib/wsContext";

interface ScreenshotItem {
  id?: string;
  data?: string;
  timestamp?: string;
  width?: number;
  height?: number;
  window_name?: string;
}export default function ScreenPage({ params }: { params: Promise<{ id: string }> }) {
  const [id, setId] = useState<string>("");  const [monitoring, setMonitoring] = useState(false);  const [monitoringStatus, setMonitoringStatus] = useState<"connected" | "offline" | "capturing" | "waiting">("waiting");
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

  useEffect(() => {
    let cancelled = false;
    params.then(({ id }) => { if (!cancelled) setId(id); });
    return () => { cancelled = true; };
  }, [params]);

  const captureScreenshot = useCallback(async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/screenshot&format=json`);
      if (res.ok) {
        const data = await res.json();
        const imgData = data.image || data.data || data.screenshot || "";
        const width = data.width || 0;
        const height = data.height || 0;
        if (imgData) {
          const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;          setScreenshot(fullData);          setLastUpdate(new Date().toLocaleTimeString());
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
        } else {          setStatus("error");
          setMonitoringStatus("offline");
        }
      } else {
        setStatus("error");        setMonitoringStatus("offline");      }
    } catch {      setStatus("error");
      setMonitoringStatus("offline");
    }
  }, [id]);  const startMonitoring = async () => {
    if (!id) return;
    setMonitoring(true);
    setMonitoringStatus("capturing");
    try {      const body = new URLSearchParams();      body.append("interval", (interval * 1000).toString());
      await fetch(`${API_BASE}?p=/agents/${id}/screen/start`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });      toast("Screen monitoring started", "success");
      } catch (e) { console.error("Screen: start monitoring failed", e);
      toast("Failed to start monitoring", "error");
    }
    await captureScreenshot();    if (autoRefresh) {
      timerRef.current = setInterval(captureScreenshot, interval * 1000);    }
  };

  const stopMonitoring = async () => {
    setMonitoring(false);
    setMonitoringStatus("waiting");
    if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }    try {
      await fetch(`${API_BASE}?p=/agents/${id}/screen/stop`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      toast("Screen monitoring stopped", "success");
    } catch (e) { console.error("Screen: stop monitoring failed", e); }
  };

  const handleManualCapture = async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/screenshot`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      if (res.ok) {
        const data = await res.json();
        const imgData = data.image || data.data || data.screenshot || "";
        const width = data.width || 0;        const height = data.height || 0;        if (imgData) {
          const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;
          setScreenshot(fullData);
          setLastUpdate(new Date().toLocaleTimeString());
          setStatus("waiting");          setMonitoringStatus("connected");
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
          toast("Screenshot captured", "success");
        } else {
          setStatus("error");
          setMonitoringStatus("offline");          toast("Failed to capture screenshot", "error");        }
      } else {        setStatus("error");
        setMonitoringStatus("offline");        toast(`Capture failed: ${res.status}`, "error");      }
    } catch (err) {      setStatus("error");
      setMonitoringStatus("offline");
      toast(String(err), "error");
    }  };

  const handleWindowScreenshot = async () => {
    if (!id) return;
    setStatus("capturing");
    setMonitoringStatus("capturing");
    try {      const res = await fetch(`${API_BASE}?p=/agents/${id}/screenshot_window`, {        method: "POST",        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      if (res.ok) {
        const data = await res.json();
        const imgData = data.image || data.data || data.screenshot || "";
        const width = data.width || 0;
        const height = data.height || 0;
        if (imgData) {
          const fullData = imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`;
          setScreenshot(fullData);
          setLastUpdate(new Date().toLocaleTimeString());
          setStatus("waiting");
          setMonitoringStatus("connected");          if (width && height) setResolution({ width, height });
          const galleryItem: ScreenshotItem = {
            id: String(++galleryIdRef.current),
            data: fullData,
            timestamp: new Date().toLocaleTimeString(),
            width,
            height,
            window_name: data.window_name || "Window Capture",
          };
          setScreenshotGallery(prev => [galleryItem, ...prev].slice(0, 50));
          toast("Window screenshot captured", "success");
        } else {
          setStatus("error");
          setMonitoringStatus("offline");
          toast("Failed to capture window", "error");
        }
      } else {
        setStatus("error");
        setMonitoringStatus("offline");
        toast(`Capture failed: ${res.status}`, "error");
      }
    } catch (err) {      setStatus("error");
      setMonitoringStatus("offline");
      toast(String(err), "error");    }
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
  };  const toast = useCallback((msg: string, type: string = "info") => {
    const el = document.createElement("div");
    el.className = `fixed top-4 right-4 z-[60] px-4 py-3 rounded-xl text-sm font-semibold shadow-2xl transition-all ${      type === "success" ? "bg-emerald-600 text-white" :
      type === "error" ? "bg-red-600 text-white" :      "bg-blue-600 text-white"
    }`;
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(() => el.remove(), 2500);
  }, []);

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
    if (status === "capturing") return { color: "bg-amber-400 animate-pulse", text: "Capturing...", icon: "fa-spinner fa-spin" };
    if (status === "error") return { color: "bg-red-500", text: "Error", icon: "fa-triangle-exclamation" };
    if (monitoring && monitoringStatus === "connected") return { color: "bg-emerald-400 animate-pulse", text: "Connected", icon: "fa-circle" };
    return { color: "bg-slate-400 dark:bg-slate-600", text: "Standby", icon: "fa-circle" };
  };

  const indicator = statusIndicator();

  return (
    <>
      <div className="flex flex-col h-[calc(100vh-4rem)]">        <div className="flex items-center justify-between mb-3 gap-3 flex-wrap">
          <div className="flex items-center gap-3">
            <a href={`/agents/${id}`} className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors">              <i className="fa-solid fa-arrow-left"></i>
            </a>
            <h1 className="text-lg font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
              <i className="fa-solid fa-desktop text-blue-500"></i>
              Screen Monitor
            </h1>
            <span className="text-xs text-slate-500 dark:text-slate-400 font-mono bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-lg">{id}</span>
            {resolution && (
              <span className="text-xs bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 px-2.5 py-1 rounded-lg flex items-center gap-1">
                <i className="fa-solid fa-expand text-slate-400"></i>{resolution.width} x {resolution.height}
              </span>
            )}          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {!monitoring ? (              <button onClick={startMonitoring}
                className="bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
                <i className="fa-solid fa-play"></i>
                Start Monitor
              </button>
            ) : (
              <button onClick={stopMonitoring}
                className="bg-red-600 hover:bg-red-700 text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
                <i className="fa-solid fa-stop"></i>                Stop
              </button>
            )}
            <button onClick={handleManualCapture} disabled={status === "capturing"}
              className="bg-blue-600 hover:bg-blue-700 disabled:opacity-50 text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <i className="fa-solid fa-camera"></i>
              Capture
            </button>
            <button onClick={handleWindowScreenshot} disabled={status === "capturing"}
              className="bg-purple-600 hover:bg-purple-700 disabled:opacity-50 text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <i className="fa-solid fa-window-maximize"></i>              Window            </button>
            <button onClick={() => screenshot && handleDownloadScreenshot()} disabled={!screenshot}
              className="bg-slate-600 hover:bg-slate-700 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5 font-medium shadow-sm">
              <i className="fa-solid fa-download"></i>              Save
            </button>          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-4 gap-4 flex-1 min-h-0">          <div className="lg:col-span-3 flex flex-col min-h-0">
            <div className="ui-card overflow-hidden shadow-sm flex-1 flex flex-col min-h-0">
              <div className="bg-slate-50 dark:bg-slate-700/50 px-4 py-2.5 flex items-center justify-between border-b border-[var(--border)] shrink-0">                <div className="flex items-center gap-2">
                  <i className="fa-solid fa-desktop text-blue-500"></i>
                  <span className="text-sm font-medium text-slate-900 dark:text-slate-100">Live View</span>                </div>
                <div className="flex items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                  <span className={`flex items-center gap-1.5 ${status === "error" ? "text-red-500" : ""}`}>
                    <span className={`w-2 h-2 rounded-full ${indicator.color}`}></span>
                    <i className={`fa-solid ${indicator.icon}`}></i>
                    {indicator.text}                  </span>
                  <span className="hidden sm:inline text-slate-400">
                    <i className="fa-regular fa-clock mr-1"></i>
                    {lastUpdate}
                  </span>
                  {monitoring && (
                    <span className="ml-1 w-2 h-2 bg-emerald-500 rounded-full animate-pulse" title="Monitoring active"></span>
                  )}
                  {wsLive && (
                    <span className="text-[10px] text-emerald-400 flex items-center gap-1" title="WebSocket live stream">
                      <i className="fa-solid fa-bolt"></i> WS
                    </span>
                  )}
                </div>
              </div>              <div className="relative bg-slate-900 dark:bg-slate-900 flex-1 flex items-center justify-center cursor-pointer overflow-hidden"
                onClick={() => screenshot && openModal(screenshot)}>
                {screenshot ? (
                  <img src={screenshot} alt="Screenshot" className="max-w-full max-h-full object-contain" />
                ) : (
                  <div className="text-center text-slate-500 py-20">
                    <i className="fa-solid fa-desktop text-5xl mb-4 opacity-30"></i>
                    <p className="text-base font-medium">No screenshot yet</p>                    <p className="text-sm mt-1 text-slate-400">Click &quot;Start Monitor&quot; or &quot;Capture&quot; to begin</p>                  </div>
                )}
                {status === "capturing" && (
                  <div className="absolute inset-0 flex items-center justify-center bg-black/40 backdrop-blur-sm">
                    <div className="flex flex-col items-center gap-3">
                      <div className="w-12 h-12 border-3 border-white/20 border-t-white rounded-full animate-spin" />
                      <span className="text-white text-sm font-medium">Capturing...</span>
                    </div>
                  </div>                )}
              </div>
            </div>
          </div>          <div className="lg:col-span-1 flex flex-col gap-3 min-h-0 overflow-y-auto">
            <div className="ui-card p-4 shadow-sm shrink-0">
              <div className="text-xs text-slate-500 dark:text-slate-400 mb-3 font-semibold uppercase tracking-wider flex items-center justify-between">
                <span>Controls</span>
                <span className={`flex items-center gap-1.5 text-[10px] font-normal ${monitoring ? "text-emerald-500" : "text-slate-400"}`}>                  <span className={`w-1.5 h-1.5 rounded-full ${monitoring ? "bg-emerald-500 animate-pulse" : "bg-slate-300 dark:bg-slate-600"}`}></span>                  {monitoring ? "LIVE" : "OFF"}
                </span>
              </div>
              <div className="space-y-3">
                <div>
                  <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5 flex items-center gap-1">
                    <i className="fa-regular fa-clock"></i>
                    Interval
                  </label>
                  <select value={interval} onChange={(e) => setInterval_(Number(e.target.value))}
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-3 h-9 text-sm focus:outline-none focus:border-blue-500 dark:text-slate-100">
                    <option value={1}>1s (High CPU)</option>
                    <option value={3}>3s</option>
                    <option value={5}>5s</option>
                    <option value={10}>10s</option>                  </select>
                </div>                <div className="flex items-center justify-between">
                  <span className="text-xs text-slate-500 dark:text-slate-400 flex items-center gap-1">
                    <i className="fa-solid fa-rotate"></i>
                    Auto Refresh
                  </span>
                  <button onClick={() => setAutoRefresh(!autoRefresh)}
                    className={`relative w-11 h-6 rounded-full transition-colors ${autoRefresh ? "bg-blue-600 dark:bg-blue-500" : "bg-slate-300 dark:bg-slate-600"}`}>
                    <span className={`absolute top-0.5 left-0.5 w-5 h-5 bg-white rounded-full transition-transform shadow-sm ${autoRefresh ? "translate-x-5" : ""}`} />
                  </button>
                </div>
                <div className="border-t border-slate-100 dark:border-slate-700 pt-3">
                  <label className="block text-xs text-slate-500 dark:text-slate-400 mb-2 flex items-center gap-1">
                    <i className="fa-solid fa-image"></i> Quick Capture                  </label>
                  <div className="grid grid-cols-2 gap-2">                    <button onClick={handleManualCapture} disabled={status === "capturing"}
                      className="px-2 py-1.5 bg-blue-100 hover:bg-blue-200 dark:bg-blue-900/30 dark:hover:bg-blue-900/50 text-blue-700 dark:text-blue-400 rounded-lg text-xs transition-colors flex items-center justify-center gap-1 disabled:opacity-50">
                      <i className="fa-solid fa-camera"></i> Desktop                    </button>
                    <button onClick={handleWindowScreenshot} disabled={status === "capturing"}
                      className="px-2 py-1.5 bg-purple-100 hover:bg-purple-200 dark:bg-purple-900/30 dark:hover:bg-purple-900/50 text-purple-700 dark:text-purple-400 rounded-lg text-xs transition-colors flex items-center justify-center gap-1 disabled:opacity-50">
                      <i className="fa-solid fa-window-maximize"></i> Window
                    </button>
                  </div>
                </div>
                {resolution && (
                  <div className="border-t border-slate-100 dark:border-slate-700 pt-3">
                    <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1.5 flex items-center gap-1">
                      <i className="fa-solid fa-expand"></i> Resolution                    </label>
                    <div className="bg-slate-50 dark:bg-slate-700 rounded-lg px-3 py-2 text-sm font-mono text-slate-700 dark:text-slate-200">
                      {resolution.width} &times; {resolution.height}
                    </div>
                  </div>
                )}
              </div>
            </div>

            <div className="ui-card p-4 shadow-sm flex-1 min-h-0 flex flex-col">
              <div className="flex items-center justify-between mb-3 shrink-0">
                <div className="text-xs text-slate-500 dark:text-slate-400 font-semibold uppercase tracking-wider flex items-center gap-1">
                  <i className="fa-solid fa-images"></i> Gallery
                </div>
                <span className="text-[10px] text-slate-400 dark:text-slate-500 bg-slate-100 dark:bg-slate-700 px-2 py-0.5 rounded">{screenshotGallery.length}</span>
              </div>
              {screenshotGallery.length > 0 ? (
                <div className="grid grid-cols-2 gap-2 overflow-y-auto flex-1 max-h-64 lg:max-h-full">
                  {screenshotGallery.map((item) => (
                    <div key={item.id} className="relative group cursor-pointer rounded-xl overflow-hidden border-2 border-transparent hover:border-blue-400 transition-colors bg-slate-100 dark:bg-slate-700"
                      onClick={() => openModal(item.data || "")}>
                      <img src={item.data} alt="Thumbnail" className="w-full h-auto aspect-video object-cover" />                      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent px-1.5 py-1 text-[9px] text-white opacity-0 group-hover:opacity-100 transition-opacity">
                        {item.timestamp}
                      </div>
                      <div className="absolute top-1 right-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          onClick={(e) => { e.stopPropagation(); handleDownloadScreenshot(item.data, `screen_${id}_${item.timestamp?.replace(/[:\s/]/g, "_")}.png`); }}
                          className="bg-black/60 hover:bg-black/80 text-white p-1 rounded text-[9px] transition-colors"
                        >
                          <i className="fa-solid fa-download"></i>
                        </button>
                      </div>
                    </div>
                  ))}
                </div>              ) : (
                <div className="text-center py-6 text-slate-400 dark:text-slate-500 text-xs flex-1 flex flex-col items-center justify-center">                  <i className="fa-regular fa-image text-xl mb-2 opacity-40"></i>                  <p>No screenshots yet</p>
                </div>
              )}            </div>
          </div>        </div>
      </div>      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 backdrop-blur-sm" onClick={() => setShowModal(false)}>
          <div className="relative max-w-[95vw] max-h-[95vh]" onClick={(e) => e.stopPropagation()}>            <div className="absolute -top-12 left-0 flex items-center gap-3">
              <button
                onClick={() => handleDownloadScreenshot(modalImage)}
                className="text-white/80 hover:text-white p-2 rounded-full bg-black/30 hover:bg-black/50 transition-colors"
                title="Download"
              >
                <i className="fa-solid fa-download text-lg"></i>
              </button>
              <button
                onClick={() => setShowModal(false)}                className="text-white/80 hover:text-white p-2 rounded-full bg-black/30 hover:bg-black/50 transition-colors"
                title="Close"
              >
                <i className="fa-solid fa-xmark text-lg"></i>
              </button>
            </div>
            <img src={modalImage} alt="Full size" className="max-w-full max-h-[90vh] rounded-lg shadow-2xl" />
          </div>
        </div>
      )}
    </>
  );
}