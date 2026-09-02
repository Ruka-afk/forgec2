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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { SearchInput } from "@/components/framework/SearchInput";
import { toast } from "sonner";
import { Bell, Bot, Cpu, Database, FileCode, Globe, Lock, Palette, Server, Shield, User, Users, Wrench, Archive, Radio, AlertTriangle, Activity, ScanSearch, SearchX } from "lucide-react";
import { useTOTP } from "./_components/useTOTP";
import { useSettingsData } from "./_components/useSettingsData";
import ProfileSection from "./_components/ProfileSection";
import ThemeSection from "./_components/ThemeSection";
import LanguageSection from "./_components/LanguageSection";
import SecuritySection from "./_components/SecuritySection";
import ApiKeysSection from "./_components/ApiKeysSection";
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
const SIEMRulesSection = dynamic(() => import("./_components/SIEMRulesSection"), { ssr: false });
const CertificatesSection = dynamic(() => import("./_components/CertificatesSection"), { ssr: false });
const ModulesSection = dynamic(() => import("./_components/ModulesSection"), { ssr: false });
const EmergencySection = dynamic(() => import("./_components/EmergencySection"), { ssr: false });
const AccessSection = dynamic(() => import("./_components/AccessSection"), { ssr: false });
const TelemetrySection = dynamic(() => import("./_components/TelemetrySection"), { ssr: false });

const SETTINGS_SECTION_KEYS = new Set([
  "profile", "theme", "language", "security", "access", "server", "agent",
  "malleable", "database", "backup", "maintenance", "notifications", "extc2",
  "siem", "certificates", "modules", "emergency", "telemetry", "about",
]);

function sectionFromHash(): string | null {
  const raw = window.location.hash.match(/^#tab=(.+)$/)?.[1];
  if (!raw) return null;
  try {
    const section = decodeURIComponent(raw);
    return SETTINGS_SECTION_KEYS.has(section) ? section : null;
  } catch {
    return null;
  }
}

export default function SettingsPage() {
  const router = useRouter();
  const { t, setLocale } = useI18n();
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
  const [sectionQuery, setSectionQuery] = useState("");
  const [purgeDays, setPurgeDays] = useState({ tasks: "30", audit: "30" });
  const [saving, setSaving] = useState(false);
  const { confirm: confirmAction, modal: modalAction } = useConfirm();
  const { confirm: confirmPurge, modal: modalPurge } = useConfirm();

  useEffect(() => {
    const syncFromHash = () => {
      const section = sectionFromHash();
      if (section) setActiveSection(section);
    };
    syncFromHash();
    window.addEventListener("hashchange", syncFromHash);
    return () => window.removeEventListener("hashchange", syncFromHash);
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

  // G8 fix: use the single i18n setLocale to keep localStorage+cookie in sync.
  // Previously only cookie was written here, causing language to revert on reload.
  const handleSetLanguage = (code: string) => {
    setLanguage(code);
    try { setLocale(code as "en" | "zh"); } catch { /* silent */ }
    setTimeout(() => { router.refresh(); }, 0);
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
    { key: "profile", label: t("settings.profile"), icon: <User className="size-4" /> },
    { key: "theme", label: t("settings.theme"), icon: <Palette className="size-4" /> },
    { key: "language", label: t("settings.language"), icon: <Globe className="size-4" /> },
    { key: "security", label: t("settings.security"), icon: <Lock className="size-4" /> },
    { key: "access", label: t("settings.access"), icon: <Users className="size-4" /> },
    { key: "server", label: t("settings.server"), icon: <Server className="size-4" /> },
    { key: "agent", label: t("settings.agent"), icon: <Bot className="size-4" /> },
    { key: "malleable", label: t("settings.malleable"), icon: <Shield className="size-4" /> },
    { key: "database", label: t("settings.database"), icon: <Database className="size-4" /> },
    { key: "backup", label: t("settings.backup"), icon: <Archive className="size-4" /> },
    { key: "maintenance", label: t("settings.maintenance"), icon: <Wrench className="size-4" /> },
    { key: "notifications", label: t("settings.notifications"), icon: <Bell className="size-4" /> },
    { key: "extc2", label: t("settings.extc2"), icon: <Radio className="size-4" /> },
    { key: "siem", label: t("settings.siem"), icon: <ScanSearch className="size-4" /> },
    { key: "certificates", label: t("settings.certificates.label"), icon: <Lock className="size-4" /> },
    { key: "modules", label: t("settings.modules.title"), icon: <FileCode className="size-4" /> },
    { key: "emergency", label: t("settings.emergency.title"), icon: <AlertTriangle className="size-4" /> },
    { key: "telemetry", label: t("settings.telemetry"), icon: <Activity className="size-4" /> },
    { key: "about", label: t("settings.about"), icon: <Cpu className="size-4" /> },
  ];

  const sectionGroups = [
    { key: "account", label: t("settings.group_account"), members: ["profile", "theme", "language", "security", "access"] },
    { key: "server", label: t("settings.group_server"), members: ["server", "database", "backup", "certificates"] },
    { key: "agent", label: t("settings.group_agent"), members: ["agent", "malleable", "modules"] },
    { key: "integrations", label: t("settings.group_integrations"), members: ["notifications", "extc2", "siem", "telemetry"] },
    { key: "maintenance", label: t("settings.group_maintenance"), members: ["maintenance", "emergency", "about"] },
  ].map((group) => ({
    ...group,
    items: sections.filter((section) => group.members.includes(section.key)),
  }));
  const normalizedSectionQuery = sectionQuery.trim().toLocaleLowerCase();
  const visibleSectionGroups = sectionGroups
    .map((group) => ({
      ...group,
      items: normalizedSectionQuery
        ? group.items.filter((section) => section.label.toLocaleLowerCase().includes(normalizedSectionQuery))
        : group.items,
    }))
    .filter((group) => group.items.length > 0);
  const activeSectionMeta = sections.find((section) => section.key === activeSection) ?? sections[0];
  const activeGroup = sectionGroups.find((group) => group.members.includes(activeSection)) ?? sectionGroups[0];
  const activeSectionIndex = sections.findIndex((section) => section.key === activeSection);
  const handleSectionChange = (section: string) => {
    if (!SETTINGS_SECTION_KEYS.has(section)) return;
    setActiveSection(section);
    window.history.replaceState(null, "", `${window.location.pathname}${window.location.search}#tab=${encodeURIComponent(section)}`);
  };

  return (
    <Permission perms="settings.read" fallback={
      <PageContainer title={t("settings.title")} subtitle={t("settings.subtitle")}>
        <ErrorState title={t("common.denied_title")} message={t("common.denied_desc")} />
      </PageContainer>
    }>
    <PageContainer variant="wide" title={t("settings.title")} subtitle={t("settings.subtitle")}>

      <Tabs value={activeSection} onValueChange={handleSectionChange} orientation="vertical">
        <div className="grid gap-4 lg:grid-cols-[16rem_minmax(0,1fr)] lg:items-start lg:gap-6">
          <aside className="hidden lg:block">
            <div className="sticky top-24 overflow-hidden rounded-xl border border-border/80 bg-card shadow-sm">
              <div className="border-b border-border/70 bg-muted/35 p-3">
                <div className="mb-2 flex items-center justify-between px-0.5">
                  <span className="text-xs font-semibold text-foreground">{t("settings.sidebar_header")}</span>
                  <span className="rounded-full bg-primary/10 px-2 py-0.5 font-mono text-[10px] font-semibold text-primary">{sections.length}</span>
                </div>
              <SearchInput
                value={sectionQuery}
                onChange={setSectionQuery}
                onClear={() => setSectionQuery("")}
                placeholder={t("settings.search_sections")}
                label={t("settings.search_sections")}
              />
              </div>
              <nav aria-label={t("settings.sidebar_header")} className="max-h-[calc(100vh-13rem)] space-y-3 overflow-y-auto p-2.5 supports-[height:100dvh]:max-h-[calc(100dvh-13rem)]">
                {visibleSectionGroups.map((group) => (
                  <div key={group.key}>
                    <div className="mono-eyebrow mb-1 px-2 text-muted-foreground">{group.label}</div>
                    <TabsList variant="sidebar">
                      {group.items.map((s) => (
                        <TabsTrigger key={s.key} value={s.key}
                          className="relative flex min-h-10 w-full items-center justify-start gap-2.5 rounded-lg border-l-2 border-transparent px-2.5 py-2 text-xs text-muted-foreground transition-colors hover:bg-primary/5 data-[selected]:border-l-primary data-[selected]:bg-primary/10 data-[selected]:font-semibold data-[selected]:text-primary data-[selected]:shadow-none data-[selected]:[&>span:first-child]:bg-primary/10 data-[selected]:[&>span:first-child]:text-primary">
                          <span className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted/75 text-muted-foreground">{s.icon}</span>
                          <span className="truncate">{s.label}</span>
                        </TabsTrigger>
                      ))}
                    </TabsList>
                  </div>
                ))}
                {visibleSectionGroups.length === 0 && (
                  <div className="flex flex-col items-center px-3 py-8 text-center">
                    <SearchX className="mb-2 size-5 text-muted-foreground" />
                    <p className="text-xs text-muted-foreground">{t("settings.no_sections")}</p>
                    <button type="button" onClick={() => setSectionQuery("")} className="mt-2 text-xs font-medium text-primary hover:underline">{t("common.clear")}</button>
                  </div>
                )}
              </nav>
            </div>
          </aside>

          <div className="min-w-0">
            <div className="sticky top-[calc(var(--shell-breadcrumb-height)+0.5rem)] z-20 mb-4 rounded-xl border border-border/80 bg-background/95 p-2 shadow-sm backdrop-blur lg:hidden">
              <Select value={activeSection} onValueChange={(value) => { if (typeof value === "string") handleSectionChange(value); }}>
                <SelectTrigger className="min-h-11 w-full bg-card" aria-label={t("settings.sidebar_header")}>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {sectionGroups.map((group) => group.items.map((section) => (
                    <SelectItem key={section.key} value={section.key}>{group.label} · {section.label}</SelectItem>
                  )))}
                </SelectContent>
              </Select>
            </div>

            <div className="mb-4 flex items-center gap-3 rounded-xl border border-border/75 bg-card px-4 py-3 shadow-xs">
              <span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">{activeSectionMeta.icon}</span>
              <div className="min-w-0 flex-1">
                <div className="mono-eyebrow text-muted-foreground">{activeGroup.label}</div>
                <h2 className="truncate text-base font-semibold text-foreground">{activeSectionMeta.label}</h2>
              </div>
              <span className="shrink-0 font-mono text-[10px] text-muted-foreground">{activeSectionIndex + 1}/{sections.length}</span>
            </div>

            <div className="mx-auto max-w-5xl space-y-6">
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
                <div className="mt-4"><ApiKeysSection /></div>
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
              <TabsContent value="siem" className="mt-0"><SIEMRulesSection /></TabsContent>
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
