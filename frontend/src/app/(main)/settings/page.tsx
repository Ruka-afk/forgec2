"use client";

import { useState, useCallback, useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { PageContainer } from "@/components/ui/page-container";
import { ErrorState } from "@/components/ui/error-state";
import { Permission } from "@/components/ui/permission";
import { PageSpinner } from "@/components/ui/spinner";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { toast } from "sonner";
import { Bell, Bot, Cpu, Database, FileCode, Globe, Lock, Palette, Server, Shield, User, Users, Wrench, Archive, Radio, AlertTriangle, Activity } from "lucide-react";
import { useTOTP } from "./_components/useTOTP";
import { useSettingsData } from "./_components/useSettingsData";
import ProfileSection from "./_components/ProfileSection";
import ThemeSection from "./_components/ThemeSection";
import LanguageSection from "./_components/LanguageSection";
import SecuritySection from "./_components/SecuritySection";
import ServerSection from "./_components/ServerSection";
import AgentSection from "./_components/AgentSection";
import dynamic from "next/dynamic";

const MalleableSection = dynamic(() => import("./_components/MalleableSection"), { ssr: false });
const DatabaseSection = dynamic(() => import("./_components/DatabaseSection"), { ssr: false });
const MaintenanceSection = dynamic(() => import("./_components/MaintenanceSection"), { ssr: false });
const AboutSection = dynamic(() => import("./_components/AboutSection"), { ssr: false });
const NotificationsSection = dynamic(() => import("./_components/NotificationsSection"), { ssr: false });
const BackupSection = dynamic(() => import("./_components/BackupSection"), { ssr: false });
const ExtC2Section = dynamic(() => import("./_components/ExtC2Section"), { ssr: false });
const CertificatesSection = dynamic(() => import("./_components/CertificatesSection"), { ssr: false });
const ModulesSection = dynamic(() => import("./_components/ModulesSection"), { ssr: false });
const EmergencySection = dynamic(() => import("./_components/EmergencySection"), { ssr: false });
const AccessSection = dynamic(() => import("./_components/AccessSection"), { ssr: false });
const TelemetrySection = dynamic(() => import("./_components/TelemetrySection"), { ssr: false });

export default function SettingsPage() {
  const router = useRouter();
  const { t } = useI18n();
  const {
    data,
    loading,
    loadSettings,
    agentForm,
    setAgentForm,
    serverForm,
    setServerForm,
    malleableForm,
    setMalleableForm,
    passwordForm,
    setPasswordForm,
    theme,
    setTheme,
    language,
    setLanguage,
  } = useSettingsData();
  const [activeSection, setActiveSection] = useState("profile");
  const [purgeDays, setPurgeDays] = useState({ tasks: "30", audit: "30" });
  const [saving, setSaving] = useState(false);
  const { confirm: confirmAction, modal: modalAction } = useConfirm();
  const { confirm: confirmPurge, modal: modalPurge } = useConfirm();

  useEffect(() => {
    const tab = window.location.hash.match(/^#tab=(.+)$/)?.[1];
    if (tab) setActiveSection(tab);
  }, []);

  const {
    totpStatus, totpSecret, totpQR, totpBackupCodes,
    totpCode, setTotpCode, showTotpSetup,
    totpEnablePassword, setTotpEnablePassword,
    totpDisablePassword, setTotpDisablePassword, totpDisableCode, setTotpDisableCode,
    handleGenerateTOTP, handleEnableTOTP, handleDisableTOTP,
  } = useTOTP(t, setSaving, activeSection);

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
    await api.post(paths.settings.agent, { interval: String(agentForm.interval), jitter: String(agentForm.jitter), skip_tls: String(agentForm.skip_tls), user_agent: agentForm.user_agent, working_start: agentForm.working_start, working_end: agentForm.working_end, working_tz: agentForm.working_tz });
    toast.success(t("settings.toast.agent_saved"));
  });

  const handleSaveServer = withSaveTimeout(async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post(paths.settings.server, {
      log_level: serverForm.log_level, tcp_enabled: String(serverForm.tcp_enabled), tcp_addr: serverForm.tcp_addr,
      offline_threshold: String(serverForm.offline_threshold), session_max_age: String(serverForm.session_max_age), cleanup_retention: String(serverForm.cleanup_retention),
    });
    toast.success(t("settings.toast.server_saved"));
  });

  const handleSaveMalleable = withSaveTimeout(async (e: React.FormEvent) => {
    e.preventDefault();
    await api.post(paths.settings.malleable, {
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
      await api.post(paths.settings.password, { current_password: passwordForm.current, new_password: passwordForm.next, confirm_password: passwordForm.confirm });
      toast.success(t("settings.toast.password_changed"));
      setPasswordForm({ current: "", next: "", confirm: "" });
    } catch { toast.error(t("settings.toast.password_change_failed")); }
    finally { setSaving(false); }
  };

  const handleRegenerateJWT = async () => {
    if (!(await confirmAction({ message: t("settings.confirm.jwt") }))) return;
    setSaving(true);
    try { await api.post(paths.settings.jwtRegenerate); toast.success(t("settings.toast.jwt_regenerated")); loadSettings(); }
    catch { toast.error(t("settings.toast.jwt_failed")); }
    finally { setSaving(false); }
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
    try { await api.post(paths.settings.dbVacuum); toast.success(t("settings.toast.vacuum_done")); }
    catch { toast.error(t("settings.toast.vacuum_failed")); }
    finally { setSaving(false); }
  };

  const handleBackup = async () => {
    setSaving(true);
    try {
      const { blob } = await api.download(paths.settings.dbBackup);
      downloadBlob(blob, `forgec2_backup_${Date.now()}.db`);
      toast.success(t("settings.toast.backup_done"));
    } catch { toast.error(t("settings.toast.backup_failed")); }
    finally { setSaving(false); }
  };

  const handleDownloadDB = async () => {
    try {
      const { blob } = await api.downloadGet(paths.settings.configDownload);
      downloadBlob(blob, "forgec2.db");
    } catch { toast.error(t("settings.toast.download_failed")); }
  };

  const handlePurge = async (type: string) => {
    const days = type === "tasks" ? purgeDays.tasks : purgeDays.audit;
    if (!(await confirmAction({ message: t("settings.confirm.purge", { type, days }) }))) return;
    setSaving(true);
    try {
      const path = type === "screenshots"
        ? paths.settings.maintenancePurge
        : paths.settings.purge(type);
      await api.post(path, { days });
      toast.success(t("settings.toast.purge_done", { type }));
    } catch { toast.error(t("settings.toast.purge_failed", { type })); }
    finally { setSaving(false); }
  };

  const handleCheckUpdate = async () => {
    try {
      const d = await api.get<{ update_available?: boolean; latest_version?: string }>(paths.updateCheck);
      if (d.update_available && d.latest_version) {
        toast.success(t("settings.toast.update_new", { version: String(d.latest_version) }));
      } else {
        toast.success(t("settings.toast.update_latest"));
      }
    } catch { toast.error(t("settings.toast.update_failed")); }
  };

  if (loading) return <PageContainer title={t("settings.title")} subtitle={t("settings.subtitle")}><PageSpinner /></PageContainer>;

  const sections = [
    { key: "profile", label: t("settings.profile"), icon: <User className="w-4 h-4" /> },
    { key: "theme", label: t("settings.theme"), icon: <Palette className="w-4 h-4" /> },
    { key: "language", label: t("settings.language"), icon: <Globe className="w-4 h-4" /> },
    { key: "security", label: t("settings.security"), icon: <Lock className="w-4 h-4" /> },
    { key: "access", label: t("settings.access"), icon: <Users className="w-4 h-4" /> },
    { key: "server", label: t("settings.server"), icon: <Server className="w-4 h-4" /> },
    { key: "agent", label: t("settings.agent"), icon: <Bot className="w-4 h-4" /> },
    { key: "malleable", label: t("settings.malleable"), icon: <Shield className="w-4 h-4" /> },
    { key: "database", label: t("settings.database"), icon: <Database className="w-4 h-4" /> },
    { key: "backup", label: t("settings.backup"), icon: <Archive className="w-4 h-4" /> },
    { key: "maintenance", label: t("settings.maintenance"), icon: <Wrench className="w-4 h-4" /> },
    { key: "notifications", label: t("settings.notifications"), icon: <Bell className="w-4 h-4" /> },
    { key: "extc2", label: t("settings.extc2"), icon: <Radio className="w-4 h-4" /> },
    { key: "certificates", label: t("settings.certificates.label"), icon: <Lock className="w-4 h-4" /> },
    { key: "modules", label: t("settings.modules.title"), icon: <FileCode className="w-4 h-4" /> },
    { key: "emergency", label: t("settings.emergency.title"), icon: <AlertTriangle className="w-4 h-4" /> },
    { key: "telemetry", label: t("settings.telemetry"), icon: <Activity className="w-4 h-4" /> },
    { key: "about", label: t("settings.about"), icon: <Cpu className="w-4 h-4" /> },
  ];

  return (
    <Permission perms="settings.read" fallback={
      <PageContainer title={t("settings.title")} subtitle={t("settings.subtitle")}>
        <ErrorState title={t("common.denied_title")} message={t("common.denied_desc")} />
      </PageContainer>
    }>
    <PageContainer title={t("settings.title")} subtitle={t("settings.subtitle")}>

      <Tabs value={activeSection} onValueChange={setActiveSection}>
        <div className="flex flex-col lg:flex-row gap-4">
          <div className="w-48 shrink-0 hidden lg:block">
            <div className="sticky lg:top-[96px] space-y-1">
              <div className="text-(--fs-micro-sm) uppercase tracking-wider text-muted-foreground px-3 mb-2 font-semibold">{t("settings.sidebar_header")}</div>
              <TabsList className="flex-col bg-transparent p-0 gap-1 w-full h-auto">
                {sections.map((s) => (
                  <TabsTrigger key={s.key} value={s.key}
                    className="flex items-center gap-2 w-full justify-start px-3 py-2 text-xs rounded-lg transition-colors data-[selected]:bg-primary/10 data-[selected]:text-primary hover:bg-primary/5 text-muted-foreground">
                    {s.icon}{s.label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>
          </div>

          <div className="flex-1 min-w-0">
            <TabsList className="lg:hidden mb-4 overflow-x-auto flex gap-2 pb-2 bg-transparent p-0 h-auto">
              {sections.map((s) => (
                <TabsTrigger key={s.key} value={s.key}
                  className="shrink-0 px-3 py-1.5 text-xs rounded-lg transition-colors data-[selected]:bg-primary/10 data-[selected]:text-primary bg-muted text-muted-foreground">
                  {s.icon}{s.label}
                </TabsTrigger>
              ))}
            </TabsList>

            <div className="space-y-6">
              <TabsContent value="profile" className="mt-0"><ProfileSection data={data} /></TabsContent>
              <TabsContent value="theme" className="mt-0"><ThemeSection theme={theme} onApplyTheme={handleApplyTheme} /></TabsContent>
              <TabsContent value="language" className="mt-0"><LanguageSection language={language} onSetLanguage={handleSetLanguage} /></TabsContent>
              <TabsContent value="access" className="mt-0"><AccessSection /></TabsContent>
              <TabsContent value="security" className="mt-0">
                <SecuritySection
                  data={data} passwordForm={passwordForm} setPasswordForm={setPasswordForm}
                  totpStatus={totpStatus} totpSecret={totpSecret} totpQR={totpQR} totpBackupCodes={totpBackupCodes}
                  totpCode={totpCode} setTotpCode={setTotpCode} showTotpSetup={showTotpSetup}
                  totpEnablePassword={totpEnablePassword} setTotpEnablePassword={setTotpEnablePassword}
                  totpDisablePassword={totpDisablePassword} setTotpDisablePassword={setTotpDisablePassword}
                  totpDisableCode={totpDisableCode} setTotpDisableCode={setTotpDisableCode}
                  saving={saving} onChangePassword={handleChangePassword} onRegenerateJWT={handleRegenerateJWT}
                  onGenerateTOTP={handleGenerateTOTP} onEnableTOTP={handleEnableTOTP} onDisableTOTP={handleDisableTOTP}
                />
              </TabsContent>
              <TabsContent value="server" className="mt-0"><ServerSection data={data} form={serverForm} setForm={setServerForm} saving={saving} onSave={handleSaveServer} /></TabsContent>
              <TabsContent value="agent" className="mt-0"><AgentSection form={agentForm} setForm={setAgentForm} saving={saving} onSave={handleSaveAgent} /></TabsContent>
              <TabsContent value="malleable" className="mt-0"><MalleableSection form={malleableForm} setForm={setMalleableForm} saving={saving} onSave={handleSaveMalleable} /></TabsContent>
              <TabsContent value="database" className="mt-0"><DatabaseSection data={data} saving={saving} onVacuum={handleVacuum} onBackup={handleBackup} onDownloadDB={handleDownloadDB} /></TabsContent>
              <TabsContent value="backup" className="mt-0"><BackupSection /></TabsContent>
              <TabsContent value="maintenance" className="mt-0"><MaintenanceSection purgeDays={purgeDays} setPurgeDays={setPurgeDays} saving={saving} onPurge={handlePurge} onPurgeScreenshots={async () => { if (await confirmPurge({ message: t("settings.confirm.purge_screenshots"), confirmText: t("settings.btn.purge") })) handlePurge("screenshots"); }} /></TabsContent>
              <TabsContent value="notifications" className="mt-0"><NotificationsSection /></TabsContent>
              <TabsContent value="about" className="mt-0"><AboutSection data={data} onCheckUpdate={handleCheckUpdate} /></TabsContent>
              <TabsContent value="extc2" className="mt-0"><ExtC2Section /></TabsContent>
              <TabsContent value="certificates" className="mt-0"><CertificatesSection data={data} saving={saving} onRefresh={loadSettings} /></TabsContent>
              <TabsContent value="modules" className="mt-0"><ModulesSection /></TabsContent>
              <TabsContent value="emergency" className="mt-0"><EmergencySection /></TabsContent>
              <TabsContent value="telemetry" className="mt-0"><TelemetrySection /></TabsContent>
            </div>
          </div>
        </div>
      </Tabs>

      {modalAction}
      {modalPurge}
    </PageContainer>
    </Permission>
  );
}
