# Remote Desktop / Screen Streaming

ForgeC2's remote-desktop surface has two operator modes: the original
**snapshot stream** (JPEG frames plus `remote_input` tasks) and an optional
**live WebRTC mode** (H.264 at 720p30, optionally 1080p30). It is still an
assist channel, not a full RDP/VNC stack, and this document records the
transport and security boundaries.

## Modes

### Snapshot mode (default, unchanged)

```
POST /agents/:id/screen/start        (agentCmd: agents.write + operator)
        -> screen_stream_start task -> agent captures frames
agent -> beacon result types screen_frame
        -> server caches the frame (screenFrames[agent]) and fans out a
           {"type":"screenshot", ...} message over the dashboard WebSocket
GET  /agents/:id/screenshot          (agentCmd: agents.write + operator)
        -> HTTP fallback: latest cached frame, or newest on-disk PNG
POST /api/agents/:id/input           (agentCmd: agents.write + operator)
        -> remote_input task ({"type":"move|click|key","x","y","key"})
POST /agents/:id/screen/stop         (agentCmd: agents.write + operator)
```

The dashboard socket is a **screen-content channel**, not just a task
notifier, and is gated accordingly (see *Gates*).

### Live WebRTC mode

```
POST /agents/:id/webrtc/start        (agentCmd: agents.write + operator)
        -> creates one session, mints a single-use signaling token + TURN
           credentials, and queues screen_webrtc_start (encrypted task Data)
agent   -> GET /ws/rd/agent?session=&token=&agent=   (public route, token auth)
browser -> GET /ws/rd/operator?session=              (operator auth + ownership)
browser -> offer / ice / input / stop                (text signaling)
agent   -> answer / ice / state / error              (text signaling)
media   -> browser  <==== DTLS-SRTP H.264 ====>  agent
           direct host candidates or embedded Pion TURN relay
POST /agents/:id/webrtc/stop         (operator ownership checked)
```

The browser is always the offerer with a `recvonly` video transceiver; the
agent is the answerer with a `sendonly` H.264 track. The server never proxies
media. The signaling WebSocket carries input events for latency, so a live
session does **not** create a task per mouse move:

- Server-side input validation (`remoteInputRequest.normalize`), the same
  40 events/second fixed-window limiter keyed by operator + agent, and the
  authenticated operator socket still apply.
- Input is forwarded only while both peers are attached. Rate-limited events
  are dropped; the session open/close and agent connect are audited, not every
  pointer sample.

Lifecycle:

| Event | Result |
| --- | --- |
| Session created | 90-second window for both sockets; TTL default 1h (max 24h) |
| Operator or agent disconnects | session ends, both sockets close, agent kills ffmpeg |
| `{"type":"stop"}` from either side | same teardown |
| Operator stop endpoint | ends the session and queues `screen_webrtc_stop` as a fallback |
| Server shutdown | all sessions end, TURN listener closes |
| Second operator/agent for the same session | rejected; one operator and one agent per screen |
| WebRTC disabled | `POST .../webrtc/start` returns `503` |

The `screen_webrtc_start` task Data is encrypted at rest (it contains the
one-shot token and TURN credentials) and on the wire. The browser response
contains the TURN username/credential but never the agent token.

## Live media settings

| Setting | Default | Accepted |
| --- | --- | --- |
| Resolution | 1280×720 | 640–1920 width, 360–1080 height, even values, 1080p ceiling |
| Frame rate | 30 | 15–30 |
| Bitrate | 2500 kbps (720p), 4500 kbps (1080p) | 500–8000 kbps |
| Encoder | `auto` | `libx264`, `h264_nvenc`, `h264_qsv`, `h264_amf`, `h264_videotoolbox` |
| Container / RTP | H.264 baseline, 90000 Hz, RTP/SAVP | — |
| Keyframe interval | 2 seconds | — |

The agent starts ffmpeg only after the PeerConnection reaches `connected`, so
the first IDR/SPS/PPS is not dropped. Software encoding uses
`libx264 -tune zerolatency -bf 0 -sc_threshold 0` with repeated headers.
Hardware encoders are opt-in; they receive no x264-private options and may
fail on old drivers (the operator sees the ffmpeg error on the stage).

ffmpeg capture backends:

| OS | Backend | Notes |
| --- | --- | --- |
| Windows | `gdigrab` (`desktop`) | cursor drawn; no elevation needed for the current desktop |
| Linux | `x11grab` | `$DISPLAY` (default `:0.0`), size from `xrandr`; **Wayland is not supported** |
| macOS | `avfoundation` (`1:none`) | screen device index can vary; first pass is best-effort |

`FORGEC2_FFMPEG` can point at a specific ffmpeg binary. Without ffmpeg (or
without the optional agent `webrtc` module) the start task fails honestly and
snapshot mode still works.

## TURN

`webrtc.enabled: true` starts an embedded Pion TURN/STUN listener at startup.
It is off by default because it opens a UDP port.

```yaml
webrtc:
  enabled: true
  turn_addr: ":3478"          # UDP listen address
  public_ip: ""               # relay address advertised to peers; empty = first non-loopback IPv4
  realm: forgec2
  relay_min_port: 49152       # open UDP 3478 and this range in the firewall
  relay_max_port: 65535
  session_ttl_seconds: 3600
  max_sessions: 32
```

- `public_ip` must be reachable by both peers when they are not on the same
  LAN. The ICE URL uses the host the operator requested; the relay allocation
  uses `public_ip` (or the detected local IPv4).
- TURN credentials are generated per session and removed when the session
  ends (the auth map also carries a 5-minute post-expiry slack for the timer
  path). TURN's own allocation lifetime still bounds stale relays.
- `turn_addr` is UDP-only in this pass. Direct host candidates remain available
  when TURN is blocked.

## Gates

Every screen-content read passes three gates; a request must clear all of them.

| Gate | Mechanism | Failure mode |
| --- | --- | --- |
| Operator identity | `requireOperator` (role `viewer`/empty rejected) | `403` |
| Tenant scope | `getAgentOrFail` / `tenantScope` | `404` |
| `agents.write` | `RequirePermission(db.PermAgentsWrite)` on the route group; `wsClientConn.canViewScreen` for socket fan-out | `403` / silently not delivered |

The operator signaling socket additionally checks that the session belongs to
the authenticated user; the agent socket consumes a single-use token bound to
the session and agent id. The token is removed on first use, so a replay fails.

## Limits and rates

| Limit | Value | Enforced in |
| --- | --- | --- |
| Viewers per agent (snapshot relay) | 8 | `RDHub.Join` + pre-upgrade check (`429`) |
| Viewer connections, global (snapshot relay) | 64 | `RDHub` (`429`) |
| Live sessions, global | 32 (configurable) | `webrtcHub.createSession` (`409`) |
| Live sessions per agent | 1 | `webrtcHub.createSession` (`409`) |
| Live operator/agent sockets | 1 each | `attachOperator` / `attachAgent` |
| Peer connect window | 90 seconds | session timer (`peer_timeout`) |
| Live session TTL | 1 hour default, 24h hard cap | session timer |
| Signaling message | 512 KiB websocket read limit; SDP ≤256 KiB; ICE candidate ≤8 KiB | handler |
| Buffered pre-agent frames | 128 messages | session (`agent_signaling_buffer_full`) |
| Input events | 40/second per (operator, agent), fixed window | `remoteInputLimiter` |
| Coordinates | ±32767 | `remoteInputRequest.normalize` |
| Key names | 1–32 characters, lowercased | `remoteInputRequest.normalize` |
| Snapshot websocket keepalive | ping every 30s, 5-minute read deadline | `handleRDWebSocket` |
| Live websocket keepalive | ping every 30s, 5-minute read deadline | `handleRDWebRTC*` |

## Audit trail

| Action | Written when |
| --- | --- |
| `screen_monitor_start` / `screen_monitor_stop` | snapshot monitor lifecycle accepted |
| `remote_input` | each accepted snapshot input event (rate limiter first) |
| `request_screenshot` | `POST /agents/:id/screenshot` task path |
| `screenshot_request` | `/rd/:id/screenshot` |
| `remote_desktop_open` / `remote_desktop_close` | legacy websocket relay join/leave |
| `webrtc_start` / `webrtc_stop` | live session start/stop accepted |
| `webrtc_open` / `webrtc_close` | operator live signaling socket opened/closed |
| `webrtc_agent_connect` | agent consumed the signaling token and attached |

Per-pointer input over the live socket is rate-limited but not audited
individually; the session records above bracket the interaction and avoid
turning every mouse sample into an audit-chain write.

## Known limits

- **Live control is still input-event mapping, not a virtual HID stack.** The
  agent uses the same `remoteInputDispatch` implementations as snapshot mode
  (Windows `SetCursorPos`/`mouse_event`/`keybd_event`, Linux
  `xdotool`/`ydotool`, macOS `osascript`). Modifier chords and some special keys
  remain best-effort.
- **Live video coordinates are not desktop pixels.** ffmpeg scales and pads to
  the selected preset, so the agent maps pointer events back to the native
  screen rectangle (including letterbox bars) before injection. This is exact
  on Windows and X11; the shared Linux/macOS injectors still have their
  existing snapshot-mode limitations for pointer events.
- **No multi-viewer, no recording, no 4K guarantee.** One operator and one
  screen per session; 1080p30 is the ceiling and is only realistic with a
  hardware H.264 encoder or a strong CPU.
- **Live media needs ffmpeg on the target.** The optional `webrtc` agent module
  is compiled in by default but can be excluded to keep Pion out of the binary;
  Win7-compatible builds always exclude it (the pinned Go 1.20 toolchain cannot
  compile Pion).
- **Wayland is not supported** by the Linux capture path in this pass.
- **macOS screen-capture permission** and the avfoundation device index are
  environmental preconditions; a failure surfaces as the ffmpeg error.
- Snapshot mode is unchanged and remains the fallback when live mode is
  disabled, the module is absent, or ffmpeg is unavailable.
- Reading the latest snapshot requires `agents.write` because the same route
  takes screenshots; listing stored screenshots is `agents.read`.

## Verification

```sh
# Session tokens/TURN credentials/validation (server)
go test ./internal/server/ -run 'TestNormalizeWebRTCStart|TestWebRTCSession|TestWebRTCTURNCredentials|TestDetectLocalIPv4' -count=1

# Agent URL/ffmpeg argument helpers
go test ./internal/payload/agent -run 'TestBuildAgentSignalURL|TestBuildScreenFFmpegArgs|TestEncoderArgsDoNotPassX264OptionsToHardware' -count=1

# Hub/input/rate-limit regression suite
go test ./internal/server/ -run 'TestRDHub|TestRemoteInput|TestHandleAgentRemoteInput|TestBroadcastScreenshot|TestScreenMonitor|TestScreenPage|TestTokenPage' -count=1

# Frontend helpers + hook
cd frontend && npx vitest run src/pages/beacons/detail/remote-desktop/components
```
