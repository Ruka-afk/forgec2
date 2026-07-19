"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { downloadBlob } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { PageHeader, PageSpinner } from "@/components/UI";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { Bell, Bot, CloudUpload, Cpu, Database, Globe, Lock, LogIn, Palette, Server, Shield, User, Wrench, Archive, Radio } from "lucide-react";
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
import NotificationsSection from "./_components/NotificationsSection";
import SyncSection from "./_components/SyncSection";
import SIEMSection from "./_components/SIEMSection";
import BackupSection from "./_components/BackupSection";
import ExtC2Section from "./_components/ExtC2Section";

export default function SettingsPage() {
  const router = useRouter();
  const { t } = useI18n();
  const [data, setData] = useState<SettingsData>({});
  const [loading, setLoading] = useState(true);
  const [activeSection, setActiveSection] = useState("profile");
  const [saving, setSaving] = useState(false);
  const [agentForm, setAgentForm] = useState<AgentForm>({ interval: 5, jitter: 10, skip_tls: false, user_agent: "", working_start: "", working_end: "", working_tz: "" });
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

  const loadSettings = useCallback(async () => {
    try {
      const d = await api.get("/settings") as unknown as SettingsData;
      setData(d);
      setAgentForm({
        interval: d.default_interval ?? 5,
        jitter: d.default_jitter ?? 10,
        skip_tls: d.default_skip_tls ?? false,
        user_agent: d.default_ua ?? "",
        working_start: d.working_start ?? "",
        working_end: d.working_end ?? "",
        working_tz: d.working_tz ?? "",
      });
      setServerForm({
        log_level: d.log_level ?? "info",
        tcp_enabled: d.tcp_enabled ?? false,
        tcp_addr: d.tcp_addr ?? "",
        offline_threshold: d.offline_threshold ?? 60,
        session_max_age: d.session_max_age ?? 24,
        cleanup_retention: d.cleanup_retention ?? 30,
      });
      setMalleableForm({
        enabled: d.malleable_enabled ?? false,
        status_code: d.malleable_status ?? 200,
        content_type: d.malleable_ct ?? "application/json",
        headers_text: "",
        prepend: d.malleable_prepend ?? "",
        append: d.malleable_append ?? "",
      });
      try {
        const storedTheme = localStorage.getItem("forgec2_theme");
        if (storedTheme) setTheme(storedTheme);
        const storedLang = document.cookie.match(/forgec2_lang=([^;]+)/);
        if (storedLang) setLanguage(storedLang[1]);
      } catch { /* silent */ }
    } catch {
      toast.error(t("settings.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { loadSettings(); }, [loadSettings]);

  useEffect(() => {
    if (activeSection === "security") {
      api.get("/settings/totp/status")
        .then((d: Record<string, unknown>) => setTotpStatus((d.enabled ?? false) as boolean))
        .catch(() => setTotpStatus(false));
    }
  }, [activeSection]);

  const withSaveTimeout = useCallback(<T extends unknown[]>(fn: (...args: T) => Promise<void>) => {
    return async (...args: T) => {
      setSaving(true);
      const timer = setTimeout(() => {
        setSaving(false);
        toast.error(t("settings.toast.save_timeout"));
      }, 10000);
      try {
        await fn(...args);
      } finally {
        clearTimeout(timer);
        setSaving(false);
      }
    };
  }, [t]);

  const handleSaveAgent = withSaveTimeout(async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/settings/agent", { interval: String(agentForm.interval), jitter: String(agentForm.jitter), skip_tls: String(agentForm.skip_tls), user_agent: agentForm.user_agent, working_start: agentForm.working_start, working_end: agentForm.working_end, working_tz: agentForm.working_tz });
    toast.success(t("settings.toast.agent_saved"));
  });

  const handleSaveServer = withSaveTimeout(async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/settings/server", {
      log_level: serverForm.log_level, tcp_enabled: String(serverForm.tcp_enabled), tcp_addr: serverForm.tcp_addr,
      offline_threshold: String(serverForm.offline_threshold), session_max_age: String(serverForm.session_max_age), cleanup_retention: String(serverForm.cleanup_retention),
    });
    toast.success(t("settings.toast.server_saved"));
  });

  const handleSaveMalleable = withSaveTimeout(async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post("/settings/malleable", {
      enabled: String(malleableForm.enabled), status_code: String(malleableForm.status_code), content_type: malleableForm.content_type,
      headers_text: malleableForm.headers_text, prepend: malleableForm.prepend, append: malleableForm.append,
    });
    toast.success(t("settings.toast.malleable_saved"));
  });

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (passwordForm.next !== passwordForm.confirm) { toast.error(t("settings.toast.password_mismatch")); return; }
    setSaving(true);
    try {
      await api.post("/settings/password", { current_password: passwordForm.current, new_password: passwordForm.next, confirm_password: passwordForm.confirm });
      toast.success(t("settings.toast.password_changed"));
      setPasswordForm({ current: "", next: "", confirm: "" });
    } catch { toast.error(t("settings.toast.password_change_failed")); }
    finally { setSaving(false); }
  };

  const handleRegenerateJWT = () => {
    setCfm({msg: t("settings.confirm.jwt"), cb: async () => {
      setSaving(true);
      try { await api.post("/settings/jwt/regenerate"); toast.success(t("settings.toast.jwt_regenerated")); loadSettings(); }
      catch { toast.error(t("settings.toast.jwt_failed")); }
      finally { setSaving(false); }
    }});
  };

  const handleApplyTheme = (th: string) => {
    setTheme(th);
    try {
      localStorage.setItem("forgec2_theme", th);
      const root = document.documentElement;
      if (th === "dark") root.classList.add("dark");
      else if (th === "light") root.classList.remove("dark");
      else {
        const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
        if (dark) root.classList.add("dark"); else root.classList.remove("dark");
      }
    } catch { /* silent */ }
  };

  const handleSetLanguage = (code: string) => {
    setLanguage(code);
    setTimeout(() => {
      try { document.cookie = `forgec2_lang=${code};path=/;max-age=31536000;SameSite=Strict`; router.refresh(); }
      catch { /* silent */ }
    }, 0);
  };

  const handleVacuum = async () => {
    setSaving(true);
    try { await api.post("/settings/db/vacuum"); toast.success(t("settings.toast.vacuum_done")); }
    catch { toast.error(t("settings.toast.vacuum_failed")); }
    finally { setSaving(false); }
  };

  const handleBackup = async () => {
    setSaving(true);
    try {
      const { blob } = await api.download("/settings/db/backup");
      downloadBlob(blob, `forgec2_backup_${Date.now()}.db`);
      toast.success(t("settings.toast.backup_done"));
    } catch { toast.error(t("settings.toast.backup_failed")); }
    finally { setSaving(false); }
  };

  const handleDownloadDB = async () => {
    try {
      const { blob } = await api.download("/settings/config/download");
      downloadBlob(blob, "forgec2.db");
    } catch { toast.error(t("settings.toast.download_failed")); }
  };

  const handlePurge = (type: string) => {
    const days = type === "tasks" ? purgeDays.tasks : purgeDays.audit;
    setCfm({msg: t("settings.confirm.purge", { type, days }), cb: async () => {
      setSaving(true);
      try {
        const path = type === "screenshots" ? "/settings/maintenance/purge" : `/settings/purge/${type}`;
        await api.post(path, { days });
        toast.success(t("settings.toast.purge_done", { type }));
      } catch { toast.error(t("settings.toast.purge_failed", { type })); }
      finally { setSaving(false); }
    }});
  };

  const handleGenerateTOTP = async () => {
    try {
      const d = await api.post("/settings/totp/generate");
      setTotpSecret((d.secret || "") as string);
      setTotpQR((d.qr || d.qr_code || "") as string);
      setTotpBackupCodes(((d.backup_codes || []) as string[]).join("\n"));
      setShowTotpSetup(true);
    } catch { toast.error(t("settings.toast.totp_generate_failed")); }
  };

  const handleEnableTOTP = async () => {
    if (!totpCode) { toast.error(t("settings.toast.totp_enter_code")); return; }
    setSaving(true);
    try {
      await api.post("/settings/totp/enable", { code: totpCode, secret: totpSecret });
      toast.success(t("settings.toast.totp_enabled"));
      setTotpStatus(true); setShowTotpSetup(false); setTotpCode("");
    } catch { toast.error(t("settings.toast.totp_enable_failed")); }
    finally { setSaving(false); }
  };

  const handleDisableTOTP = async () => {
    if (!totpDisablePassword) { toast.error(t("settings.toast.totp_enter_password")); return; }
    setSaving(true);
    try {
      await api.post("/settings/totp/disable", { password: totpDisablePassword });
      toast.success(t("settings.toast.totp_disabled"));
      setTotpStatus(false); setTotpDisablePassword("");
    } catch { toast.error(t("settings.toast.totp_disable_failed")); }
    finally { setSaving(false); }
  };

  const handleCheckUpdate = async () => {
    try {
      const d = await api.get("/api/update-check");
      if (d.update_available) toast.success(t("settings.toast.update_new", { version: String(d.version) }));
      else toast.success(t("settings.toast.update_latest"));
    } catch { toast.error(t("settings.toast.update_failed")); }
  };

  if (loading) return <PageSpinner />;

  const sections = [
    { key: "profile", label: t("settings.profile"), icon: <User className="w-4 h-4" /> },
    { key: "theme", label: t("settings.theme"), icon: <Palette className="w-4 h-4" /> },
    { key: "language", label: t("settings.language"), icon: <Globe className="w-4 h-4" /> },
    { key: "security", label: t("settings.security"), icon: <Lock className="w-4 h-4" /> },
    { key: "server", label: t("settings.server"), icon: <Server className="w-4 h-4" /> },
    { key: "agent", label: t("settings.agent"), icon: <Bot className="w-4 h-4" /> },
    { key: "malleable", label: t("settings.malleable"), icon: <Shield className="w-4 h-4" /> },
    { key: "database", label: t("settings.database"), icon: <Database className="w-4 h-4" /> },
    { key: "backup", label: t("settings.backup"), icon: <Archive className="w-4 h-4" /> },
    { key: "maintenance", label: t("settings.maintenance"), icon: <Wrench className="w-4 h-4" /> },
    { key: "notifications", label: t("settings.notifications"), icon: <Bell className="w-4 h-4" /> },
    { key: "siem", label: t("settings.siem"), icon: <LogIn className="w-4 h-4" /> },
    { key: "sync", label: t("settings.sync"), icon: <CloudUpload className="w-4 h-4" /> },
    { key: "extc2", label: t("settings.extc2"), icon: <Radio className="w-4 h-4" /> },
    { key: "about", label: t("settings.about"), icon: <Cpu className="w-4 h-4" /> },
  ];

  const inputCls = "w-full bg-background border border-border text-foreground focus-visible:border-ring rounded-xl px-4 py-3 text-sm";
  const textareaCls = "w-full bg-background border border-border text-foreground focus-visible:border-ring rounded-xl px-4 py-3 text-sm font-mono";

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("settings.title")} subtitle={t("settings.subtitle")} />

      <div className="flex flex-col lg:flex-row gap-4">
        <div className="w-48 shrink-0 hidden lg:block">
          <div className="sticky top-6 space-y-1">
            <div className="text-[10px] uppercase tracking-wider text-muted-foreground px-3 mb-2 font-semibold">{t("settings.sidebar_header")}</div>
            {sections.map((s) => (
              <Button key={s.key} variant="ghost" onClick={() => setActiveSection(s.key)}
                className={`block w-full text-left px-3 py-2 text-xs rounded-xl transition-colors ${activeSection === s.key ? "bg-primary/10 text-primary" : "hover:bg-primary/5 text-muted-foreground"}`}>
                {s.icon}{s.label}
              </Button>
            ))}
          </div>
        </div>

        <div className="flex-1 min-w-0">
          <nav className="lg:hidden mb-4 overflow-x-auto flex gap-2 pb-2">
            {sections.map((s) => (
              <Button key={s.key} variant="ghost" size="sm" onClick={() => setActiveSection(s.key)}
                className={`shrink-0 px-3 py-1.5 text-xs rounded-xl transition-colors ${activeSection === s.key ? "bg-primary/10 text-primary" : "bg-muted text-muted-foreground"}`}>
                {s.icon}{s.label}
              </Button>
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
                totpCode={totpCode} setTotpCode={setTotpCode} showTotpSetup={showTotpSetup}
                totpDisablePassword={totpDisablePassword} setTotpDisablePassword={setTotpDisablePassword}
                saving={saving} onChangePassword={handleChangePassword} onRegenerateJWT={handleRegenerateJWT}
                onGenerateTOTP={handleGenerateTOTP} onEnableTOTP={handleEnableTOTP} onDisableTOTP={handleDisableTOTP} inputCls={inputCls}
              />
            )}
            {activeSection === "server" && <ServerSection data={data} form={serverForm} setForm={setServerForm} saving={saving} inputCls={inputCls} onSave={handleSaveServer} />}
            {activeSection === "agent" && <AgentSection form={agentForm} setForm={setAgentForm} saving={saving} inputCls={inputCls} onSave={handleSaveAgent} />}
            {activeSection === "malleable" && <MalleableSection form={malleableForm} setForm={setMalleableForm} saving={saving} inputCls={inputCls} textareaCls={textareaCls} onSave={handleSaveMalleable} />}
            {activeSection === "database" && <DatabaseSection data={data} saving={saving} onVacuum={handleVacuum} onBackup={handleBackup} onDownloadDB={handleDownloadDB} />}
            {activeSection === "backup" && <BackupSection />}
            {activeSection === "maintenance" && <MaintenanceSection purgeDays={purgeDays} setPurgeDays={setPurgeDays} saving={saving} onPurge={handlePurge} onPurgeScreenshots={() => setCfmInline({msg: t("settings.confirm.purge_screenshots"), cb: () => handlePurge("screenshots") })} />}
            {activeSection === "notifications" && <NotificationsSection inputCls={inputCls} />}
            {activeSection === "siem" && <SIEMSection inputCls={inputCls} />}
            {activeSection === "sync" && <SyncSection inputCls={inputCls} />}
            {activeSection === "about" && <AboutSection data={data} onCheckUpdate={handleCheckUpdate} />}
            {activeSection === "extc2" && <ExtC2Section />}
          </div>
        </div>
      </div>

      <Dialog open={!!cfm} onOpenChange={(open) => { if (!open) setCfm(null); }}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{cfm?.msg || ""}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCfm(null)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={() => { cfm?.cb(); setCfm(null); }}>{t("common.confirm")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={!!cfmInline} onOpenChange={(open) => { if (!open) setCfmInline(null); }}>
        <DialogContent showCloseButton={false}>
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{cfmInline?.msg || ""}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCfmInline(null)}>{t("common.cancel")}</Button>
            <Button variant="destructive" onClick={() => { cfmInline?.cb(); setCfmInline(null); }}>{t("settings.btn.purge")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
