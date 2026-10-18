import { useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { downloadText } from "@/lib/download";
import { DEFAULT_LISTENER_ADDR } from "@/lib/constants";
import { useMutation } from "@/lib/hooks/useMutation";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import type { InfraListener } from "./useInfrastructureData";
export function useInfrastructureConfigForm(listeners: InfraListener[]) {
  const { t } = useI18n();
  const { toastError } = useErrorToast();
  const [selectedListener, setSelectedListener] = useState("");
  const [domain, setDomain] = useState("");
  const [port, setPort] = useState(443);
  const [certPath, setCertPath] = useState("");
  const [keyPath, setKeyPath] = useState("");
  const [wsSupport, setWsSupport] = useState(true);
  const [extC2Path, setExtC2Path] = useState("");
  const [configOutput, setConfigOutput] = useState("");
  const [configType, setConfigType] = useState("");
  const [copied, setCopied] = useState(false);
  const [activeSection, setActiveSection] = useState<"config" | "acme" | "export" | "redirectors">("config");
  const [acmeDomain, setAcmeDomain] = useState("");
  const [acmeEmail, setAcmeEmail] = useState("");
  const [acmePort, setAcmePort] = useState(80);
  const [acmeStaging, setAcmeStaging] = useState(false);
  const [exportFormat, setExportFormat] = useState("json");

  // Local template shown when /generate fails. It used to appear silently,
  // wearing the same "generated config" chrome as a server response; the
  // mutation reports the failure alongside it now.
  const buildFallbackConfig = useCallback((type: string): string => {
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
        proxy_pass https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : DEFAULT_LISTENER_ADDR};
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
    ProxyPass / https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : DEFAULT_LISTENER_ADDR}/
    ProxyPassReverse / https://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : DEFAULT_LISTENER_ADDR}/
${wsSupport ? `
    RewriteEngine On
    RewriteCond %{HTTP:Upgrade} websocket [NC]
   RewriteRule /(*) ws://${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : DEFAULT_LISTENER_ADDR}/$1 [P,L]` : ""}
</VirtualHost>`,
      haproxy: `# HAProxy Redirector Configuration
# Generated: ${new Date().toISOString()}

frontend c2_frontend
    bind *:${port} ssl crt ${certPath || "/etc/letsencrypt/live/" + (domain || "c2.example.com") + "/fullchain.pem"}
    mode http
    default_backend c2_backend

backend c2_backend
    mode http
    server c2server ${selectedListener ? (listeners.find(l => l.id === selectedListener)?.host || "127.0.0.1") + ":" + (listeners.find(l => l.id === selectedListener)?.port || "443") : DEFAULT_LISTENER_ADDR} ssl verify none${wsSupport ? " alpn h2,http/1.1" : ""}`,
    };
    return defaultConfigs[type] || "# Error generating config";
  }, [domain, port, wsSupport, certPath, keyPath, selectedListener, listeners]);

  const generateMutation = useMutation({
    fn: (type: string) =>
      api.postJson<{ config?: string }>(`/infrastructure/generate/${type}`, {
        domain, listen_port: port, ws_enabled: !!wsSupport,
        cert_path: certPath, key_path: keyPath, extc2_paths: extC2Path ? [extC2Path] : [],
      }),
    onSuccess: (data) => setConfigOutput((data.config || "") as string),
    onError: (e, type) => {
      setConfigOutput(buildFallbackConfig(type));
      toastError(e, "infrastructure.toast.generate_config_failed");
    },
  });

  const generateConfig = (type: string) => {
    setConfigType(type);
    void generateMutation.mutate(type);
  };

  const copyConfig = useCallback(async () => {
    // Local clipboard capability failure, not an API error: stays a direct
    // translated toast (the normalizer has no DOMException mapping).
    try {
      await navigator.clipboard.writeText(configOutput);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch { toast.error(t("infrastructure.toast.copy_config_failed")); }
  }, [configOutput, t]);

  const downloadConfig = useCallback(() => {
    const ext = configType === "nginx" ? "conf" : configType === "apache" ? "conf" : "cfg";
    downloadText(configOutput, `redirector-${configType}-${domain || "c2"}.${ext}`);
  }, [configOutput, configType, domain]);

  const provisionMutation = useMutation({
    fn: () => api.postJson(paths.infrastructure.acmeProvision, { domain: acmeDomain, email: acmeEmail, port: acmePort, staging: acmeStaging }),
    onSuccess: () => {
      setCertPath(`/etc/letsencrypt/live/${acmeDomain}/fullchain.pem`);
      setKeyPath(`/etc/letsencrypt/live/${acmeDomain}/privkey.pem`);
    },
    onError: (e) => toastError(e, "infrastructure.toast.provision_cert_failed"),
  });

  const exportMutation = useMutation({
    fn: async () => {
      const data = await api.get(paths.infrastructure.profileExport(exportFormat));
      const content = exportFormat === "json" ? JSON.stringify(data, null, 2) : JSON.stringify(data);
      const ext = exportFormat === "json" ? "json" : exportFormat === "nginx" ? "conf" : "env";
      downloadText(content, `c2-profile-${exportFormat}.${ext}`, "application/json");
    },
    onError: (e) => toastError(e, "infrastructure.toast.export_profile_failed"),
  });

  return {
    selectedListener, setSelectedListener,
    domain, setDomain,
    port, setPort,
    certPath, setCertPath,
    keyPath, setKeyPath,
    wsSupport, setWsSupport,
    extC2Path, setExtC2Path,
    configOutput, setConfigOutput,
    configType, setConfigType,
    generating: generateMutation.isPending,
    copied, setCopied,
    activeSection, setActiveSection,
    acmeDomain, setAcmeDomain,
    acmeEmail, setAcmeEmail,
    acmePort, setAcmePort,
    acmeStaging, setAcmeStaging,
    acmeProvisioning: provisionMutation.isPending,
    exportFormat, setExportFormat,
    exporting: exportMutation.isPending,
    generateConfig, copyConfig, downloadConfig,
    provisionCert: () => { void provisionMutation.mutate(); },
    exportProfile: () => { void exportMutation.mutate(); },
  };
}
