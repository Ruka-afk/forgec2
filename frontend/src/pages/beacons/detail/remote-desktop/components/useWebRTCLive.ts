import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";

/**
 * Live remote-desktop transport: the browser sends a recvonly H.264 offer over
 * the authenticated operator WebSocket, the agent answers from its single-use
 * signaling socket, and media flows directly or through the server's TURN
 * relay. Input events reuse the same socket; they never become tasks.
 *
 * Snapshot streaming in useRemoteDesktop is untouched, so a beacon without the
 * optional WebRTC module (or without ffmpeg) can fall back to JPEG frames.
 */

type LiveStatus = "idle" | "connecting" | "connected" | "error";

export interface LivePreset {
  value: "720p" | "1080p";
  width: number;
  height: number;
  bitrateKbps: number;
  fps: number;
}

export const LIVE_PRESETS: LivePreset[] = [
  { value: "720p", width: 1280, height: 720, bitrateKbps: 2500, fps: 30 },
  { value: "1080p", width: 1920, height: 1080, bitrateKbps: 4500, fps: 30 },
];

export function getLivePreset(value: string): LivePreset {
  return LIVE_PRESETS.find((preset) => preset.value === value) ?? LIVE_PRESETS[0];
}

export function buildOperatorWsUrl(
  sessionId: string,
  location: Pick<Location, "protocol" | "host"> & Partial<Pick<Location, "hostname">> = window.location,
): string {
  const path = `/ws/rd/operator?session=${encodeURIComponent(sessionId)}`;
  const envURL = import.meta.env.VITE_FORGEC2_WS_URL;
  if (envURL) return envURL + path;
  const scheme = location.protocol === "https:" ? "wss" : "ws";
  const backendPort = import.meta.env.VITE_FORGEC2_BACKEND_PORT;
  const hostname = location.hostname || location.host.replace(/:\d+$/, "");
  const host = backendPort ? `${hostname}:${backendPort}` : location.host;
  return `${scheme}://${host}${path}`;
}

interface IceServerResponse {
  urls: string[];
  username?: string;
  credential?: string;
}

interface StartResponse {
  session_id?: string;
  ice_servers?: IceServerResponse[];
  expires_at?: string;
}

interface SignalMessage {
  type?: string;
  sdp?: string;
  candidate?: RTCIceCandidateInit;
  state?: string;
  reason?: string;
  error?: string;
}

function normalizeIceServers(servers: IceServerResponse[] | undefined): RTCIceServer[] {
  const out: RTCIceServer[] = [];
  for (const server of servers ?? []) {
    const urls = (server.urls ?? []).filter((url) => typeof url === "string" && url.length > 0);
    if (urls.length === 0) continue;
    out.push({
      urls,
      ...(server.username ? { username: server.username } : {}),
      ...(server.credential ? { credential: server.credential } : {}),
    });
  }
  return out;
}

export function useWebRTCLive(agentId: string) {
  const { t } = useI18n();
  const [active, setActive] = useState(false);
  const [status, setStatus] = useState<LiveStatus>("idle");
  const [error, setError] = useState("");
  const [liveWidth, setLiveWidth] = useState(0);
  const [liveHeight, setLiveHeight] = useState(0);
  const [livePreset, setLivePreset] = useState<LivePreset["value"]>("720p");

  const videoRef = useRef<HTMLVideoElement>(null);
  const pcRef = useRef<RTCPeerConnection | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const activeRef = useRef(false);
  const stoppingRef = useRef(false);
  const pendingLocalCandidatesRef = useRef<RTCIceCandidateInit[]>([]);
  const pendingRemoteCandidatesRef = useRef<RTCIceCandidateInit[]>([]);
  const answerTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sessionIdRef = useRef("");

  const presets = useMemo(
    () =>
      LIVE_PRESETS.map((preset) => ({
        ...preset,
        label:
          preset.value === "1080p"
            ? t("agents.rdp_live_1080p")
            : t("agents.rdp_live_720p"),
      })),
    [t],
  );

  const clearAnswerTimer = useCallback(() => {
    if (answerTimerRef.current) {
      clearTimeout(answerTimerRef.current);
      answerTimerRef.current = null;
    }
  }, []);

  const cleanupLocal = useCallback(() => {
    activeRef.current = false;
    setActive(false);
    clearAnswerTimer();
    const ws = wsRef.current;
    wsRef.current = null;
    if (ws) {
      ws.onopen = null;
      ws.onmessage = null;
      ws.onerror = null;
      ws.onclose = null;
      try {
        ws.close();
      } catch {
        /* already closed */
      }
    }
    const pc = pcRef.current;
    pcRef.current = null;
    if (pc) {
      pc.ontrack = null;
      pc.onicecandidate = null;
      pc.onconnectionstatechange = null;
      try {
        pc.close();
      } catch {
        /* already closed */
      }
    }
    const video = videoRef.current;
    if (video) {
      video.srcObject = null;
    }
    pendingLocalCandidatesRef.current = [];
    pendingRemoteCandidatesRef.current = [];
  }, [clearAnswerTimer]);

  const fail = useCallback(
    (message: string) => {
      const sessionId = sessionIdRef.current;
      sessionIdRef.current = "";
      cleanupLocal();
      setStatus("error");
      setError(message);
      // A failure can happen before the operator socket opens; in that case
      // closing the socket cannot end the server session, so use the
      // id-scoped fallback stop.
      if (agentId && sessionId) {
        void api
          .postJson(paths.agents.webrtcStop(agentId), { session_id: sessionId })
          .catch(() => {});
      }
    },
    [agentId, cleanupLocal],
  );

  const sendSignal = useCallback((message: Record<string, unknown>): boolean => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    try {
      ws.send(JSON.stringify(message));
      return true;
    } catch {
      return false;
    }
  }, []);

  const sendInput = useCallback(
    (event: { type: "move" | "click"; x: number; y: number } | { type: "key"; key: string }): boolean =>
      sendSignal({ type: "input", event }),
    [sendSignal],
  );

  const stop = useCallback(async () => {
    const sessionId = sessionIdRef.current;
    sessionIdRef.current = "";
    stoppingRef.current = true;
    sendSignal({ type: "stop" });
    cleanupLocal();
    setStatus("idle");
    setError("");
    if (agentId) {
      // Fallback for an agent whose signaling socket died before it saw the
      // close: the server queues screen_webrtc_stop. The session id keeps a
      // delayed fallback from stopping a newer session on the same agent.
      await api
        .postJson(paths.agents.webrtcStop(agentId), sessionId ? { session_id: sessionId } : {})
        .catch(() => {});
    }
    stoppingRef.current = false;
  }, [agentId, cleanupLocal, sendSignal]);

  const start = useCallback(
    async (presetValue: string) => {
      if (!agentId) return;
      if (typeof RTCPeerConnection === "undefined" || typeof WebSocket === "undefined") {
        fail(t("agents.rdp_live_unsupported"));
        return;
      }
      if (activeRef.current) {
        await stop();
      }
      const preset = getLivePreset(presetValue);
      setLivePreset(preset.value);
      setLiveWidth(preset.width);
      setLiveHeight(preset.height);
      setError("");
      setStatus("connecting");

      let response: StartResponse;
      try {
        response = await api.postJson<StartResponse>(paths.agents.webrtcStart(agentId), {
          width: preset.width,
          height: preset.height,
          fps: preset.fps,
          bitrate_kbps: preset.bitrateKbps,
          encoder: "auto",
        });
      } catch (err) {
        setStatus("error");
        setError(formatThrownError(err));
        throw err;
      }
      const sessionId = String(response.session_id || "");
      if (!sessionId) {
        setStatus("error");
        setError(t("agents.rdp_live_start_failed"));
        return;
      }
      sessionIdRef.current = sessionId;

      let pc: RTCPeerConnection;
      try {
        pc = new RTCPeerConnection({ iceServers: normalizeIceServers(response.ice_servers) });
      } catch (err) {
        fail(formatThrownError(err));
        return;
      }
      pcRef.current = pc;
      activeRef.current = true;
      setActive(true);
      pc.addTransceiver("video", { direction: "recvonly" });

      pc.ontrack = (event) => {
        const video = videoRef.current;
        if (!video) return;
        video.srcObject = event.streams[0] ?? new MediaStream([event.track]);
        video.muted = true;
        void video.play().catch(() => {});
      };
      pc.onicecandidate = (event) => {
        if (!event.candidate) return;
        const candidate = event.candidate.toJSON();
        const ws = wsRef.current;
        if (ws?.readyState === WebSocket.OPEN) {
          sendSignal({ type: "ice", candidate });
        } else {
          pendingLocalCandidatesRef.current.push(candidate);
        }
      };
      pc.onconnectionstatechange = () => {
        if (pc.connectionState === "failed") {
          fail(t("agents.rdp_live_connection_failed"));
        }
      };

      let ws: WebSocket;
      try {
        ws = new WebSocket(buildOperatorWsUrl(sessionId));
      } catch {
        fail(t("agents.rdp_live_start_failed"));
        return;
      }
      wsRef.current = ws;
      ws.onopen = () => {
        void (async () => {
          try {
            const offer = await pc.createOffer();
            await pc.setLocalDescription(offer);
            const sdp = pc.localDescription?.sdp ?? offer.sdp ?? "";
            ws.send(JSON.stringify({ type: "offer", sdp }));
            for (const candidate of pendingLocalCandidatesRef.current) {
              ws.send(JSON.stringify({ type: "ice", candidate }));
            }
            pendingLocalCandidatesRef.current = [];
            clearAnswerTimer();
            answerTimerRef.current = setTimeout(() => {
              if (activeRef.current && pc.remoteDescription === null) {
                fail(t("agents.rdp_live_timeout"));
              }
            }, 20_000);
          } catch (err) {
            fail(formatThrownError(err));
          }
        })();
      };
      ws.onmessage = (event) => {
        let message: SignalMessage;
        try {
          message = JSON.parse(String(event.data)) as SignalMessage;
        } catch {
          return;
        }
        switch (message.type) {
          case "answer":
            if (!message.sdp) return;
            clearAnswerTimer();
            void pc
              .setRemoteDescription({ type: "answer", sdp: message.sdp })
              .then(async () => {
                const queued = pendingRemoteCandidatesRef.current;
                pendingRemoteCandidatesRef.current = [];
                for (const candidate of queued) {
                  await pc.addIceCandidate(candidate).catch(() => {});
                }
              })
              .catch((err) => fail(formatThrownError(err)));
            break;
          case "ice":
            if (!message.candidate) return;
            if (pc.remoteDescription === null) {
              pendingRemoteCandidatesRef.current.push(message.candidate);
            } else {
              void pc.addIceCandidate(message.candidate).catch(() => {});
            }
            break;
          case "state":
            if (message.state === "connected" || message.state === "streaming") {
              setStatus("connected");
            }
            break;
          case "waiting":
            setStatus("connecting");
            break;
          case "error":
            fail(message.reason || message.error || t("agents.rdp_live_connection_failed"));
            break;
          case "session_end": {
            if (stoppingRef.current) return;
            fail(message.reason ? `${t("agents.rdp_live_ended")} (${message.reason})` : t("agents.rdp_live_ended"));
            break;
          }
        }
      };
      ws.onerror = () => {
        if (activeRef.current && !stoppingRef.current) {
          fail(t("agents.rdp_live_connection_failed"));
        }
      };
      ws.onclose = () => {
        if (activeRef.current && !stoppingRef.current) {
          fail(t("agents.rdp_live_ended"));
        }
      };
    },
    [agentId, clearAnswerTimer, fail, sendSignal, stop, t],
  );

  const markPlaying = useCallback(() => {
    if (activeRef.current) setStatus("connected");
  }, []);

  // Closing the signaling socket ends the server session, which tells the
  // agent to kill ffmpeg. No API call is made from unload paths.
  useEffect(() => {
    return () => {
      if (activeRef.current) cleanupLocal();
    };
  }, [cleanupLocal]);

  useEffect(() => {
    if (!agentId) return;
    cleanupLocal();
    setStatus("idle");
    setError("");
  }, [agentId, cleanupLocal]);

  return {
    t,
    active,
    status,
    error,
    presets,
    livePreset,
    setLivePreset,
    liveWidth,
    liveHeight,
    videoRef,
    start,
    stop,
    sendInput,
    markPlaying,
  };
}
