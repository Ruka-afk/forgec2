"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { esc } from "@/lib/sanitize";
import {
  Listener, ProfilePreset, SharedState, EXEForm, PS1Form, LinuxForm, MacOSForm,
  StagerForm, ShellcodeForm, DonutForm, OneLinerForm, BusyState, Results,
} from "./_components/types";
import Toast from "./_components/Toast";
import SharedSettings from "./_components/SharedSettings";
import {
  EXEPanel, PS1Panel, LinuxPanel, MacOSPanel, StagerPanel, StagerLinuxPanel,
  ShellcodePanel, DonutPanel,
} from "./_components/BuildPanels";
import OneLinerPanel from "./_components/OneLinerPanel";

const defaultProfilePresets: ProfilePreset[] = [
  { name: "default", description: "Default", user_agent: "", sleep: 0, jitter: 0 },
  { name: "google", description: "Google", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36", sleep: 8, jitter: 15 },
  { name: "bing", description: "Bing", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Edg/120.0.0.0", sleep: 10, jitter: 20 },
  { name: "amazon", description: "Amazon - AWS CDN", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0", sleep: 15, jitter: 25 },
  { name: "cloudflare", description: "Cloudflare", user_agent: "Mozilla/5.0 (compatible; Cloudflare-Health-Checks/1.0; +https://www.cloudflare.com/)", sleep: 30, jitter: 10 },
  { name: "github", description: "GitHub", user_agent: "GitHub-Hookshot/abcd1234", sleep: 5, jitter: 5 },
  { name: "office365", description: "Office 365", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 OPR/106.0.0.0", sleep: 20, jitter: 15 },
  { name: "teams", description: "Microsoft Teams", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Teams/1.6.00.27573", sleep: 10, jitter: 20 },
  { name: "slack", description: "Slack", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0.0 Safari/537.36 Slack/4.36.0", sleep: 8, jitter: 15 },
  { name: "zoom", description: "Zoom", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Zoom/5.17.5", sleep: 6, jitter: 10 },
  { name: "dropbox", description: "Dropbox", user_agent: "DropboxDesktopClient/187.4.6204 (Windows; 10.0; Win64; x64)", sleep: 12, jitter: 20 },
  { name: "windows_update", description: "Windows Update", user_agent: "Windows-Update-Agent/10.0.19041.3636", sleep: 60, jitter: 10 },
  { name: "firefox_update", description: "Firefox", user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/121.0", sleep: 120, jitter: 15 },
  { name: "apple", description: "Apple", user_agent: "Mac OS X/10.15.7 (KHTML, like Gecko) Version/17.2 Safari/605.1.15", sleep: 30, jitter: 20 },
  { name: "adobe", description: "Adobe", user_agent: "Creative Cloud/6.4.0.361 (Windows; x64)", sleep: 45, jitter: 25 },
];

interface ToastMessage {
  id: number;
  text: string;
  type: "success" | "error" | "info";
}

export default function GeneratePage() {
  const [listeners, setListeners] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(true);
  const [profilePresets, setProfilePresets] = useState<ProfilePreset[]>(defaultProfilePresets);
  const [profileLocked, setProfileLocked] = useState(false);
  const savedManualSettingsRef = useRef<{ interval: string; jitter: string; ua: string } | null>(null);
  const prevProfileRef = useRef<string>("default");
  const [toasts, setToasts] = useState<ToastMessage[]>([]);
  const toastIdRef = useRef(0);

  const [showListenerModal, setShowListenerModal] = useState(false);
  const [listenerForm, setListenerForm] = useState({ name: "", ltype: "http", host: "", port: "8080", proto: "http" });

  const [shared, setShared] = useState<SharedState>({
    listener_id: "", c2_url: "", protocol: "http", interval: "5", jitter: "0",
    ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
    proxy: "", failover: "", crypto_key: "", profile: "",
  });

  const [exeForm, setExeForm] = useState<EXEForm>({
    filename: "forge_agent.exe", persist: false, skip_tls: false, evasion: false, obfuscate: false, domain_front: "",
    p2p_mode: "", p2p_parent: "", p2p_listen_addr: "", dns_domain: "", dns_server: "",
  });
  const [ps1Form, setPs1Form] = useState<PS1Form>({ persist: false, skip_tls: false });
  const [linuxForm, setLinuxForm] = useState<LinuxForm>({ filename: "forge_implant", persist: false, skip_tls: false, obfuscate: false, domain_front: "" });
  const [macosForm, setMacosForm] = useState<MacOSForm>({ filename: "forge_implant", persist: false, skip_tls: false, obfuscate: false, domain_front: "" });
  const [stagerForm, setStagerForm] = useState<StagerForm>({ filename: "stager.exe", skip_tls: false });
  const [stagerLinuxForm, setStagerLinuxForm] = useState<StagerForm>({ filename: "stager", skip_tls: false });
  const [shellcodeForm, setShellcodeForm] = useState<ShellcodeForm>({
    command: "powershell -NoP -EP Bypass -Enc ...", filename: "shellcode.bin",
  });
  const [donutForm, setDonutForm] = useState<DonutForm>({
    arch: "amd64", class: "", method: "", args: "", filename: "donut_loader.bin", assembly: null,
  });
  const [onelinerForm, setOnelinerForm] = useState<OneLinerForm>({
    payload_type: "exe", beacon_time: "5", jitter: "10", skip_tls: false, persist: false,
    listener_id: "", c2_url: "", protocol: "http",
  });

  const [results, setResults] = useState<Results>({
    exe: "", ps1: "", linux: "", macos: "", stager: "", stager_linux: "",
    shellcode: "", donut: "", oneliner: "",
  });

  const [busy, setBusy] = useState<BusyState>({
    exe: false, ps1: false, linux: false, macos: false, stager: false, stager_linux: false,
    shellcode: false, donut: false, oneliner: false,
  });

  const fileInputRef = useRef<HTMLInputElement>(null);
  const donutFileRef = useRef<HTMLInputElement>(null);

  const showToast = useCallback((text: string, type: "success" | "error" | "info") => {
    const id = ++toastIdRef.current;
    setToasts((prev) => [...prev, { id, text, type }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3000);
  }, []);

  const loadData = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/generate&format=json`);
      const data = await res.json();
      setListeners(data.listeners || data.Listeners || []);
    } catch {
      setListeners([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadData()); }, [loadData]);

  const getListenerInfo = (listenerId: string) => {
    const l = listeners.find((x) => (x.id || x.ID) === listenerId);
    if (!l) return null;
    return {
      scheme: l.scheme || l.Scheme || l.type || l.Type || "http",
      host: l.host || l.Host || "",
      port: l.port || l.Port || "",
      type: l.type || l.Type || "",
      name: l.name || l.Name || "Unknown",
    };
  };

  const getSharedSettings = useCallback(() => {
    const preset = profilePresets.find((p) => p.name === prevProfileRef.current);
    let interval = shared.interval;
    let jitter = shared.jitter;
    let ua = shared.ua;
    if (prevProfileRef.current !== "default" && prevProfileRef.current !== "" && preset) {
      interval = String(preset.sleep && preset.sleep > 0 ? preset.sleep : 10);
      jitter = String(preset.jitter && preset.jitter >= 0 ? preset.jitter : 20);
      ua = preset.user_agent || ua;
    }
    return { profile: prevProfileRef.current, interval, jitter, user_agent: ua, proxy: shared.proxy, crypto_key: shared.crypto_key };
  }, [shared, profilePresets]);

  const buildFormData = useCallback((formData: FormData, extra?: Record<string, string>) => {
    const s = getSharedSettings();
    formData.set("profile", s.profile);
    formData.set("interval", s.interval);
    formData.set("jitter", s.jitter);
    formData.set("user_agent", s.user_agent);
    formData.set("proxy", s.proxy);
    formData.set("crypto_key", s.crypto_key);
    formData.set("listener_id", shared.listener_id);
    if (extra) for (const [k, v] of Object.entries(extra)) formData.set(k, v);
  }, [getSharedSettings, shared.listener_id]);

  const buildC2URL = useCallback(() => {
    const info = getListenerInfo(shared.listener_id);
    if (!info) return "";
    let c2url = `${info.scheme}://${info.host}:${info.port}`;
    if (shared.failover.trim()) c2url += "," + shared.failover.trim();
    return c2url;
  }, [shared.listener_id, shared.failover]);

  const getProtocol = useCallback(() => {
    const info = getListenerInfo(shared.listener_id);
    if (!info) return "http";
    if (info.scheme === "tcp" || info.scheme === "tls") return "tcp";
    if (info.scheme === "dns" || info.type === "dns") return "dns";
    return "http";
  }, [shared.listener_id]);

  const handleGenerate = useCallback(async (endpoint: string, formName: string, opts?: { isJson?: boolean; extra?: Record<string, string> }) => {
    if (!shared.listener_id) {
      showToast("Please select a listener in shared settings first", "error");
      return null;
    }
    const c2url = buildC2URL();
    const protocol = getProtocol();
    const formData = new FormData();
    formData.set("c2_url", c2url);
    formData.set("protocol", protocol);
    buildFormData(formData, opts?.extra);
    const res = await fetch(`${API_BASE}?p=/generate${endpoint}`, { method: "POST", body: formData });
    if (!res.ok) return { error: await res.text() };
    if (opts?.isJson) return await res.json();
    const blob = await res.blob();
    const cd = res.headers.get("Content-Disposition");
    let fn = formName;
    if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g, ""); }
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a"); a.href = url; a.download = fn;
    document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
    return { success: true };
  }, [shared.listener_id, buildC2URL, getProtocol, buildFormData, showToast]);

  const handleGenerateEXE = async () => {
    setBusy((b) => ({ ...b, exe: true }));
    setResults((r) => ({ ...r, exe: "" }));
    const extra: Record<string, string> = {
      persist: exeForm.persist ? "true" : "", skip_tls_verify: exeForm.skip_tls ? "true" : "",
      evasion: exeForm.evasion ? "true" : "", obfuscate: exeForm.obfuscate ? "true" : "", filename: exeForm.filename,
      domain_front: exeForm.domain_front,
      p2p_mode: exeForm.p2p_mode, p2p_parent: exeForm.p2p_parent,
      p2p_listen_addr: exeForm.p2p_listen_addr, dns_domain: exeForm.dns_domain, dns_server: exeForm.dns_server,
    };
    const result = await handleGenerate("/exe", "forge_agent.exe", { extra });
    if (result?.error) setResults((r) => ({ ...r, exe: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("EXE generated and downloaded successfully!", "success");
    setBusy((b) => ({ ...b, exe: false }));
  };

  const handleGeneratePS1 = async () => {
    setBusy((b) => ({ ...b, ps1: true }));
    setResults((r) => ({ ...r, ps1: "" }));
    const extra: Record<string, string> = { persist: ps1Form.persist ? "true" : "", skip_tls_verify: ps1Form.skip_tls ? "true" : "" };
    const result = await handleGenerate("/ps1", "", { isJson: true, extra });
    if (result?.error) setResults((r) => ({ ...r, ps1: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) {
      showToast(`Generated! Original: ${result.original_length} B / Obfuscated: ${result.obfuscated_len} B`, "success");
      setResults((r) => ({ ...r, ps1: "success", _ps1Code: result.code, _ps1Original: result.original_length, _ps1Obfuscated: result.obfuscated_len }));
    }
    setBusy((b) => ({ ...b, ps1: false }));
  };

  const handleGenerateLinux = async () => {
    setBusy((b) => ({ ...b, linux: true }));
    setResults((r) => ({ ...r, linux: "" }));
    const extra: Record<string, string> = { persist: linuxForm.persist ? "true" : "", skip_tls_verify: linuxForm.skip_tls ? "true" : "", obfuscate: linuxForm.obfuscate ? "true" : "", filename: linuxForm.filename, domain_front: linuxForm.domain_front };
    const result = await handleGenerate("/linux", "forge_implant", { extra });
    if (result?.error) setResults((r) => ({ ...r, linux: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("Linux ELF generated and downloaded successfully!", "success");
    setBusy((b) => ({ ...b, linux: false }));
  };

  const handleGenerateMacOS = async () => {
    setBusy((b) => ({ ...b, macos: true }));
    setResults((r) => ({ ...r, macos: "" }));
    const extra: Record<string, string> = { persist: macosForm.persist ? "true" : "", skip_tls_verify: macosForm.skip_tls ? "true" : "", obfuscate: macosForm.obfuscate ? "true" : "", filename: macosForm.filename, domain_front: macosForm.domain_front };
    const result = await handleGenerate("/macos", "forge_implant", { extra });
    if (result?.error) setResults((r) => ({ ...r, macos: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("macOS Binary generated and downloaded successfully!", "success");
    setBusy((b) => ({ ...b, macos: false }));
  };

  const handleGenerateStager = async () => {
    setBusy((b) => ({ ...b, stager: true }));
    setResults((r) => ({ ...r, stager: "" }));
    const extra: Record<string, string> = { filename: stagerForm.filename, skip_tls_verify: stagerForm.skip_tls ? "true" : "" };
    const result = await handleGenerate("/stager", "stager.exe", { extra });
    if (result?.error) setResults((r) => ({ ...r, stager: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("Loader + staging payload generated successfully!", "success");
    setBusy((b) => ({ ...b, stager: false }));
  };

  const handleGenerateStagerLinux = async () => {
    setBusy((b) => ({ ...b, stager_linux: true }));
    setResults((r) => ({ ...r, stager_linux: "" }));
    const extra: Record<string, string> = { filename: stagerLinuxForm.filename, skip_tls_verify: stagerLinuxForm.skip_tls ? "true" : "" };
    const result = await handleGenerate("/stager_linux", "stager", { extra });
    if (result?.error) setResults((r) => ({ ...r, stager_linux: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("Linux Loader + staging payload generated successfully!", "success");
    setBusy((b) => ({ ...b, stager_linux: false }));
  };

  const handleGenerateShellcode = async () => {
    setBusy((b) => ({ ...b, shellcode: true }));
    setResults((r) => ({ ...r, shellcode: "" }));
    const extra: Record<string, string> = { command: shellcodeForm.command, filename: shellcodeForm.filename };
    const result = await handleGenerate("/shellcode", "shellcode.bin", { extra });
    if (result?.error) setResults((r) => ({ ...r, shellcode: `<div class="text-sm text-red-500">Error: ${esc(result.error)}</div>` }));
    else if (result?.success) showToast("Shellcode generated and downloaded successfully!", "success");
    setBusy((b) => ({ ...b, shellcode: false }));
  };

  const handleGenerateDonut = async () => {
    setBusy((b) => ({ ...b, donut: true }));
    setResults((r) => ({ ...r, donut: "" }));
    if (!donutForm.assembly) {
      showToast("Please select a .NET assembly file", "error");
      setBusy((b) => ({ ...b, donut: false }));
      return;
    }
    const c2url = buildC2URL();
    const protocol = getProtocol();
    const formData = new FormData();
    formData.set("c2_url", c2url); formData.set("protocol", protocol);
    buildFormData(formData);
    formData.set("arch", donutForm.arch); formData.set("class", donutForm.class);
    formData.set("method", donutForm.method); formData.set("args", donutForm.args);
    formData.set("filename", donutForm.filename); formData.set("assembly", donutForm.assembly);
    try {
      const res = await fetch(`${API_BASE}?p=/generate/donut`, { method: "POST", body: formData });
      if (res.ok) {
        const blob = await res.blob();
        const cd = res.headers.get("Content-Disposition");
        let fn = "donut_loader.bin";
        if (cd) { const m = cd.match(/filename=(.+)/); if (m) fn = m[1].replace(/"/g, ""); }
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a"); a.href = url; a.download = fn;
        document.body.appendChild(a); a.click(); window.URL.revokeObjectURL(url); a.remove();
        showToast("Donut Shellcode generated and downloaded successfully!", "success");
      } else {
        const text = await res.text();
        setResults((r) => ({ ...r, donut: `<div class="text-sm text-red-500">Error: ${esc(text)}</div>` }));
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setResults((r) => ({ ...r, donut: `<div class="text-sm text-red-500">Error: ${esc(msg)}</div>` }));
    }
    setBusy((b) => ({ ...b, donut: false }));
  };

  const handleGenerateOneLiner = async () => {
    setBusy((b) => ({ ...b, oneliner: true }));
    setResults((r) => ({ ...r, oneliner: "" }));
    if (!onelinerForm.listener_id) {
      showToast("Please select a listener first", "error");
      setBusy((b) => ({ ...b, oneliner: false }));
      return;
    }
    const info = getListenerInfo(onelinerForm.listener_id);
    let c2url = "";
    let protocol = "http";
    if (info) {
      c2url = `${info.scheme}://${info.host}:${info.port}`;
      if (info.scheme === "tcp" || info.scheme === "tls") protocol = "tcp";
      else if (info.scheme === "dns" || info.type === "dns") protocol = "dns";
    }
    const formData = new FormData();
    formData.set("c2_url", c2url); formData.set("protocol", protocol);
    formData.set("listener_id", onelinerForm.listener_id);
    formData.set("payload_type", onelinerForm.payload_type);
    formData.set("beacon_time", onelinerForm.beacon_time);
    formData.set("jitter", onelinerForm.jitter);
    formData.set("skip_tls_verify", onelinerForm.skip_tls ? "true" : "");
    formData.set("persist", onelinerForm.persist ? "true" : "");
    const s = getSharedSettings();
    formData.set("profile", s.profile); formData.set("interval", s.interval);
    formData.set("user_agent", s.user_agent); formData.set("proxy", s.proxy);
    try {
      const res = await fetch(`${API_BASE}?p=/generate/one-liner`, { method: "POST", body: formData });
      const data = await res.json();
      if (!data.success) {
        setResults((r) => ({ ...r, oneliner: `<div class="text-sm text-red-500">Error: ${esc(data.error || "")}</div>` }));
      } else {
        setResults((r) => ({ ...r, oneliner: "success", _onelinerData: data }));
        showToast(`Generated ${data.types.length} One-Liner commands`, "success");
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setResults((r) => ({ ...r, oneliner: `<div class="text-sm text-red-500">Request failed: ${msg}</div>` }));
    }
    setBusy((b) => ({ ...b, oneliner: false }));
  };

  const changeProfile = (profile: string) => {
    if (profile === "__import__") {
      if (fileInputRef.current) fileInputRef.current.click();
      return;
    }
    if (profile === "default" || profile === "") {
      setProfileLocked(false);
      if (savedManualSettingsRef.current) {
        setShared((s) => ({ ...s, interval: savedManualSettingsRef.current!.interval, jitter: savedManualSettingsRef.current!.jitter, ua: savedManualSettingsRef.current!.ua }));
      }
    } else {
      if (!profileLocked) savedManualSettingsRef.current = { interval: shared.interval, jitter: shared.jitter, ua: shared.ua };
      const preset = profilePresets.find((p) => p.name === profile);
      if (preset) setShared((s) => ({ ...s, interval: String(preset.sleep && preset.sleep > 0 ? preset.sleep : 10), jitter: String(preset.jitter && preset.jitter >= 0 ? preset.jitter : 20), ua: preset.user_agent || s.ua }));
      setProfileLocked(true);
    }
    prevProfileRef.current = profile;
    setShared((s) => ({ ...s, profile }));
  };

  const handleProfileImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.append("profile", file);
    try {
      const res = await fetch(`${API_BASE}?p=/api/generate/profile/import`, { method: "POST", body: fd });
      const data = await res.json();
      if (!data.success) { showToast(data.error || "Import failed", "error"); return; }
      const preset: ProfilePreset = { name: data.profile.name, description: data.profile.description, user_agent: data.profile.user_agent, sleep: data.profile.sleep, jitter: data.profile.jitter };
      setProfilePresets((prev) => { const idx = prev.findIndex((p) => p.name === preset.name); if (idx >= 0) { const next = [...prev]; next[idx] = preset; return next; } return [...prev, preset]; });
      showToast("Profile imported", "success");
    } catch (err: unknown) { showToast(err instanceof Error ? err.message : String(err), "error"); }
    finally { e.target.value = ""; }
  };

  const handleCreateListener = () => {
    setListenerForm({ name: "My DNS Listener", ltype: "http", host: "c2.example.com", port: "8080", proto: "http" });
    setShowListenerModal(true);
  };

  const submitListener = async () => {
    const { name, ltype, host, port, proto } = listenerForm;
    if (!name || !ltype || !host || !port || !proto) return;
    setShowListenerModal(false);
    try {
      const res = await fetch(`${API_BASE}?p=/listeners`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, type: ltype, host, port: parseInt(port) || 8080, protocol: proto, enabled: true }) });
      const data = await res.json();
      if (data.success) {
        showToast("Listener created successfully", "success");
        loadData();
        const newId = String(data.listener?.ID || data.listener?.id || "");
        if (newId) setShared((prev) => ({ ...prev, listener_id: newId }));
      } else showToast(`Creation failed: ${data.error || ""}`, "error");
    } catch (err: unknown) { showToast(err instanceof Error ? err.message : String(err), "error"); }
  };

  const copyToClipboard = async (text: string) => {
    try { await navigator.clipboard.writeText(text); showToast("Copied!", "success"); }
    catch {
      const ta = document.createElement("textarea"); ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta); ta.select(); document.execCommand("copy"); ta.remove(); showToast("Copied!", "success");
    }
  };

  useEffect(() => {
    if (shared.listener_id) {
      const info = getListenerInfo(shared.listener_id);
      if (info) {
        let url = `${info.scheme}://${info.host}:${info.port}`;
        if (shared.failover.trim()) url += "," + shared.failover.trim();
        setShared((s) => ({ ...s, c2_url: url }));
      }
    } else {
      setShared((s) => ({ ...s, c2_url: "" }));
    }
  }, [shared.listener_id, shared.failover]);

  if (loading) return <div className="flex items-center justify-center h-64"><i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i></div>;

  return (
    <div className="mb-20 md:mb-0">
      <Toast toasts={toasts} />

      <div className="mb-4 sm:mb-6">
        <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-slate-900 dark:text-slate-100">Generate Implant</h1>
        <p className="text-slate-500 dark:text-slate-400 text-xs sm:text-sm mt-1">Generate payloads (EXE / PS1 / ELF / macOS / Shellcode / One-Liner)</p>
        <div className="mt-3 inline-flex items-center gap-2 px-4 py-2 bg-amber-50 border border-amber-200 rounded-xl">
          <i className="fa-solid fa-circle-info text-amber-600 text-sm"></i>
          <span className="text-xs text-amber-700">Native compilation requires local Go toolchain. <a href="https://go.dev/dl/" target="_blank" className="underline hover:text-amber-800">Download</a></span>
        </div>
      </div>

      <SharedSettings
        listeners={listeners}
        shared={shared}
        profilePresets={profilePresets}
        profileLocked={profileLocked}
        showListenerModal={showListenerModal}
        listenerForm={listenerForm}
        setShared={setShared}
        changeProfile={changeProfile}
        handleCreateListener={handleCreateListener}
        submitListener={submitListener}
        setShowListenerModal={setShowListenerModal}
        setListenerForm={setListenerForm}
      />

      <input ref={fileInputRef} type="file" accept=".json,application/json" className="hidden" onChange={handleProfileImport} />

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-5">
        <EXEPanel form={exeForm} setForm={setExeForm} busy={busy.exe} result={results.exe} onGenerate={handleGenerateEXE} />
        <PS1Panel form={ps1Form} setForm={setPs1Form} busy={busy.ps1} result={results.ps1} code={results._ps1Code} originalLen={results._ps1Original} obfuscatedLen={results._ps1Obfuscated} onGenerate={handleGeneratePS1} onCopy={copyToClipboard} />
        <LinuxPanel form={linuxForm} setForm={setLinuxForm} busy={busy.linux} result={results.linux} onGenerate={handleGenerateLinux} />
        <MacOSPanel form={macosForm} setForm={setMacosForm} busy={busy.macos} result={results.macos} onGenerate={handleGenerateMacOS} />
      </div>

      <div className="mt-8">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-indigo-100 rounded-xl flex items-center justify-center"><i className="fa-solid fa-box-open text-indigo-600"></i></div>
          <div>
            <div className="text-sm font-semibold text-slate-900">Artifact Kit</div>
            <div className="text-xs text-slate-500">XOR-encrypted Shellcode loaders</div>
          </div>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <StagerPanel form={stagerForm} setForm={setStagerForm} busy={busy.stager} result={results.stager} onGenerate={handleGenerateStager} />
          <StagerLinuxPanel form={stagerLinuxForm} setForm={setStagerLinuxForm} busy={busy.stager_linux} result={results.stager_linux} onGenerate={handleGenerateStagerLinux} />
        </div>
      </div>

      <div className="mt-8">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-yellow-100 rounded-xl flex items-center justify-center"><i className="fa-solid fa-microchip text-yellow-600"></i></div>
          <div>
            <div className="text-sm font-semibold text-slate-900">Shellcode / Donut / sRDI</div>
            <div className="text-xs text-slate-500">Raw shellcode / .NET → Shellcode (Donut)</div>
          </div>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <ShellcodePanel form={shellcodeForm} setForm={setShellcodeForm} busy={busy.shellcode} result={results.shellcode} onGenerate={handleGenerateShellcode} />
          <DonutPanel form={donutForm} setForm={setDonutForm} busy={busy.donut} result={results.donut} onGenerate={handleGenerateDonut} fileRef={donutFileRef} />
        </div>
      </div>

      <OneLinerPanel
        form={onelinerForm} setForm={setOnelinerForm} busy={busy.oneliner}
        result={results.oneliner} onelinerData={results._onelinerData}
        listeners={listeners} getListenerInfo={getListenerInfo}
        onGenerate={handleGenerateOneLiner} onCopy={copyToClipboard}
      />

      <div className="mt-6 text-center text-xs text-slate-400">
        Generated agents connect back to C2 automatically<br />
        <span className="text-amber-600">For authorized penetration testing only</span>
      </div>
    </div>
  );
}
