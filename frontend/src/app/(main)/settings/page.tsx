"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";
import { SettingsData, AgentForm, ServerForm, MalleableForm, PasswordForm } from "./_components/types";
import ProfileSection from "./_components/ProfileSection";
import ThemeSection from "./_components/ThemeSection";
import LanguageSection from "./_components/LanguageSection";
import SecuritySection from "./_components/SecuritySection";
import ServerSection from "./_components/ServerSection";
import AgentSection from "./_components/AgentSection";
import MalleableSection from "./_components/MalleableSection";
import DatabaseSection from "./_components/DatabaseSection";
import MaintenanceSection from "./_components/MaintenanceSection";
import AboutSection from "./_components/AboutSection";

export default function SettingsPage() {
  const router = useRouter();
  const [data, setData] = useState<SettingsData>({});
  const [loading, setLoading] = useState(true);
  const [activeSection, setActiveSection] = useState("profile");
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState<{ type: "success" | "error"; msg: string } | null>(null);

  const [agentForm, setAgentForm] = useState<AgentForm>({ interval: 5, jitter: 10, skip_tls: false, user_agent: "" });
  const [serverForm, setServerForm] = useState<ServerForm>({ log_level: "info", tcp_enabled: false, tcp_addr: "", offline_threshold: 60, session_max_age: 24, cleanup_retention: 30 });
  const [malleableForm, setMalleableForm] = useState<MalleableForm>({ enabled: false, status_code: 200, content_type: "application/json", headers_text: "", prepend: "", append: "" });
  const [passwordForm, setPasswordForm] = useState<PasswordForm>({ current: "", next: "", confirm: "" });
  const [theme, setTheme] = useState<string>("system");
  const [language, setLanguage] = useState<string>("en");
  const [purgeDays, setPurgeDays] = useState({ tasks: "30", audit: "30" });
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [cfmInline, setCfmInline] = useState<{msg: string; cb: () => void} | null>(null);
  const [totpStatus, setTotpStatus] = useState<boolean | null>(null);
  const [totpSecret, setTotpSecret] = useState("");
  const [totpQR, setTotpQR] = useState("");
  const [totpBackupCodes, setTotpBackupCodes] = useState("");
  const [totpCode, setTotpCode] = useState("");
  const [showTotpSetup, setShowTotpSetup] = useState(false);
  const [totpDisablePassword, setTotpDisablePassword] = useState("");

  const showToast = useCallback((type: "success" | "error", msg: string) => {
    setToast({ type, msg });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const loadSettings = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/settings&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const d = await res.json();
      setData(d);
      setAgentForm({
        interval: d.DefaultInterval ?? d.default_interval ?? 5,
        jitter: d.DefaultJitter ?? d.default_jitter ?? 10,
        skip_tls: d.DefaultSkipTLS ?? d.default_skip_tls ?? false,
        user_agent: d.DefaultUA ?? d.default_ua ?? "",
      });
      setServerForm({
        log_level: d.LogLevel ?? d.log_level ?? "info",
        tcp_enabled: d.TCPEnabled ?? d.tcp_enabled ?? false,
        tcp_addr: d.TCPAddr ?? d.tcp_addr ?? "",
        offline_threshold: d.OfflineThreshold ?? d.offline_threshold ?? 60,
        session_max_age: d.SessionMaxAge ?? d.session_max_age ?? 24,
        cleanup_retention: d.CleanupRetention ?? d.cleanup_retention ?? 30,
      });
      setMalleableForm({
        enabled: d.MalleableEnabled ?? d.malleable_enabled ?? false,
        status_code: d.MalleableStatus ?? d.malleable_status ?? 200,
        content_type: d.MalleableCT ?? d.malleable_ct ?? "application/json",
        headers_text: "",
        prepend: d.MalleablePrepend ?? d.malleable_prepend ?? "",
        append: d.MalleableAppend ?? d.malleable_append ?? "",
      });
      try {
        const storedTheme = localStorage.getItem("forgec2_theme");
        if (storedTheme) setTheme(storedTheme);
        const storedLang = document.cookie.match(/forgec2_lang=([^;]+)/);
        if (storedLang) setLanguage(storedLang[1]);
      } catch (e) { console.error("Settings: load stored settings failed", e); }
    } catch {
      showToast("error", "Failed to load settings");
    } finally {
      setLoading(false);
    }
  }, [showToast]);

  useEffect(() => { Promise.resolve().then(() => loadSettings()); }, [loadSettings]);

  useEffect(() => {
    if (activeSection === "security") {
      fetch(`${API_BASE}?p=/settings/totp/status&format=json`)
        .then(async (r) => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); })
        .then((d) => setTotpStatus(d.enabled ?? d.Enabled ?? false))
        .catch(() => setTotpStatus(false));
    }
  }, [activeSection]);

  const handleSaveAgent = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const body = new URLSearchParams({ interval: String(agentForm.interval), jitter: String(agentForm.jitter), skip_tls: String(agentForm.skip_tls), user_agent: agentForm.user_agent });
      await fetch(`${API_BASE}?p=/settings/agent&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Agent configuration saved");
    } catch { showToast("error", "Failed to save agent config"); }
    finally { setSaving(false); }
  };

  const handleSaveServer = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const body = new URLSearchParams({
        log_level: serverForm.log_level, tcp_enabled: String(serverForm.tcp_enabled), tcp_addr: serverForm.tcp_addr,
        offline_threshold: String(serverForm.offline_threshold), session_max_age: String(serverForm.session_max_age), cleanup_retention: String(serverForm.cleanup_retention),
      });
      await fetch(`${API_BASE}?p=/settings/server&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Server configuration saved");
    } catch { showToast("error", "Failed to save server config"); }
    finally { setSaving(false); }
  };

  const handleSaveMalleable = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const body = new URLSearchParams({
        enabled: String(malleableForm.enabled), status_code: String(malleableForm.status_code), content_type: malleableForm.content_type,
        headers_text: malleableForm.headers_text, prepend: malleableForm.prepend, append: malleableForm.append,
      });
      await fetch(`${API_BASE}?p=/settings/malleable&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Malleable C2 profile saved");
    } catch { showToast("error", "Failed to save malleable profile"); }
    finally { setSaving(false); }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (passwordForm.next !== passwordForm.confirm) { showToast("error", "New passwords do not match"); return; }
    setSaving(true);
    try {
      const body = new URLSearchParams({ current_password: passwordForm.current, new_password: passwordForm.next, confirm_password: passwordForm.confirm });
      await fetch(`${API_BASE}?p=/settings/password&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Password changed");
      setPasswordForm({ current: "", next: "", confirm: "" });
    } catch { showToast("error", "Failed to change password"); }
    finally { setSaving(false); }
  };

  const handleRegenerateJWT = () => {
    setCfm({msg: "Regenerate JWT key? All current sessions will be invalidated.", cb: async () => {
      setSaving(true);
      try { await fetch(`${API_BASE}?p=/settings/jwt/regenerate&format=json`, { method: "POST" }); showToast("success", "JWT key regenerated"); loadSettings(); }
      catch { showToast("error", "Failed to regenerate JWT"); }
      finally { setSaving(false); }
    }});
  };

  const handleApplyTheme = (t: string) => {
    setTheme(t);
    try {
      localStorage.setItem("forgec2_theme", t);
      const root = document.documentElement;
      if (t === "dark") root.classList.add("dark");
      else if (t === "light") root.classList.remove("dark");
      else {
        const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
        if (dark) root.classList.add("dark"); else root.classList.remove("dark");
      }
    } catch (e) { console.error("Settings: apply theme failed", e); }
  };

  const handleSetLanguage = (code: string) => {
    setLanguage(code);
    setTimeout(() => {
      try { document.cookie = `forgec2_lang=${code};path=/;max-age=31536000`; router.refresh(); }
      catch (e) { console.error("Settings: set language failed", e); }
    }, 0);
  };

  const handleVacuum = async () => {
    setSaving(true);
    try { await fetch(`${API_BASE}?p=/settings/db/vacuum&format=json`, { method: "POST" }); showToast("success", "Database vacuumed"); }
    catch { showToast("error", "Failed to vacuum database"); }
    finally { setSaving(false); }
  };

  const handleBackup = async () => {
    setSaving(true);
    try {
      const res = await fetch(`${API_BASE}?p=/settings/db/backup&format=json`, { method: "POST" });
      if (res.ok) {
        const blob = await res.blob(); const url = URL.createObjectURL(blob);
        const a = document.createElement("a"); a.href = url; a.download = `forgec2_backup_${Date.now()}.db`;
        a.click(); URL.revokeObjectURL(url);
      }
      showToast("success", "Database backup created");
    } catch { showToast("error", "Failed to backup database"); }
    finally { setSaving(false); }
  };

  const handleDownloadDB = async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/settings/config/download&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob(); const url = URL.createObjectURL(blob);
      const a = document.createElement("a"); a.href = url; a.download = "forgec2.db";
      a.click(); URL.revokeObjectURL(url);
    } catch { showToast("error", "Failed to download database"); }
  };

  const handlePurge = (type: string) => {
    const days = type === "tasks" ? purgeDays.tasks : purgeDays.audit;
    setCfm({msg: `Purge ${type} older than ${days} days? This cannot be undone.`, cb: async () => {
      setSaving(true);
      try {
        const body = new URLSearchParams({ days });
        const path = type === "screenshots" ? "/settings/maintenance/purge" : `/settings/purge/${type}`;
        await fetch(`${API_BASE}?p=${path}&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
        showToast("success", `Purged old ${type}`);
      } catch { showToast("error", `Failed to purge ${type}`); }
      finally { setSaving(false); }
    }});
  };

  const handleGenerateTOTP = async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/settings/totp/generate&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const d = await res.json();
      setTotpSecret(d.secret || d.Secret || "");
      setTotpQR(d.qr || d.QR || d.qr_code || "");
      setTotpBackupCodes((d.backup_codes || d.BackupCodes || []).join("\n"));
      setShowTotpSetup(true);
    } catch { showToast("error", "Failed to generate TOTP"); }
  };

  const handleEnableTOTP = async () => {
    if (!totpCode) { showToast("error", "Please enter verification code"); return; }
    setSaving(true);
    try {
      const body = new URLSearchParams({ code: totpCode, secret: totpSecret });
      await fetch(`${API_BASE}?p=/settings/totp/enable&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Two-factor authentication enabled");
      setTotpStatus(true); setShowTotpSetup(false); setTotpCode("");
    } catch { showToast("error", "Failed to enable TOTP"); }
    finally { setSaving(false); }
  };

  const handleDisableTOTP = async () => {
    if (!totpDisablePassword) { showToast("error", "Please enter password"); return; }
    setSaving(true);
    try {
      const body = new URLSearchParams({ password: totpDisablePassword });
      await fetch(`${API_BASE}?p=/settings/totp/disable&format=json`, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: body.toString() });
      showToast("success", "Two-factor authentication disabled");
      setTotpStatus(false); setTotpDisablePassword("");
    } catch { showToast("error", "Failed to disable TOTP"); }
    finally { setSaving(false); }
  };

  const handleCheckUpdate = async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/api/update-check&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const d = await res.json();
      if (d.update_available || d.UpdateAvailable) showToast("success", `New version: ${d.version || d.Version}`);
      else showToast("success", "You are running the latest version");
    } catch { showToast("error", "Failed to check for updates"); }
  };

  if (loading) return <div className="flex items-center justify-center h-64"><i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i></div>;

  const sections = [
    { key: "profile", label: "Profile", icon: "fa-user" },
    { key: "theme", label: "Theme", icon: "fa-palette" },
    { key: "language", label: "Language", icon: "fa-language" },
    { key: "security", label: "Security", icon: "fa-lock" },
    { key: "server", label: "Server", icon: "fa-server" },
    { key: "agent", label: "Agent", icon: "fa-robot" },
    { key: "malleable", label: "Malleable C2", icon: "fa-shield" },
    { key: "database", label: "Database", icon: "fa-database" },
    { key: "maintenance", label: "Maintenance", icon: "fa-broom" },
    { key: "about", label: "About", icon: "fa-microchip" },
  ];

  const inputCls = "w-full bg-[var(--card-bg)] border border-[var(--border)] text-[var(--text-primary)] focus:border-indigo-500 rounded-xl px-4 h-11 text-sm";
  const textareaCls = "w-full bg-[var(--card-bg)] border border-[var(--border)] text-[var(--text-primary)] focus:border-indigo-500 rounded-xl px-4 py-3 text-sm font-mono";

  return (
    <div className="mb-20 md:mb-0">
      <div className="page-header">
        <h1 className="page-title">Settings</h1>
        <p className="page-subtitle">Configure system preferences and security settings</p>
      </div>

      {toast && (
        <div className={`fixed top-4 right-4 z-50 px-4 py-3 rounded-xl text-sm font-medium shadow-lg ${toast.type === "success" ? "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300" : "bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"}`}>
          <i className={`fa-solid ${toast.type === "success" ? "fa-check-circle" : "fa-exclamation-circle"} mr-2`}></i>
          {toast.msg}
        </div>
      )}

      <div className="flex flex-col lg:flex-row gap-6">
        <div className="w-48 shrink-0 hidden lg:block">
          <div className="sticky top-6 space-y-1">
            <div className="text-[10px] uppercase tracking-wider text-slate-400 px-3 mb-2 font-semibold">Settings</div>
            {sections.map((s) => (
              <button key={s.key} onClick={() => setActiveSection(s.key)}
                className={`block w-full text-left px-3 py-2 text-xs rounded-xl transition-colors ${activeSection === s.key ? "bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-400" : "hover:bg-indigo-50 dark:hover:bg-indigo-900/30 hover:text-indigo-700 dark:hover:text-indigo-400 text-slate-600 dark:text-slate-400"}`}>
                <i className={`fa-solid ${s.icon} mr-2 w-4 text-center`}></i>{s.label}
              </button>
            ))}
          </div>
        </div>

        <div className="flex-1 min-w-0">
          <nav className="lg:hidden mb-4 overflow-x-auto flex gap-2 pb-2">
            {sections.map((s) => (
              <button key={s.key} onClick={() => setActiveSection(s.key)}
                className={`shrink-0 px-3 py-1.5 text-xs rounded-xl transition-colors ${activeSection === s.key ? "bg-indigo-100 dark:bg-indigo-900/40 text-indigo-700 dark:text-indigo-300" : "bg-slate-100 dark:bg-slate-700 text-slate-600 dark:text-slate-400"}`}>
                <i className={`fa-solid ${s.icon} mr-1`}></i>{s.label}
              </button>
            ))}
          </nav>

          <div className="space-y-6">
            {activeSection === "profile" && <ProfileSection data={data} />}
            {activeSection === "theme" && <ThemeSection theme={theme} onApplyTheme={handleApplyTheme} />}
            {activeSection === "language" && <LanguageSection language={language} onSetLanguage={handleSetLanguage} />}
            {activeSection === "security" && (
              <SecuritySection
                data={data} passwordForm={passwordForm} setPasswordForm={setPasswordForm}
                totpStatus={totpStatus} totpSecret={totpSecret} totpQR={totpQR} totpBackupCodes={totpBackupCodes}
                totpCode={totpCode} setTotpCode={setTotpCode} showTotpSetup={showTotpSetup} setShowTotpSetup={setShowTotpSetup}
                totpDisablePassword={totpDisablePassword} setTotpDisablePassword={setTotpDisablePassword}
                saving={saving} onChangePassword={handleChangePassword} onRegenerateJWT={handleRegenerateJWT}
                onGenerateTOTP={handleGenerateTOTP} onEnableTOTP={handleEnableTOTP} onDisableTOTP={handleDisableTOTP} inputCls={inputCls}
              />
            )}
            {activeSection === "server" && <ServerSection data={data} form={serverForm} setForm={setServerForm} saving={saving} inputCls={inputCls} onSave={handleSaveServer} />}
            {activeSection === "agent" && <AgentSection form={agentForm} setForm={setAgentForm} saving={saving} inputCls={inputCls} onSave={handleSaveAgent} />}
            {activeSection === "malleable" && <MalleableSection form={malleableForm} setForm={setMalleableForm} saving={saving} inputCls={inputCls} textareaCls={textareaCls} onSave={handleSaveMalleable} />}
            {activeSection === "database" && <DatabaseSection data={data} saving={saving} onVacuum={handleVacuum} onBackup={handleBackup} onDownloadDB={handleDownloadDB} />}
            {activeSection === "maintenance" && <MaintenanceSection purgeDays={purgeDays} setPurgeDays={setPurgeDays} saving={saving} onPurge={handlePurge} onPurgeScreenshots={() => setCfmInline({msg: "Delete all screenshots? This cannot be undone.", cb: () => handlePurge("screenshots") })} />}
            {activeSection === "about" && <AboutSection data={data} saving={saving} onCheckUpdate={handleCheckUpdate} />}
          </div>
        </div>
      </div>

      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Confirm" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
      <ConfirmModal open={!!cfmInline} title="Confirm" message={cfmInline?.msg || ""} confirmText="Purge" cancelText="Cancel" danger onConfirm={() => { cfmInline?.cb(); setCfmInline(null); }} onCancel={() => setCfmInline(null)} />
    </div>
  );
}
