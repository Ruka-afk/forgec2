package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/forgec2/forgec2/internal/malleable"
	"github.com/gin-gonic/gin"
)

func (s *Server) applyMalleableProfile(c *gin.Context, body []byte) {
	s.configMu.RLock()
	mp := s.cfg.Malleable
	s.configMu.RUnlock()
	if !mp.Enabled {
		c.Data(http.StatusOK, "application/json", body)
		return
	}

	// Apply named profile preset if set
	if mp.ProfileName != "" {
		presets := malleable.PredefinedProfiles()
		if profile, ok := presets[mp.ProfileName]; ok {
			s.applyProfilePreset(c, body, profile)
			return
		}
		// v2 file-based profile (data/profiles/<name>.json): apply its
		// ServerOutput chain + headers so custom chains actually encode.
		if out, ct, headers, ok := s.applyV2FileProfile(mp.ProfileName, body); ok {
			for k, v := range headers {
				c.Header(k, v)
			}
			c.Header("Content-Type", ct)
			c.Status(http.StatusOK)
			c.Writer.Write(out)
			return
		}
	}

	statusCode := mp.StatusCode
	if statusCode < 100 || statusCode > 599 {
		statusCode = http.StatusOK
	}

	wrapped := string(body)
	if mp.Prepend != "" {
		wrapped = mp.Prepend + wrapped
	}
	if mp.Append != "" {
		wrapped = wrapped + mp.Append
	}

	for k, v := range mp.Headers {
		c.Header(k, v)
	}

	ct := mp.ContentType
	if ct == "" {
		ct = "application/json"
	}
	c.Header("Content-Type", ct)

	c.Status(statusCode)
	c.Writer.WriteString(wrapped)
}

// profileDataDir resolves the profiles directory without panicking on a
// partially-constructed Server (tests). It replaces the silent recover()
// guards previously wrapped around implantDataDir().
func (s *Server) profileDataDir() string {
	if s != nil && s.cfg != nil {
		if d := s.implantDataDir(); d != "" {
			return d
		}
	}
	return "data"
}

// stripMalleableRequest removes the request-side malleable prepend/append that
// the agent wraps around its OUTGOING beacon body (see wrapMalleableRequest on
// the agent). Mirrors stripMalleableWrapping but uses the request-side tokens
// from the server's malleable config. The operation is the inverse of the
// agent's wrap, so the enclosed JSON envelope is recovered unchanged. When no
// request-side transform is configured the body is returned untouched.
func (s *Server) stripMalleableRequest(raw []byte) []byte {
	s.configMu.RLock()
	prepend := s.cfg.Malleable.RequestPrepend
	appendStr := s.cfg.Malleable.RequestAppend
	s.configMu.RUnlock()

	switch {
	case prepend == "" && appendStr == "":
		return raw
	case prepend == "":
		return bytes.TrimSuffix(raw, []byte(appendStr))
	case appendStr == "":
		return bytes.TrimPrefix(raw, []byte(prepend))
	default:
		raw = bytes.TrimPrefix(raw, []byte(prepend))
		return bytes.TrimSuffix(raw, []byte(appendStr))
	}
}

// activePlacements returns the server-side placement list: global config
// Placements JSON first, else the active profile file's placements.
func (s *Server) activePlacements() []malleable.PlacementV2 {
	s.configMu.RLock()
	raw := s.cfg.Malleable.Placements
	name := s.cfg.Malleable.ProfileName
	s.configMu.RUnlock()
	if strings.TrimSpace(raw) != "" {
		var pls []malleable.PlacementV2
		if err := json.Unmarshal([]byte(raw), &pls); err == nil && len(pls) > 0 {
			return pls
		}
	}
	if strings.TrimSpace(name) == "" {
		return nil
	}
	dir := s.profileDataDir()
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, strings.TrimSpace(name))
	if sanitized == "" {
		return nil
	}
	rawBytes, err := os.ReadFile(filepath.Join(dir, "profiles", sanitized+".json"))
	if err != nil {
		return nil
	}
	v2, err := malleable.MigrateProfileJSON(rawBytes, sanitized)
	if err != nil || v2 == nil {
		return nil
	}
	return v2.Placements
}

// extractPlacedBody tries each configured placement location and returns the
// decoded envelope bytes when one yields a plausible beacon frame. It returns
// nil when no placement is configured or none decodes, in which case the
// caller falls back to the request body. Cover-copy model: the placed value
// decodes (chain reverse) to the padded+wrapped body, so strip+unpad apply
// before the cheap '{' plausibility check — no decrypt attempted here.
func (s *Server) extractPlacedBody(c *gin.Context) []byte {
	pls := s.activePlacements()
	if len(pls) == 0 {
		return nil
	}
	tryDecode := func(encoded string, chain string) []byte {
		if strings.TrimSpace(encoded) == "" {
			return nil
		}
		var steps []malleable.Transform
		for _, st := range malleable.ParseWire(chain) {
			steps = append(steps, malleable.Transform{Type: st.Type, Value: st.Value})
		}
		decoded := []byte(encoded)
		if len(steps) > 0 {
			tb := &malleable.TransformBlock{Transforms: steps}
			out, err := tb.Apply(decoded, false)
			if err != nil {
				return nil
			}
			decoded = out
		}
		decoded = s.stripMalleableRequest(decoded)
		decoded = s.stripBodyPadding(decoded)
		trimmed := bytes.TrimSpace(decoded)
		if len(trimmed) > 0 && trimmed[0] == '{' {
			return decoded
		}
		return nil
	}
	for _, pl := range pls {
		kind, name, ok := malleable.ParsePlacementTarget(pl.Target)
		if !ok {
			continue
		}
		switch kind {
		case "query":
			// Scan every query value: the param name may rotate per
			// beacon through the profile's parameter pool.
			for _, vals := range c.Request.URL.Query() {
				for _, v := range vals {
					if dec := tryDecode(v, pl.Chain); dec != nil {
						return dec
					}
				}
			}
			_ = name
		case "cookie":
			// Same scan-all policy as query for rotated names.
			for _, ck := range c.Request.Cookies() {
				if dec := tryDecode(ck.Value, pl.Chain); dec != nil {
					return dec
				}
			}
		case "header":
			if dec := tryDecode(c.GetHeader(name), pl.Chain); dec != nil {
				return dec
			}
		default:
			continue
		}
	}
	return nil
}

// stripBodyPadding removes the agent's ContentLengthJitter padding (8-byte
// big-endian length prefix + random trailing bytes, see padBeaconBody on the
// agent) from an HTTP/WS beacon body. Bodies without the prefix — plain
// envelopes and any future transport that does not pad — decode the prefix to
// an absurd length and are returned untouched, so the strip is safe to apply
// unconditionally on every inbound beacon.
func (s *Server) stripBodyPadding(raw []byte) []byte {
	const prefixLen = 8
	if len(raw) < prefixLen+16 {
		return raw
	}
	n := binary.BigEndian.Uint64(raw[:prefixLen])
	// Plausibility bounds: the enclosed envelope is at least 16 bytes and at
	// most the whole body minus the prefix. A raw JSON envelope starts with
	// '{' (0x7b) whose uint64 interpretation is ~8.8e18 — far outside these
	// bounds — so valid unpadded frames are never mis-stripped.
	if n < 16 || n > uint64(len(raw)-prefixLen) {
		return raw
	}
	return raw[prefixLen : prefixLen+int(n)]
}

// applyMalleableWrapping wraps a raw (non-HTTP) beacon response body with the
// configured malleable prepend/append bytes so raw TCP/SMB links get the same
// cover as the HTTP transport. Headers/status are intentionally omitted because
// a raw socket has no HTTP semantics; the agent strips the identical bytes on
// read via stripMalleableWrapping. The operation is a no-op when malleable is
// disabled or no prepend/append is configured, preserving backward-compatible
// framing for links that do not use a profile.
func (s *Server) applyMalleableWrapping(body []byte) []byte {
	s.configMu.RLock()
	mp := s.cfg.Malleable
	s.configMu.RUnlock()
	if !mp.Enabled {
		return body
	}
	wrapped := string(body)
	if mp.Prepend != "" {
		wrapped = mp.Prepend + wrapped
	}
	if mp.Append != "" {
		wrapped = wrapped + mp.Append
	}
	return []byte(wrapped)
}

// applyV2FileProfile loads a v2/v1 profile file by name and applies its
// ServerOutput chain. Returns (body, contentType, headers, ok).
func (s *Server) applyV2FileProfile(name string, body []byte) ([]byte, string, map[string]string, bool) {
	dir := s.profileDataDir()
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, strings.TrimSpace(name))
	if sanitized == "" || sanitized == "default" {
		return nil, "", nil, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, "profiles", sanitized+".json"))
	if err != nil {
		return nil, "", nil, false
	}
	v2, err := malleable.MigrateProfileJSON(raw, sanitized)
	if err != nil || v2 == nil {
		return nil, "", nil, false
	}
	out := body
	if len(v2.ServerOutput) > 0 {
		tb := &malleable.TransformBlock{}
		for _, st := range v2.ServerOutput {
			tb.Transforms = append(tb.Transforms, malleable.Transform{Type: st.Type, Value: st.Value})
		}
		if enc, err := tb.Apply(body, true); err == nil {
			out = enc
		}
	} else {
		if v2.Prepend != "" {
			out = append([]byte(v2.Prepend), out...)
		}
		if v2.Append != "" {
			out = append(out, []byte(v2.Append)...)
		}
	}
	headers := map[string]string{}
	for k, v := range v2.Headers {
		headers[k] = v
	}
	ct := headers["Content-Type"]
	if ct == "" {
		ct = "text/plain"
	}
	return out, ct, headers, true
}

// v2RespDecodeWire returns the agent wire form of a file profile's ServerOutput.
func (s *Server) v2RespDecodeWire(name string) string {
	dir := s.profileDataDir()
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, strings.TrimSpace(name))
	if sanitized == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(dir, "profiles", sanitized+".json"))
	if err != nil {
		return ""
	}
	v2, err := malleable.MigrateProfileJSON(raw, sanitized)
	if err != nil || v2 == nil {
		return ""
	}
	return malleable.StepsToWire(v2.ServerOutput)
}

func (s *Server) applyProfilePreset(c *gin.Context, body []byte, profile *malleable.Profile) {
	// Apply output transforms
	if profile.HttpPost.Output != nil {
		transformed, err := profile.HttpPost.Output.Apply(body, true)
		if err == nil {
			body = transformed
		}
	}

	wrapped := string(body)

	for k, v := range profile.HttpPost.Headers {
		c.Header(k, v)
	}

	ct := profile.HttpPost.Headers["Content-Type"]
	if ct == "" {
		ct = "text/plain"
	}
	c.Header("Content-Type", ct)

	c.Status(http.StatusOK)
	c.Writer.WriteString(wrapped)
}
