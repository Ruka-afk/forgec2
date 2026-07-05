"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";

interface InfraListener {
  id: string;
  name: string;
  host: string;
  port: string;
  protocol: string;
}


export default function InfrastructurePage() {
  const [listeners, setListeners] = useState<InfraListener[]>([]);
  const [selectedListener, setSelectedListener] = useState("");
  const [domain, setDomain] = useState("");
  const [port, setPort] = useState(443);
  const [certPath, setCertPath] = useState("");
  const [keyPath, setKeyPath] = useState("");
  const [wsSupport, setWsSupport] = useState(true);
  const [extC2Path, setExtC2Path] = useState(false);
  const [configOutput, setConfigOutput] = useState("");
  const [configType, setConfigType] = useState("");
  const [generating, setGenerating] = useState(false);
  const [copied, setCopied] = useState(false);
  const [activeSection, setActiveSection] = useState<"config" | "acme" | "export">("config");
  const [acmeDomain, setAcmeDomain] = useState("");
  const [acmeEmail, setAcmeEmail] = useState("");
  const [acmePort, setAcmePort] = useState(80);
  const [acmeStaging, setAcmeStaging] = useState(false);
  const [acmeProvisoining, setAcmeProvisioning] = useState(false);
  const [exportFormat, setExportFormat] = useState("json");

  const loadListeners = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/api/listeners&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setListeners(data.Listeners || data.listeners || []);
    } catch {
      setListeners([]);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadListeners()); }, [loadListeners]);

  const generateConfig = async (type: string) => {
    setGenerating(true);
    setConfigType(type);
    try {
      const params = new URLSearchParams({
        format: "json", type, listener: selectedListener, domain,
        port: String(port), cert: certPath, key: keyPath,
        ws: String(wsSupport), extc2: String(extC2Path),
      });
      const res = await fetch(`${API_BASE}?p=/infrastructure/generate/${type}&${params.toString()}`, { credentials: "include" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setConfigOutput(data.config || "");
    } catch {
      const defaultConfigs: Record<string, string> = {
        nginx: `# Nginx Redirector Configuration
# Generated: ${new Date().toISOString()}
# Forwarding to: ${selectedListener ? listeners.find(l => l.id === selectedListener)?.name : "n/a"}

server {
    listen ${port} ssl http2;
    server_name ${domain || "c2.example.com"};

    ssl_certificate ${certPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/fullchain.pem"};
    ssl_certificate_key ${keyPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/privkey.pem"};

    location / {
        proxy_pass https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : "127.0.0.1:443"};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_ssl_server_name on;
        proxy_ssl_protocols TLSv1.2 TLSv1.3;
${wsSupport ? `        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";` : ""}
    }
}`,
        apache: `# Apache Redirector Configuration
# Generated: ${new Date().toISOString()}

<VirtualHost *:${port}>
    ServerName ${domain || "c2.example.com"}
    SSLEngine On
    SSLCertificateFile ${certPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/fullchain.pem"}
    SSLCertificateKeyFile ${keyPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/privkey.pem"}

    SSLProxyEngine On
    ProxyPreserveHost On
    ProxyPass / https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : "127.0.0.1:443"}/
    ProxyPassReverse / https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : "127.0.0.1:443"}/
${wsSupport ? `
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
   RewriteRule /(*) ws://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : "127.0.0.1:443"}/$1 [P,L]` : ""}
</VirtualHost>`,
        haproxy: `# HAProxy Redirector Configuration
# Generated: ${new Date().toISOString()}

frontend c2_frontend
    bind *:${port} ssl crt ${certPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/fullchain.pem"}
    mode http
    default_backend c2_backend

backend c2_backend
    mode http
    server c2server ${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : "127.0.0.1:443"} ssl verify none${wsSupport ? " alpn h2,http/1.1" : ""}`,
      };
      setConfigOutput(defaultConfigs[type] || "# Error generating config");
    }
    setGenerating(false);
  };

  const copyConfig = async () => {
    try {
      await navigator.clipboard.writeText(configOutput);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) { console.error("Infrastructure: copy config failed", e); }
  };

  const downloadConfig = () => {
    const ext = configType === "nginx" ? "conf" : configType === "apache" ? "conf" : "cfg";
    const blob = new Blob([configOutput], { type: "text/plain" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `redirector-${configType}-${domain || "c2"}.${ext}`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const provisionCert = async () => {
    setAcmeProvisioning(true);
    try {
      await fetch(`${API_BASE}?p=/infrastructure/acme/provision`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ domain: acmeDomain, email: acmeEmail, port: acmePort, staging: acmeStaging }),
      });
      setCertPath(`/etc/letsencrypt/live/${acmeDomain}/fullchain.pem`);
      setKeyPath(`/etc/letsencrypt/live/${acmeDomain}/privkey.pem`);
    } catch (e) { console.error("Infrastructure: provision cert failed", e); }
    setAcmeProvisioning(false);
  };

  const exportProfile = async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/infrastructure/profile/export?format=${exportFormat}&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      const content = exportFormat === "json" ? JSON.stringify(data, null, 2) : JSON.stringify(data);
      const ext = exportFormat === "json" ? "json" : exportFormat === "nginx" ? "conf" : "env";
      const blob = new Blob([content], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `c2-profile-${exportFormat}.${ext}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) { console.error("Infrastructure: export profile failed", e); }
  };

  const currentListener = listeners.find(l => l.id === selectedListener);

  const sections = [
    { key: "config" as const, label: "反代配置", icon: "fa-cog" },
    { key: "acme" as const, label: "ACME 证书", icon: "fa-certificate" },
    { key: "export" as const, label: "C2 Profile", icon: "fa-file-export" },
  ];

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      <div className="bg-gradient-to-r from-indigo-600 to-indigo-800 rounded-3xl shadow-sm p-6 text-white">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 bg-white/10 rounded-2xl flex items-center justify-center">
            <i className="fa-solid fa-cloud-upload-alt text-2xl"></i>
          </div>
          <div>
            <h1 className="text-xl font-bold">Infrastructure Automation</h1>
            <p className="text-sm text-indigo-200 mt-0.5">Generate redirector config, auto TLS certificates, export C2 profiles</p>
          </div>
        </div>
      </div>

      <div className="flex border-b border-[var(--border)]">
        {sections.map(s => (
          <button key={s.key} onClick={() => setActiveSection(s.key)}
            className={`flex items-center gap-x-2 px-5 py-3 text-sm font-medium border-b-2 transition-colors ${activeSection === s.key ? "border-indigo-500 text-indigo-600 dark:text-indigo-400" : "border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300"}`}>
            <i className={`fa-solid ${s.icon}`}></i>
            <span>{s.label}</span>
          </button>
        ))}
      </div>

      {activeSection === "config" && (
        <>
          <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-700 flex items-center gap-3">
              <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center text-indigo-600 dark:text-indigo-400"><i className="fa-solid fa-plug"></i></div>
              <div><h2 className="text-sm font-semibold text-slate-800 dark:text-slate-100">选择 Listener</h2><p className="text-[11px] text-slate-500 dark:text-slate-400">Select a listener to point the redirector to</p></div>
            </div>
            <div className="p-6">
              <select value={selectedListener} onChange={e => setSelectedListener(e.target.value)}
                className="w-full max-w-md bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-indigo-400 dark:text-slate-100">
                <option value="">-- Select a listener --</option>
                {listeners.map(l => (
                  <option key={l.id} value={l.id}>{l.name} ({l.protocol}://{l.host}:{l.port})</option>
                ))}
              </select>
              {currentListener && (
                <div className="mt-3 text-xs text-slate-400 dark:text-slate-500">
                  <i className="fa-solid fa-circle-info mr-1"></i>
                  Forwarding to: {currentListener.protocol}://{currentListener.host}:{currentListener.port}
                </div>
              )}
            </div>
          </div>

          <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-700 flex items-center gap-3">
              <div className="w-8 h-8 bg-emerald-100 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center text-emerald-600 dark:text-emerald-400"><i className="fa-solid fa-globe"></i></div>
              <div><h2 className="text-sm font-semibold text-slate-800 dark:text-slate-100">域名与参数</h2><p className="text-[11px] text-slate-500 dark:text-slate-400">Configure frontend domain and redirector parameters</p></div>
            </div>
            <div className="p-6 space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">Domain</label>
                  <input type="text" value={domain} onChange={e => setDomain(e.target.value)} placeholder="c2.example.com"
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-indigo-400 dark:text-slate-100" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">Listen Port</label>
                  <input type="number" value={port} onChange={e => setPort(Number(e.target.value))}
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-indigo-400 dark:text-slate-100" />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">SSL Certificate Path</label>
                  <input type="text" value={certPath} onChange={e => setCertPath(e.target.value)} placeholder="/etc/letsencrypt/live/c2.example.com/fullchain.pem"
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-indigo-400 font-mono text-xs dark:text-slate-100" />
                </div>
                <div>
                  <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">SSL Key Path</label>
                  <input type="text" value={keyPath} onChange={e => setKeyPath(e.target.value)} placeholder="/etc/letsencrypt/live/c2.example.com/privkey.pem"
                    className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-indigo-400 font-mono text-xs dark:text-slate-100" />
                </div>
              </div>
              <div className="flex items-center gap-6">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={wsSupport} onChange={e => setWsSupport(e.target.checked)} className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500" />
                  <span className="text-xs text-[var(--text-secondary)]">WebSocket Support</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" checked={extC2Path} onChange={e => setExtC2Path(e.target.checked)} className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500" />
                  <span className="text-xs text-[var(--text-secondary)]">Include External C2 Path</span>
                </label>
              </div>
            </div>
          </div>

          <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
            <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-700 flex items-center gap-3">
              <div className="w-8 h-8 bg-amber-100 dark:bg-amber-900/30 rounded-xl flex items-center justify-center text-amber-600 dark:text-amber-400"><i className="fa-solid fa-file-export"></i></div>
              <div><h2 className="text-sm font-semibold text-slate-800 dark:text-slate-100">生成配置</h2><p className="text-[11px] text-slate-500 dark:text-slate-400">Select target redirector type and generate config file</p></div>
            </div>
            <div className="p-6">
              <div className="flex flex-wrap gap-2 mb-4">
                <button onClick={() => generateConfig("nginx")} className="px-5 py-2.5 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium flex items-center gap-2 transition-colors">
                  <i className="fa-solid fa-server"></i> Nginx
                </button>
                <button onClick={() => generateConfig("apache")} className="px-5 py-2.5 bg-orange-600 hover:bg-orange-700 text-white rounded-xl text-sm font-medium flex items-center gap-2 transition-colors">
                  <i className="fa-solid fa-server"></i> Apache
                </button>
                <button onClick={() => generateConfig("haproxy")} className="px-5 py-2.5 bg-red-600 hover:bg-red-700 text-white rounded-xl text-sm font-medium flex items-center gap-2 transition-colors">
                  <i className="fa-solid fa-server"></i> HAProxy
                </button>
              </div>
              {generating && <div className="text-slate-500 dark:text-slate-400 text-sm"><i className="fa-solid fa-circle-notch fa-spin mr-2"></i>Generating...</div>}
              {configOutput && !generating && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <label className="text-xs font-medium text-[var(--text-secondary)]">Generated Config ({configType.toUpperCase()})</label>
                    <div className="flex gap-2">
                      <button onClick={copyConfig} className="text-xs px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-indigo-100 dark:hover:bg-indigo-900/30 rounded-lg flex items-center gap-1.5 text-slate-600 dark:text-slate-300 transition-colors">
                        <i className="fa-regular fa-copy"></i> {copied ? "Copied!" : "Copy"}
                      </button>
                      <button onClick={downloadConfig} className="text-xs px-3 py-1.5 bg-slate-100 dark:bg-slate-700 hover:bg-emerald-100 dark:hover:bg-emerald-900/30 rounded-lg flex items-center gap-1.5 text-slate-600 dark:text-slate-300 transition-colors">
                        <i className="fa-solid fa-download"></i> Download
                      </button>
                    </div>
                  </div>
                  <pre className="bg-slate-900 text-emerald-400 p-4 rounded-2xl text-xs overflow-auto max-h-[400px] whitespace-pre-wrap font-mono leading-relaxed select-all border border-slate-700">{configOutput}</pre>
                </div>
              )}
            </div>
          </div>
        </>
      )}

      {activeSection === "acme" && (
        <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-700 flex items-center gap-3">
            <div className="w-8 h-8 bg-cyan-100 dark:bg-cyan-900/30 rounded-xl flex items-center justify-center text-cyan-600 dark:text-cyan-400"><i className="fa-solid fa-certificate"></i></div>
            <div><h2 className="text-sm font-semibold text-slate-800 dark:text-slate-100">ACME TLS Auto-Renewal</h2><p className="text-[11px] text-slate-500 dark:text-slate-400">Auto-provision TLS certificates via Let&apos;s Encrypt</p></div>
          </div>
          <div className="p-6 space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1"><i className="fa-solid fa-globe mr-1"></i>域名</label>
                <input type="text" value={acmeDomain} onChange={e => setAcmeDomain(e.target.value)} placeholder="c2.example.com" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-cyan-400 dark:text-slate-100" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1"><i className="fa-solid fa-envelope mr-1"></i>Email</label>
                <input type="email" value={acmeEmail} onChange={e => setAcmeEmail(e.target.value)} placeholder="admin@example.com" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-cyan-400 dark:text-slate-100" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1"><i className="fa-solid fa-plug mr-1"></i>HTTP-01 Port</label>
                <input type="number" value={acmePort} onChange={e => setAcmePort(Number(e.target.value))} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-cyan-400 dark:text-slate-100" />
              </div>
            </div>
            <div className="flex items-center gap-4">
              <label className="flex items-center gap-2 cursor-pointer">
                <input type="checkbox" checked={acmeStaging} onChange={e => setAcmeStaging(e.target.checked)} className="rounded border-slate-300 text-cyan-600 focus:ring-cyan-500" />
                <span className="text-xs text-[var(--text-secondary)]">Use Staging (test) environment</span>
              </label>
              <button onClick={provisionCert} disabled={acmeProvisoining}
                className="px-5 py-2.5 bg-cyan-600 hover:bg-cyan-700 disabled:opacity-50 text-white rounded-xl text-sm font-medium flex items-center gap-2 transition-colors">
                <i className={`fa-solid ${acmeProvisoining ? "fa-circle-notch fa-spin" : "fa-certificate"}`}></i> {acmeProvisoining ? "Provisioning..." : "Auto-Provision Certificate"}
              </button>
            </div>
            {certPath && keyPath && (
              <div className="p-3 bg-emerald-50 dark:bg-emerald-900/20 rounded-xl text-xs text-emerald-700 dark:text-emerald-400">
                <i className="fa-solid fa-check-circle mr-1"></i>
                <b>Success:</b> Cert set to {certPath}
              </div>
            )}
          </div>
        </div>
      )}

      {activeSection === "export" && (
        <div className="ui-card rounded-3xl shadow-sm overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-700 flex items-center gap-3">
            <div className="w-8 h-8 bg-purple-100 dark:bg-purple-900/30 rounded-xl flex items-center justify-center text-purple-600 dark:text-purple-400"><i className="fa-solid fa-file-export"></i></div>
            <div><h2 className="text-sm font-semibold text-slate-800 dark:text-slate-100">C2 Profile Export</h2><p className="text-[11px] text-slate-500 dark:text-slate-400">Export current Malleable C2 profile configuration</p></div>
          </div>
          <div className="p-6 space-y-4">
            <div className="mb-4">
              <label className="block text-xs font-medium text-[var(--text-secondary)] mb-2">导出格式</label>
              <div className="flex gap-3">
                {([["json", "JSON", "fa-code", "purple"], ["nginx", "Nginx", "fa-server", "green"], ["env", "ENV", "fa-terminal", "slate"]] as const).map(([value, label, icon, color]) => (
                  <button key={value} onClick={() => setExportFormat(value)}
                    className={`px-4 py-2 rounded-xl text-sm font-medium flex items-center gap-2 transition-colors border-2 ${exportFormat === value ? `border-${color}-500 bg-${color}-50 dark:bg-${color}-900/20 text-${color}-600 dark:text-${color}-400` : "border-[var(--border)] text-slate-600 dark:text-slate-400 hover:border-slate-300 dark:hover:border-slate-500"}`}>
                    <i className={`fa-solid ${icon}`}></i>{label}
                  </button>
                ))}
              </div>
            </div>
            <button onClick={exportProfile} className="px-5 py-2.5 bg-purple-600 hover:bg-purple-700 text-white rounded-xl text-sm font-medium flex items-center gap-2 transition-colors">
              <i className="fa-solid fa-download"></i> Export as {exportFormat.toUpperCase()}
            </button>
            <div className="mt-4 p-3 bg-slate-50 dark:bg-slate-700/30 rounded-xl text-xs text-slate-500 dark:text-slate-400">
              <i className="fa-solid fa-info-circle mr-1"></i>
              <b>About:</b> Exports the current Malleable C2 profile used by the beacon for communication. The exported profile can be imported into compatible implants.
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
