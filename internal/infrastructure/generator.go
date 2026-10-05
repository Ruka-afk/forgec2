package infrastructure

import (
	"fmt"
	"strings"
)

type RedirectorConfig struct {
	Type        string // "nginx", "apache", "haproxy"
	Domain      string
	ListenPort  int
	BackendURL  string // e.g. "http://127.0.0.1:8080"
	CertPath    string
	KeyPath     string
	ExtC2Paths  []string // additional External C2 paths to proxy
	WSEnabled   bool
	BlockedIPs  []string
	UserAgent   string
	Profile     string
	ExtraConfig map[string]string
}

type ACMEConfig struct {
	Domain     string
	Email      string
	DataDir    string
	UseStaging bool
	Port       int
}

func (rc *RedirectorConfig) fillDefaults() {
	if rc.ListenPort == 0 {
		rc.ListenPort = 443
	}
	if rc.CertPath == "" {
		rc.CertPath = "/etc/letsencrypt/live/" + rc.Domain + "/fullchain.pem"
	}
	if rc.KeyPath == "" {
		rc.KeyPath = "/etc/letsencrypt/live/" + rc.Domain + "/privkey.pem"
	}
	if rc.ExtC2Paths == nil {
		rc.ExtC2Paths = []string{}
	}
	if rc.UserAgent == "" {
		rc.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}
}

// backendProxyScheme derives the proxy_pass scheme from the backend URL.
// Pointing nginx at a plain-HTTP backend over TLS makes every proxied
// request fail the handshake (502), and nginx rejects an upstream block
// that is used with mixed protocols — so the scheme must match.
func backendProxyScheme(rc RedirectorConfig) string {
	if strings.Contains(rc.BackendURL, "https://") {
		return "https"
	}
	return "http"
}

func GenerateNginxConfig(rc RedirectorConfig) string {
	rc.fillDefaults()
	var b strings.Builder
	scheme := backendProxyScheme(rc)

	fmt.Fprintf(&b, "# ForgeC2 Nginx Redirector Configuration\n")
	fmt.Fprintf(&b, "# Generated for domain: %s\n", rc.Domain)
	fmt.Fprintf(&b, "# Backend: %s\n\n", rc.BackendURL)

	// Upstream
	upstreamName := "forgec2_backend"
	fmt.Fprintf(&b, "upstream %s {\n", upstreamName)
	b.WriteString("    zone upstream_forgec2 64k;\n")
	fmt.Fprintf(&b, "    server %s;\n", stripScheme(rc.BackendURL))
	b.WriteString("    keepalive 32;\n")
	b.WriteString("}\n\n")

	// Rate limit zone
	b.WriteString("limit_req_zone $binary_remote_addr zone=forgec2_login:10m rate=5r/m;\n")
	b.WriteString("limit_req_zone $binary_remote_addr zone=forgec2_beacon:10m rate=100r/m;\n\n")

	// Redirect HTTP → HTTPS
	b.WriteString("server {\n")
	b.WriteString("    listen 80;\n")
	fmt.Fprintf(&b, "    server_name %s;\n", rc.Domain)
	b.WriteString("    return 301 https://$server_name$request_uri;\n")
	b.WriteString("}\n\n")

	// HTTPS server
	b.WriteString("server {\n")
	fmt.Fprintf(&b, "    listen %d ssl http2;\n", rc.ListenPort)
	fmt.Fprintf(&b, "    server_name %s;\n\n", rc.Domain)

	b.WriteString("    # SSL\n")
	fmt.Fprintf(&b, "    ssl_certificate %s;\n", rc.CertPath)
	fmt.Fprintf(&b, "    ssl_certificate_key %s;\n", rc.KeyPath)
	b.WriteString("    ssl_protocols TLSv1.2 TLSv1.3;\n")
	b.WriteString("    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;\n")
	b.WriteString("    ssl_prefer_server_ciphers on;\n")
	b.WriteString("    ssl_session_cache shared:SSL:10m;\n")
	b.WriteString("    ssl_session_timeout 10m;\n\n")

	b.WriteString("    # Security headers\n")
	b.WriteString("    add_header Strict-Transport-Security \"max-age=63072000; includeSubDomains; preload\" always;\n")
	b.WriteString("    add_header X-Content-Type-Options nosniff;\n")
	b.WriteString("    add_header X-Frame-Options SAMEORIGIN;\n")
	b.WriteString("    add_header Referrer-Policy \"same-origin\";\n\n")

	// Blocked IPs
	if len(rc.BlockedIPs) > 0 {
		b.WriteString("    # Blocked IPs\n")
		for _, ip := range rc.BlockedIPs {
			fmt.Fprintf(&b, "    deny %s;\n", ip)
		}
		b.WriteString("\n")
	}

	// Logging (minimal to avoid OPSEC)
	b.WriteString("    # Minimal access logging (OPSEC)\n")
	fmt.Fprintf(&b, "    access_log /var/log/nginx/%s-access.log combined buffer=32k flush=5m;\n", rc.Domain)
	b.WriteString("    error_log /var/log/nginx/forgec2-error.log warn;\n\n")

	// Beacon proxy
	b.WriteString("    # C2 Beacon endpoints\n")
	b.WriteString("    location /api/v1/ {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	b.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	b.WriteString("        proxy_buffering off;\n")
	b.WriteString("        proxy_cache off;\n")
	if scheme == "https" {
		// Plain-HTTP backends have no TLS layer to skip verification of.
		b.WriteString("        proxy_ssl_verify off;\n")
	}
	b.WriteString("        limit_req zone=forgec2_beacon burst=20 nodelay;\n")
	b.WriteString("    }\n\n")

	// External C2 paths
	for _, path := range rc.ExtC2Paths {
		loc := path
		if !strings.HasPrefix(loc, "/") {
			loc = "/" + loc
		}
		fmt.Fprintf(&b, "    location %s {\n", loc)
		fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
		b.WriteString("        proxy_http_version 1.1;\n")
		b.WriteString("        proxy_set_header Host $host;\n")
		b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		b.WriteString("        proxy_buffering off;\n")
		b.WriteString("        proxy_cache off;\n")
		b.WriteString("    }\n\n")
	}

	// Stage serving
	b.WriteString("    # Stager payload download\n")
	b.WriteString("    location /stage/ {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        proxy_buffering off;\n")
	b.WriteString("        proxy_cache off;\n")
	b.WriteString("    }\n\n")

	// Payload hosting
	b.WriteString("    # Hosted payloads\n")
	b.WriteString("    location /payloads/ {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        proxy_buffering off;\n")
	b.WriteString("        proxy_cache off;\n")
	b.WriteString("    }\n\n")

	// WebSocket support
	if rc.WSEnabled {
		b.WriteString("    # WebSocket for beacon\n")
		b.WriteString("    location /ws {\n")
		fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
		b.WriteString("        proxy_http_version 1.1;\n")
		b.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		b.WriteString("        proxy_set_header Connection \"upgrade\";\n")
		b.WriteString("        proxy_set_header Host $host;\n")
		b.WriteString("        proxy_read_timeout 86400s;\n")
		b.WriteString("        proxy_send_timeout 86400s;\n")
		b.WriteString("    }\n\n")
	}

	// ICMP/health endpoints
	b.WriteString("    # Health check & misc\n")
	b.WriteString("    location /health {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("    }")
	b.WriteString("\n    location /ready {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("    }\n\n")

	// Login with rate limiting
	b.WriteString("    # Login (rate limited)\n")
	b.WriteString("    location /login {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        limit_req zone=forgec2_login burst=3 nodelay;\n")
	b.WriteString("    }\n\n")

	// Static assets (serve from redirector to reduce backend load)
	b.WriteString("    # Static assets (cached at redirector)\n")
	b.WriteString("    location /static/ {\n")
	fmt.Fprintf(&b, "        proxy_pass %s://%s;\n", scheme, upstreamName)
	b.WriteString("        proxy_http_version 1.1;\n")
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        expires 7d;\n")
	b.WriteString("        add_header Cache-Control \"public, immutable\";\n")
	b.WriteString("    }\n\n")

	// Deny all other requests
	b.WriteString("    # Deny everything else\n")
	b.WriteString("    location / {\n")
	b.WriteString("        return 404;\n")
	b.WriteString("    }\n")

	b.WriteString("}\n")
	return b.String()
}

func GenerateApacheConfig(rc RedirectorConfig) string {
	rc.fillDefaults()
	var b strings.Builder

	fmt.Fprintf(&b, "# ForgeC2 Apache Redirector Configuration\n")
	fmt.Fprintf(&b, "# Generated for domain: %s\n", rc.Domain)
	fmt.Fprintf(&b, "# Backend: %s\n\n", rc.BackendURL)

	b.WriteString("<IfModule mod_ssl.c>\n")
	b.WriteString("<IfModule mod_proxy.c>\n\n")

	// HTTP → HTTPS redirect
	b.WriteString("<VirtualHost *:80>\n")
	fmt.Fprintf(&b, "    ServerName %s\n", rc.Domain)
	fmt.Fprintf(&b, "    Redirect permanent / https://%s/\n", rc.Domain)
	b.WriteString("</VirtualHost>\n\n")

	// HTTPS VirtualHost
	fmt.Fprintf(&b, "<VirtualHost *:%d>\n", rc.ListenPort)
	fmt.Fprintf(&b, "    ServerName %s\n", rc.Domain)
	fmt.Fprintf(&b, "    ServerAdmin admin@%s\n\n", rc.Domain)

	b.WriteString("    # SSL\n")
	b.WriteString("    SSLEngine on\n")
	fmt.Fprintf(&b, "    SSLCertificateFile %s\n", rc.CertPath)
	fmt.Fprintf(&b, "    SSLCertificateKeyFile %s\n", rc.KeyPath)
	b.WriteString("    SSLProtocol all -SSLv3 -TLSv1 -TLSv1.1\n")
	b.WriteString("    SSLCipherSuite ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256\n")
	b.WriteString("    SSLHonorCipherOrder on\n")
	b.WriteString("    SSLCompression off\n\n")

	b.WriteString("    # Security headers\n")
	b.WriteString("    Header always set Strict-Transport-Security \"max-age=63072000; includeSubDomains; preload\"\n")
	b.WriteString("    Header always set X-Content-Type-Options nosniff\n")
	b.WriteString("    Header always set X-Frame-Options SAMEORIGIN\n")
	b.WriteString("    Header always set Referrer-Policy \"same-origin\"\n\n")

	// Proxy settings
	b.WriteString("    # Proxy configuration\n")
	b.WriteString("    ProxyPreserveHost On\n")
	fmt.Fprintf(&b, "    ProxyPass /api/v1/ %s/api/v1/ timeout=300 keepalive=On\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPassReverse /api/v1/ %s/api/v1/\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPass /stage/ %s/stage/ timeout=300 keepalive=On\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPassReverse /stage/ %s/stage/\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPass /payloads/ %s/payloads/ timeout=300 keepalive=On\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPassReverse /payloads/ %s/payloads/\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPass /health %s/health timeout=10\n", rc.BackendURL)
	fmt.Fprintf(&b, "    ProxyPassReverse /health %s/health\n", rc.BackendURL)

	// External C2
	for _, path := range rc.ExtC2Paths {
		loc := path
		if !strings.HasPrefix(loc, "/") {
			loc = "/" + loc
		}
		fmt.Fprintf(&b, "    ProxyPass %s %s timeout=300 keepalive=On\n", loc, rc.BackendURL)
		fmt.Fprintf(&b, "    ProxyPassReverse %s %s\n", loc, rc.BackendURL)
	}

	// WebSocket
	if rc.WSEnabled {
		b.WriteString("\n    # WebSocket proxy\n")
		fmt.Fprintf(&b, "    ProxyPass /ws ws://%s/ws timeout=86400\n", stripScheme(rc.BackendURL))
	}

	// Blocked IPs
	for _, ip := range rc.BlockedIPs {
		fmt.Fprintf(&b, "    Require not ip %s\n", ip)
	}

	b.WriteString("\n    # Logging\n")
	fmt.Fprintf(&b, "    CustomLog ${APACHE_LOG_DIR}/%s-access.log combined\n", rc.Domain)
	b.WriteString("    ErrorLog ${APACHE_LOG_DIR}/forgec2-error.log\n\n")

	b.WriteString("</VirtualHost>\n")
	b.WriteString("</IfModule>\n")
	b.WriteString("</IfModule>\n")

	return b.String()
}

func GenerateHAProxyConfig(rc RedirectorConfig) string {
	rc.fillDefaults()
	var b strings.Builder

	b.WriteString("# ForgeC2 HAProxy Redirector Configuration\n")
	fmt.Fprintf(&b, "# Generated for domain: %s\n", rc.Domain)
	fmt.Fprintf(&b, "# Backend: %s\n\n", rc.BackendURL)

	// Global
	b.WriteString("global\n")
	b.WriteString("    daemon\n")
	b.WriteString("    maxconn 4096\n")
	b.WriteString("    ssl-default-bind-ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256\n")
	b.WriteString("    ssl-default-bind-options no-sslv3 no-tlsv10 no-tlsv11\n\n")

	// Defaults
	b.WriteString("defaults\n")
	b.WriteString("    mode http\n")
	b.WriteString("    log global\n")
	b.WriteString("    option httplog\n")
	b.WriteString("    option dontlognull\n")
	b.WriteString("    timeout connect 5s\n")
	b.WriteString("    timeout client 30s\n")
	b.WriteString("    timeout server 30s\n")
	b.WriteString("    timeout tunnel 86400s  # WebSocket long-poll\n\n")

	// Frontend HTTP → HTTPS redirect
	b.WriteString("frontend http-in\n")
	b.WriteString("    bind *:80\n")
	b.WriteString("    redirect scheme https code 301 if !{ ssl_fc }\n\n")

	// Frontend HTTPS
	b.WriteString("frontend https-in\n")
	fmt.Fprintf(&b, "    bind *:%d ssl crt /etc/ssl/%s.pem\n", rc.ListenPort, rc.Domain)
	b.WriteString("    default_backend forgec2_backend\n\n")

	// ACLs for specific paths
	b.WriteString("    # C2 Beacon API\n")
	b.WriteString("    acl is_beacon path_beg /api/v1/\n")
	b.WriteString("    acl is_stage path_beg /stage/\n")
	b.WriteString("    acl is_payload path_beg /payloads/\n")
	b.WriteString("    acl is_health path_beg /health\n")
	b.WriteString("    acl is_ready path_beg /ready\n")
	b.WriteString("    acl is_login path_beg /login\n")
	b.WriteString("    acl is_static path_beg /static/\n")
	if rc.WSEnabled {
		b.WriteString("    acl is_ws path_beg /ws\n")
	}

	// External C2 ACLs (one uniquely-named ACL per path: duplicate ACL names are a config error)
	for i, path := range rc.ExtC2Paths {
		loc := path
		if !strings.HasPrefix(loc, "/") {
			loc = "/" + loc
		}
		fmt.Fprintf(&b, "    acl is_extc2_%d path_beg %s\n", i, loc)
	}

	// Rate limiting via stick-table
	b.WriteString("\n    # Rate limiting (beacon)\n")
	b.WriteString("    stick-table type ip size 100k expire 30s store http_req_rate(10s)\n")
	b.WriteString("    http-request track-sc0 src\n")
	b.WriteString("    http-request reject if { sc_http_req_rate(0) gt 100 } !is_health !is_ready\n\n")

	b.WriteString("    # Login rate limiting\n")
	b.WriteString("    http-request reject if is_login { sc_http_req_rate(0) gt 5 }\n\n")

	// Blocked IPs
	for _, ip := range rc.BlockedIPs {
		fmt.Fprintf(&b, "    http-request deny if { src %s }\n", ip)
	}

	b.WriteString("\n    # Logging (OPSEC - minimal)\n")
	b.WriteString("    option httplog\n\n")

	b.WriteString("    # Access control: only allow specific paths to backend\n")
	// HAProxy ANDs multiple criteria in one "if" clause, so each ACL must get
	// its own use_backend line — a request can only match one path_beg ACL.
	for _, acl := range []string{"is_beacon", "is_stage", "is_payload", "is_health", "is_ready", "is_login", "is_static"} {
		fmt.Fprintf(&b, "    use_backend forgec2_backend if %s\n", acl)
	}
	if rc.WSEnabled {
		b.WriteString("    use_backend forgec2_backend if is_ws\n")
	}
	for i := range rc.ExtC2Paths {
		fmt.Fprintf(&b, "    use_backend forgec2_backend if is_extc2_%d\n", i)
	}
	b.WriteString("    default_backend forgec2_deny\n\n")

	// Backend
	upstream := stripScheme(rc.BackendURL)
	b.WriteString("backend forgec2_backend\n")
	// No "resolvers" directive: the upstream is a static host:port and
	// referencing an undefined resolvers section makes HAProxy refuse to start.
	fmt.Fprintf(&b, "    server forgec1 %s check\n", upstream)
	if rc.WSEnabled {
		b.WriteString("    # WebSocket support\n")
		b.WriteString("    timeout tunnel 86400s\n")
	}

	// Deny backend
	b.WriteString("\nbackend forgec2_deny\n")
	b.WriteString("    http-request deny deny_status 404\n")

	return b.String()
}

func stripScheme(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return url
}
