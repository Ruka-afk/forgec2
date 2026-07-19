"use client";

import { useEffect, useState, useCallback, useMemo, useRef } from "react";
import { toast } from "sonner";
import { API_BASE } from "@/lib/constants";
import { api, getCsrfToken } from "@/lib/api";
import { downloadFromResponse, parseFilename } from "@/lib/download";
import {
  Listener, ProfilePreset, SharedState, PayloadForms, PayloadStates, PayloadExtras,
  PayloadKey, BinaryForm, UnixForm, createDefaultForms, createDefaultStates,
} from "@/types/generate";

const DEFAULT_UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36";

const DEFAULT_PROFILE_PRESETS: ProfilePreset[] = [
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

export function usePayloadGenerator() {
  const [listeners, setListeners] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(true);
  const [profilePresets, setProfilePresets] = useState<ProfilePreset[]>(DEFAULT_PROFILE_PRESETS);
  const [profileLocked, setProfileLocked] = useState(false);
  const savedManualRef = useRef<{ interval: string; jitter: string; ua: string } | null>(null);
  const prevProfileRef = useRef<string>("default");
  const [showListenerModal, setShowListenerModal] = useState(false);
  const [listenerForm, setListenerForm] = useState({ name: "", ltype: "http", host: "", port: "8443", proto: "http" });

  const [shared, setShared] = useState<SharedState>({
    listener_id: "", c2_url: "", protocol: "http", interval: "5", jitter: "0",
    ua: DEFAULT_UA, proxy: "", failover: "", crypto_key: "", profile: "",
  });

  const [forms, setForms] = useState<PayloadForms>(createDefaultForms);
  const [states, setStates] = useState<PayloadStates>(createDefaultStates);
  const [extras, setExtras] = useState<PayloadExtras>({});

  const fileInputRef = useRef<HTMLInputElement>(null);
  const donutFileRef = useRef<HTMLInputElement>(null);

  // Load listeners
  const loadData = useCallback(async () => {
    try {
      const data = await api.get("/api/listeners");
      setListeners(Array.isArray(data) ? data : []);
    } catch { setListeners([]); } finally { setLoading(false); }
    try {
      const profileData = await api.get<{success: boolean; data?: {profiles?: ProfilePreset[]}}>(`/api/generate/profiles`);
      if (profileData.success && profileData.data?.profiles?.length) {
        setProfilePresets(profileData.data.profiles);
      }
    } catch { /* keep defaults */ }
  }, []);

  useEffect(() => { loadData(); }, [loadData]);

  const getListenerInfo = useCallback((listenerId: string) => {
    const l = listeners.find((x) => String(x.id) === String(listenerId));
    if (!l) return null;
    return {
      scheme: l.scheme || l.type || "http",
      host: l.host || "",
      port: l.port || "",
      type: l.type || "",
      name: l.name || "Unknown",
    };
  }, [listeners]);

  // Shared settings resolved with profile
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
  }, [shared.listener_id, shared.failover, getListenerInfo]);

  const getProtocol = useCallback(() => {
    const info = getListenerInfo(shared.listener_id);
    if (!info) return "http";
    if (info.scheme === "tcp" || info.scheme === "tls") return "tcp";
    if (info.scheme === "dns" || info.type === "dns") return "dns";
    return "http";
  }, [shared.listener_id, getListenerInfo]);

  // Core generate request — for binary builds (exe/dll/linux/macos) this uses
  // the async build system: POST returns a build_id, then we poll until done
  // and download the result.  Non-binary endpoints (ps1, stager, etc.) still
  // use the synchronous flow where the response body IS the result.
  const handleGenerate = useCallback(async (endpoint: string, formName: string, opts?: { isJson?: boolean; async?: boolean; extra?: Record<string, string> }) => {
    if (!shared.listener_id) {
      toast.error("Please select a listener in shared settings first");
      return null;
    }
    const c2url = buildC2URL();
    const protocol = getProtocol();
    const formData = new FormData();
    formData.set("c2_url", c2url);
    formData.set("protocol", protocol);
    buildFormData(formData, opts?.extra);
    try {
      // 1) Start the build (or synchronous request)
      const res = await fetch(`${API_BASE}/generate${endpoint}`, { method: "POST", body: formData, headers: { "X-CSRF-Token": getCsrfToken() }, credentials: "include" });
      if (!res.ok) {
        const ct = res.headers.get("content-type") || "";
        if (ct.includes("application/json")) {
          try { const j = await res.json(); return { error: j.error || JSON.stringify(j) }; } catch { /* fallthrough */ }
        }
        const raw = await res.text();
        const msg = raw.length > 200 ? raw.substring(0, 200) + "..." : raw;
        return { error: msg || `Request failed (${res.status})` };
      }

      // Non-async endpoints: return JSON or download file directly
      if (!opts?.async) {
        if (opts?.isJson) return await res.json();
        await downloadFromResponse(res, formName);
        return { success: true };
      }

      // 2) Async build: poll for completion
      const startData = await res.json();
      const buildId = startData.build_id;
      if (!buildId) return { error: startData.error || "No build ID returned" };

      const pollInterval = 2000;
      const maxAttempts = 300; // 10 minutes max
      for (let i = 0; i < maxAttempts; i++) {
        await new Promise((r) => setTimeout(r, pollInterval));
        const pollRes = await fetch(`${API_BASE}/generate/builds/${buildId}`, { credentials: "include", headers: { "Accept": "application/json" } });
        if (!pollRes.ok) continue;
        const pollData = await pollRes.json();
        if (pollData.status === "completed") {
          // 3) Download the result
          const dlRes = await fetch(`${API_BASE}/generate/builds/${buildId}/download`, { credentials: "include" });
          if (!dlRes.ok) return { error: `Download failed (${dlRes.status})` };
          await downloadFromResponse(dlRes, formName);
          return { success: true };
        }
        if (pollData.status === "failed") {
          return { error: pollData.error || "Build failed" };
        }
        // Still building — continue polling
      }
      return { error: "Build timed out after 10 minutes" };
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes("abort")) return { error: "Request timed out" };
      return { error: msg };
    }
  }, [shared.listener_id, buildC2URL, getProtocol, buildFormData]);

  // Form updaters
  const updateForm = useCallback(<K extends PayloadKey>(key: K, updater: (prev: PayloadForms[K]) => PayloadForms[K]) => {
    setForms((prev) => ({ ...prev, [key]: updater(prev[key]) }));
  }, []);

  // Binary form helper (exe/dll share same shape)
  const binaryExtra = useCallback((form: BinaryForm): Record<string, string> => ({
    persist: form.persist ? "true" : "",
    skip_tls_verify: form.skip_tls ? "true" : "",
    evasion: form.evasion ? "true" : "",
    obfuscate: form.obfuscate ? "true" : "",
    filename: form.filename,
    architecture: form.arch,
    domain_front: form.domain_front,
    p2p_mode: form.p2p_mode,
    p2p_parent: form.p2p_parent,
    p2p_listen_addr: form.p2p_listen_addr,
    dns_domain: form.dns_domain,
    dns_server: form.dns_server,
    working_start: form.working_start,
    working_end: form.working_end,
    working_tz: form.working_tz,
  }), []);

  const unixExtra = useCallback((form: UnixForm): Record<string, string> => ({
    persist: form.persist ? "true" : "",
    skip_tls_verify: form.skip_tls ? "true" : "",
    obfuscate: form.obfuscate ? "true" : "",
    filename: form.filename,
    domain_front: form.domain_front,
    working_start: form.working_start,
    working_end: form.working_end,
    working_tz: form.working_tz,
  }), []);

  // Generic standard handler
  const generateStandard = useCallback(async (
    key: PayloadKey,
    endpoint: string,
    defaultFilename: string,
    extra: Record<string, string>,
    successMsg: string,
    isAsync = false,
  ) => {
    setStates((s) => ({ ...s, [key]: { busy: true, result: "" } }));
    const result = await handleGenerate(endpoint, defaultFilename, { extra, async: isAsync });
    if (result?.error) {
      setStates((s) => ({ ...s, [key]: { busy: false, result: result.error! } }));
    } else if (result?.success) {
      toast.success(successMsg);
      setStates((s) => ({ ...s, [key]: { busy: false, result: "" } }));
    } else {
      setStates((s) => ({ ...s, [key]: { busy: false, result: "" } }));
    }
  }, [handleGenerate, ]);

  // Generate handlers
  const handleGenerateBinary = useCallback(async (key: "exe" | "dll") => {
    const form = forms[key];
    const ext = key === "exe" ? "exe" : "dll";
    await generateStandard(key, `/${key}`, `forge_agent.${ext}`, binaryExtra(form), `${key.toUpperCase()} generated and downloaded successfully!`, true);
  }, [forms, generateStandard, binaryExtra]);

  const handleGeneratePS1 = useCallback(async () => {
    setStates((s) => ({ ...s, ps1: { busy: true, result: "" } }));
    const extra: Record<string, string> = { persist: forms.ps1.persist ? "true" : "", skip_tls_verify: forms.ps1.skip_tls ? "true" : "", filename: forms.ps1.filename };
    const result = await handleGenerate("/ps1", "", { isJson: true, extra });
    if (result?.error) {
      setStates((s) => ({ ...s, ps1: { busy: false, result: result.error! } }));
    } else if (result?.success) {
      toast.success(`Generated! Original: ${result.original_length} B / Obfuscated: ${result.obfuscated_len} B`);
      setStates((s) => ({ ...s, ps1: { busy: false, result: "" } }));
      setExtras((e) => ({ ...e, ps1: { code: result.code, original_length: result.original_length, obfuscated_len: result.obfuscated_len } }));
    } else {
      setStates((s) => ({ ...s, ps1: { busy: false, result: "" } }));
    }
  }, [forms.ps1, handleGenerate, ]);

  const handleGenerateUnix = useCallback(async (key: "linux" | "macos") => {
    await generateStandard(key, `/${key}`, "forge_implant", unixExtra(forms[key]), `${key === "linux" ? "Linux ELF" : "macOS Binary"} generated and downloaded successfully!`, true);
  }, [forms, generateStandard, unixExtra]);

  const handleGenerateStager = useCallback(async (key: "stager" | "stager_linux") => {
    const form = forms[key];
    const endpoint = key === "stager" ? "/stager" : "/stager_linux";
    const filename = key === "stager" ? "stager.exe" : "stager";
    const label = key === "stager" ? "Windows" : "Linux";
    await generateStandard(key, endpoint, filename, { filename: form.filename, skip_tls_verify: form.skip_tls ? "true" : "" }, `${label} Loader + staging payload generated successfully!`);
  }, [forms, generateStandard]);

  const handleGenerateShellcode = useCallback(async () => {
    await generateStandard("shellcode", "/shellcode", "shellcode.bin", { command: forms.shellcode.command, filename: forms.shellcode.filename }, "Shellcode generated and downloaded successfully!");
  }, [forms.shellcode, generateStandard]);

  const handleGenerateDonut = useCallback(async () => {
    setStates((s) => ({ ...s, donut: { busy: true, result: "" } }));
    if (!forms.donut.assembly) {
      toast.error("Please select a .NET assembly file");
      setStates((s) => ({ ...s, donut: { busy: false, result: "" } }));
      return;
    }
    const c2url = buildC2URL();
    const protocol = getProtocol();
    const formData = new FormData();
    formData.set("c2_url", c2url); formData.set("protocol", protocol);
    buildFormData(formData);
    formData.set("arch", forms.donut.arch); formData.set("class", forms.donut.class);
    formData.set("method", forms.donut.method); formData.set("args", forms.donut.args);
    formData.set("filename", forms.donut.filename); formData.set("assembly", forms.donut.assembly);
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 120000);
      try {
        const res = await fetch(`${API_BASE}/generate/donut`, { method: "POST", body: formData, headers: { "X-CSRF-Token": getCsrfToken() }, credentials: "include", signal: controller.signal });
        if (res.ok) {
          await downloadFromResponse(res, "donut_loader.bin");
          toast.success("Donut Shellcode generated and downloaded successfully!");
          setStates((s) => ({ ...s, donut: { busy: false, result: "" } }));
        } else {
          const text = await res.text();
          setStates((s) => ({ ...s, donut: { busy: false, result: text } }));
        }
      } finally {
        clearTimeout(timeoutId);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setStates((s) => ({ ...s, donut: { busy: false, result: msg } }));
    }
  }, [forms.donut, buildC2URL, getProtocol, buildFormData, ]);

  const handleGenerateOneLiner = useCallback(async () => {
    setStates((s) => ({ ...s, oneliner: { busy: true, result: "" } }));
    if (!forms.oneliner.listener_id) {
      toast.error("Please select a listener first");
      setStates((s) => ({ ...s, oneliner: { busy: false, result: "" } }));
      return;
    }
    const info = getListenerInfo(forms.oneliner.listener_id);
    let c2url = "";
    let protocol = "http";
    if (info) {
      c2url = `${info.scheme}://${info.host}:${info.port}`;
      if (info.scheme === "tcp" || info.scheme === "tls") protocol = "tcp";
      else if (info.scheme === "dns" || info.type === "dns") protocol = "dns";
    }
    const formData = new FormData();
    formData.set("c2_url", c2url); formData.set("protocol", protocol);
    formData.set("listener_id", forms.oneliner.listener_id);
    formData.set("payload_type", forms.oneliner.payload_type);
    formData.set("beacon_time", forms.oneliner.beacon_time);
    formData.set("jitter", forms.oneliner.jitter);
    formData.set("skip_tls_verify", forms.oneliner.skip_tls ? "true" : "");
    formData.set("persist", forms.oneliner.persist ? "true" : "");
    const s = getSharedSettings();
    formData.set("profile", s.profile); formData.set("interval", s.interval);
    formData.set("user_agent", s.user_agent); formData.set("proxy", s.proxy);
    formData.set("crypto_key", s.crypto_key);
    try {
      const data = await api.postFormData<{ success?: boolean; error?: string; types?: Array<{ name: string; desc: string; command: string }>; download_url?: string }>("/generate/one-liner", formData);
      if (!data.success) {
        setStates((s) => ({ ...s, oneliner: { busy: false, result: data.error || "" } }));
      } else {
        toast.success(`Generated ${data.types?.length ?? 0} One-Liner commands`);
        setStates((s) => ({ ...s, oneliner: { busy: false, result: "success" } }));
        setExtras((e) => ({ ...e, oneliner: { data: { download_url: data.download_url || "", types: data.types || [] } } }));
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setStates((s) => ({ ...s, oneliner: { busy: false, result: msg } }));
    }
  }, [forms.oneliner, getListenerInfo, getSharedSettings, ]);

  // Profile management
  const changeProfile = useCallback((profile: string) => {
    if (profile === "__import__") {
      if (fileInputRef.current) fileInputRef.current.click();
      return;
    }
    if (profile === "default" || profile === "") {
      setProfileLocked(false);
      if (savedManualRef.current) {
        setShared((s) => ({ ...s, interval: savedManualRef.current!.interval, jitter: savedManualRef.current!.jitter, ua: savedManualRef.current!.ua }));
      }
    } else {
      if (!profileLocked) savedManualRef.current = { interval: shared.interval, jitter: shared.jitter, ua: shared.ua };
      const preset = profilePresets.find((p) => p.name === profile);
      if (preset) setShared((s) => ({ ...s, interval: String(preset.sleep && preset.sleep > 0 ? preset.sleep : 10), jitter: String(preset.jitter && preset.jitter >= 0 ? preset.jitter : 20), ua: preset.user_agent || s.ua }));
      setProfileLocked(true);
    }
    prevProfileRef.current = profile;
    setShared((s) => ({ ...s, profile }));
  }, [shared, profilePresets, profileLocked]);

  const handleProfileImport = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const fd = new FormData();
    fd.append("profile", file);
    try {
      const data = await api.postFormData<{ success?: boolean; error?: string; profile?: ProfilePreset }>("/api/generate/profile/import", fd);
      if (!data.success) { toast.error(data.error || "Import failed"); return; }
      const p = data.profile;
      if (!p) return;
      const preset: ProfilePreset = { name: p.name, description: p.description, user_agent: p.user_agent, sleep: p.sleep, jitter: p.jitter };
      setProfilePresets((prev) => { const idx = prev.findIndex((p) => p.name === preset.name); if (idx >= 0) { const next = [...prev]; next[idx] = preset; return next; } return [...prev, preset]; });
      toast.success("Profile imported");
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : String(err)); }
    finally { e.target.value = ""; }
  }, []);

  const deleteProfile = useCallback(async (name: string) => {
    if (!name || name === "default") return false;
    try {
      const data = await api.del(`/api/generate/profile/${name}`);
      if (data.success) {
        setProfilePresets((prev) => prev.filter((p) => p.name !== name));
        changeProfile("default");
        return true;
      }
    } catch { toast.error("Failed to delete profile"); }
    return false;
  }, [changeProfile]);

  // Listener management
  const handleCreateListener = useCallback(() => {
    setListenerForm({ name: "My DNS Listener", ltype: "http", host: "c2.example.com", port: "8080", proto: "http" });
    setShowListenerModal(true);
  }, []);

  const submitListener = useCallback(async () => {
    const { name, ltype, host, port, proto } = listenerForm;
    if (!name) { toast.error("Listener name is required"); return; }
    if (!host) { toast.error("Host is required"); return; }
    if (!port) { toast.error("Port is required"); return; }
    try {
      const data = await api.postJson<{ success: boolean; error?: string; listener?: { ID?: number; id?: number } }>("/api/listeners", { name, type: ltype, host, port: parseInt(port) || 8080, protocol: proto, enabled: true });
      if (data.success) {
        setShowListenerModal(false);
        toast.success("Listener created successfully");
        loadData();
        const newId = String(data.listener?.ID || data.listener?.id || "");
        if (newId) setShared((prev) => ({ ...prev, listener_id: newId }));
      } else toast.error(`Creation failed: ${data.error || ""}`);
    } catch (err: unknown) { toast.error(err instanceof Error ? err.message : String(err)); }
  }, [listenerForm, loadData]);

  const copyToClipboard = useCallback(async (text: string) => {
    try { await navigator.clipboard.writeText(text); toast.success("Copied!"); }
    catch {
      const ta = document.createElement("textarea"); ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
      document.body.appendChild(ta); ta.select(); document.execCommand("copy"); ta.remove(); toast.success("Copied!");
    }
  }, []);

  // Sync c2_url with listener
  useEffect(() => {
    if (shared.listener_id) {
      const info = getListenerInfo(shared.listener_id);
      if (info) {
        let url = `${info.scheme}://${info.host}:${info.port}`;
        if (shared.failover.trim()) url += "," + shared.failover.trim();
        setShared((s) => s.c2_url === url ? s : { ...s, c2_url: url });
      }
    } else {
      setShared((s) => s.c2_url === "" ? s : { ...s, c2_url: "" });
    }
  }, [shared.listener_id, shared.failover, getListenerInfo]);

  // Quick presets
  const applyPreset = useCallback((preset: "opsec" | "evasion" | "blend") => {
    const resetBinary = (f: BinaryForm) => ({ ...f, persist: false, skip_tls: false, evasion: false, obfuscate: false });
    const maxBinary = (f: BinaryForm) => ({ ...f, persist: true, skip_tls: false, evasion: true, obfuscate: true });
    switch (preset) {
      case "opsec":
        setForms((f) => ({ ...f, exe: resetBinary(f.exe), dll: resetBinary(f.dll) }));
        setShared((s) => ({ ...s, interval: "5", jitter: "0" }));
        break;
      case "evasion":
        setForms((f) => ({ ...f, exe: maxBinary(f.exe), dll: maxBinary(f.dll) }));
        setShared((s) => ({ ...s, interval: "30", jitter: "50" }));
        break;
      case "blend":
        setForms((f) => ({ ...f, exe: resetBinary(f.exe), dll: resetBinary(f.dll) }));
        changeProfile("google");
        break;
    }
  }, [changeProfile]);

  const handlerMap = useMemo(() => ({
    exe: () => handleGenerateBinary("exe"),
    dll: () => handleGenerateBinary("dll"),
    ps1: handleGeneratePS1,
    linux: () => handleGenerateUnix("linux"),
    macos: () => handleGenerateUnix("macos"),
    stager: () => handleGenerateStager("stager"),
    stager_linux: () => handleGenerateStager("stager_linux"),
    shellcode: handleGenerateShellcode,
    donut: handleGenerateDonut,
    oneliner: handleGenerateOneLiner,
  }), [handleGenerateBinary, handleGeneratePS1, handleGenerateUnix, handleGenerateStager, handleGenerateShellcode, handleGenerateDonut, handleGenerateOneLiner]);

  return {
    listeners, loading, profilePresets, profileLocked,
    showListenerModal, setShowListenerModal,
    listenerForm, setListenerForm,
    shared, setShared,
    forms, setForms, updateForm,
    states, setStates,
    extras,
    fileInputRef, donutFileRef,
    changeProfile, handleProfileImport, deleteProfile,
    handleCreateListener, submitListener,
    copyToClipboard, getListenerInfo,
    handlerMap, applyPreset,
  };
}

