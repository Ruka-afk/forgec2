"use client";

import { useEffect } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Checkbox } from "@/components/ui/checkbox";
import { Award, CheckCircle, CircleAlert, Code, Copy, Download, FileUp, FileDown, Globe, Info, Mail, Pencil, Plug, Plus, Rocket, Save, Server, Share2, ShieldCheck, Terminal, Trash2, WandSparkles } from "lucide-react";
import { DataState } from "@/components/ui/data-state";
import { useInfrastructureData } from "./_components/useInfrastructureData";
import { useInfrastructureDialogs } from "./_components/useInfrastructureDialogs";
import { useInfrastructureConfigForm } from "./_components/useInfrastructureConfigForm";
import { useInfrastructureRedirectorForm } from "./_components/useInfrastructureRedirectorForm";

export default function InfrastructurePage() {
  const { t } = useI18n();
  const { listeners, redirectors, loading, error, loadListeners, loadRedirectors } = useInfrastructureData();
  const { showGenModal, setShowGenModal, confirm, modal, editingRd, setEditingRd } = useInfrastructureDialogs();
  const {
    selectedListener, setSelectedListener, domain, setDomain, port, setPort,
    certPath, setCertPath, keyPath, setKeyPath, wsSupport, setWsSupport,
    extC2Path, setExtC2Path, configOutput, configType,
    generating, copied, activeSection, setActiveSection,
    acmeDomain, setAcmeDomain, acmeEmail, setAcmeEmail, acmePort, setAcmePort,
    acmeStaging, setAcmeStaging, acmeProvisioning, exportFormat, setExportFormat,
    exporting, setExporting, generateConfig, copyConfig, downloadConfig, provisionCert, exportProfile,
  } = useInfrastructureConfigForm(listeners);
  const {
    rdName, setRdName, rdHost, setRdHost, rdType, setRdType,
    rdSSHUser, setRdSSHUser, rdSSHPort, setRdSSHPort,
    rdSSHKey, setRdSSHKey, rdSSHPassword, setRdSSHPassword,
    rdConfig, setRdConfig, rdGenerateHost, setRdGenerateHost,
    rdGenerateDomain, setRdGenerateDomain, rdGeneratePort, setRdGeneratePort,
    rdGenerateTLS, setRdGenerateTLS, rdGenerateWS, setRdGenerateWS,
    deploying, setDeploying, saving, setSaving, deleting, setDeleting,
    testingSSH, setTestingSSH, generatingModal, setGeneratingModal,
    deployResult, setDeployResult, backendURL, setBackendURL,
  } = useInfrastructureRedirectorForm();

  useEffect(() => { loadListeners(); }, [loadListeners]);

  useEffect(() => {
    if (activeSection === "redirectors") {
      loadRedirectors();
    }
  }, [activeSection, loadRedirectors]);

  const currentListener = listeners.find(l => l.id === selectedListener);

  const sections = [
    { key: "config" as const, label: t("infra.reverse_proxy_config"), icon: <Code className="w-4 h-4" /> },
    { key: "redirectors" as const, label: t("infra.redirectors"), icon: <Share2 className="w-4 h-4" /> },
    { key: "acme" as const, label: t("infra.acme_cert"), icon: <ShieldCheck className="w-4 h-4" /> },
    { key: "export" as const, label: t("infra.c2_profile_tab"), icon: <FileDown className="w-4 h-4" /> },
  ];

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Server className="w-4 h-4" />{t("infra.title")}</>} subtitle={t("infra.subtitle")} />

      <DataState loading={loading} error={error} onRetry={() => { loadListeners(); if (activeSection === "redirectors") loadRedirectors(); }}>
      <Tabs value={activeSection} onValueChange={setActiveSection}>
        <TabsList className="flex w-full justify-start rounded-none border-b border-border bg-transparent p-0 h-auto">
          {sections.map(s => (
            <TabsTrigger key={s.key} value={s.key}
              className="flex items-center gap-x-2 px-5 py-3 text-sm font-medium rounded-none border-b-2 border-transparent data-[selected]:border-primary data-[selected]:text-primary text-muted-foreground hover:text-foreground transition-colors">
              {s.icon}
              <span>{s.label}</span>
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="config" className="mt-0">
          <Card className="overflow-hidden">
            <CardHeader className="px-6 py-4 border-b">
              <div className="w-8 h-8 bg-primary/10 dark:bg-primary/20 rounded-xl flex items-center justify-center text-primary"><Plug className="w-4 h-4" /></div>
              <div><CardTitle>{t("infra.select")} Listener</CardTitle><CardDescription>{t("infra.select_listener_hint")}</CardDescription></div>
            </CardHeader>
            <CardContent className="p-4 sm:p-5">
              <Select value={selectedListener} onValueChange={(v) => setSelectedListener(v ?? "")}>
                <SelectTrigger className="max-w-md">
                  <SelectValue placeholder={t("infra.select_listener_placeholder")} />
                </SelectTrigger>
                <SelectContent>
                  {listeners.map(l => (
                    <SelectItem key={l.id} value={l.id}>{l.name} ({l.protocol}://{l.host}:{l.port})</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {currentListener && (
                <div className="mt-3 text-xs text-muted-foreground">
                  <Info className="w-4 h-4" />
                  {t("infra.forwarding_to")} {currentListener.protocol}://{currentListener.host}:{currentListener.port}
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="overflow-hidden">
            <CardHeader className="px-6 py-4 border-b">
              <div className="w-8 h-8 bg-emerald-100 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center text-emerald-600 dark:text-emerald-400"><Globe className="w-4 h-4" /></div>
              <div><CardTitle>{t("infra.domain_params")}</CardTitle><CardDescription>{t("infra.domain_desc")}</CardDescription></div>
            </CardHeader>
            <CardContent className="p-4 sm:p-5 space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs">{t("infra.domain")}</Label>
                  <Input aria-label="c2.example.com" name="input-1" type="text" value={domain} onChange={e => setDomain(e.target.value)} placeholder="c2.example.com" />
                </div>
                <div>
                  <Label className="text-xs">{t("infra.listen_port")}</Label>
                  <Input aria-label={t("infra.listen_port")} name="input-2" type="number" value={port} onChange={e => setPort(Number(e.target.value))} />
                </div>
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <Label className="text-xs">{t("infra.ssl_cert_path")}</Label>
                  <Input aria-label="/etc/letsencrypt/live/c2.example.com/fullchain.pem" name="input-3" type="text" value={certPath} onChange={e => setCertPath(e.target.value)} placeholder="/etc/letsencrypt/live/c2.example.com/fullchain.pem" className="font-mono text-xs" />
                </div>
                <div>
                  <Label className="text-xs">{t("infra.ssl_key_path")}</Label>
                  <Input aria-label="/etc/letsencrypt/live/c2.example.com/privkey.pem" name="input-4" type="text" value={keyPath} onChange={e => setKeyPath(e.target.value)} placeholder="/etc/letsencrypt/live/c2.example.com/privkey.pem" className="font-mono text-xs" />
                </div>
              </div>
              <div className="flex items-center gap-4">
                <Label className="flex items-center gap-2 cursor-pointer">
                  <Checkbox checked={wsSupport} onCheckedChange={(v) => setWsSupport(!!v)} />
                  <span className="text-xs text-muted-foreground">{t("infra.ws_support")}</span>
                </Label>
                <Label className="flex items-center gap-2 cursor-pointer">
                  <Input value={extC2Path} onChange={(e) => setExtC2Path(e.target.value)} placeholder={t("infra.ext_c2_placeholder")} className="text-xs" />
                  <span className="text-xs text-muted-foreground">{t("infra.ext_c2_path")}</span>
                </Label>
              </div>
            </CardContent>
          </Card>

          <Card className="overflow-hidden">
            <CardHeader className="px-6 py-4 border-b">
              <div className="w-8 h-8 bg-amber-100 dark:bg-amber-900/30 rounded-xl flex items-center justify-center text-amber-600 dark:text-amber-400"><FileUp className="w-4 h-4" /></div>
              <div><CardTitle>{t("infra.generate_config")}</CardTitle><CardDescription>{t("infra.generate_desc")}</CardDescription></div>
            </CardHeader>
            <CardContent className="p-4 sm:p-5">
              <div className="flex flex-wrap gap-2 mb-4">
                <Button onClick={() => generateConfig("nginx")} className="bg-emerald-600 hover:bg-emerald-700 text-white">
                  <Server className="w-4 h-4" /> Nginx
                </Button>
                <Button onClick={() => generateConfig("apache")} className="bg-amber-500 hover:bg-amber-600 text-white">
                  <Server className="w-4 h-4" /> Apache
                </Button>
                <Button onClick={() => generateConfig("haproxy")} className="bg-destructive hover:bg-destructive/90 text-destructive-foreground">
                  <Server className="w-4 h-4" /> HAProxy
                </Button>
              </div>
              {generating && <div className="text-muted-foreground text-sm"><Spinner size="sm" /> {t("infra.generating")}</div>}
              {configOutput && !generating && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs font-medium text-muted-foreground">{t("infra.generated_config")} ({configType.toUpperCase()})</span>
                    <div className="flex gap-2">
                      <Button onClick={copyConfig} variant="outline" size="sm">
                        <Copy className="w-4 h-4" /> {copied ? t("infra.copied") : t("common.copy")}
                      </Button>
                      <Button onClick={downloadConfig} variant="outline" size="sm">
                        <Download className="w-4 h-4" /> {t("common.download")}
                      </Button>
                    </div>
                  </div>
                  <pre className="bg-card text-emerald-400 p-4 rounded-2xl text-xs overflow-auto max-h-[400px] whitespace-pre-wrap font-mono leading-relaxed select-all border border-border">{configOutput}</pre>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="redirectors" className="mt-0">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            <div className="lg:col-span-2">
              <Card className="overflow-hidden">
                <CardHeader className="px-6 py-4 border-b">
                  <div className="w-8 h-8 bg-primary/10 dark:bg-primary/20 rounded-xl flex items-center justify-center text-primary"><Share2 className="w-4 h-4" /></div>
                  <div><CardTitle>{t("infra.redirectors")}</CardTitle><CardDescription>{t("infra.redirectors_desc")}</CardDescription></div>
                  <Button onClick={() => { setEditingRd(null); setRdName(""); setRdHost(""); setRdType("nginx"); setRdSSHUser("root"); setRdSSHPort(22); setRdSSHKey(""); setRdSSHPassword(""); setRdConfig(""); }} className="ml-auto">
                    <Plus className="w-4 h-4" /> {t("infra.add_redirector")}
                  </Button>
                </CardHeader>
                <CardContent className="p-4 sm:p-5">
                  {redirectors.length === 0 ? (
                    <div className="text-center py-12 text-muted-foreground">
                      <EmptyState icon={Share2} title={t("infra.no_redirectors")} message={t("infra.no_redirectors_hint")} />
                    </div>
                  ) : (
                    <div className="space-y-3">
                      {redirectors.map(rd => (
                        <div key={rd.id} className="flex items-center justify-between p-4">
                          <Card className="bg-muted w-full">
                            <CardContent>
                              <div className="flex items-center justify-between">
                                <div className="flex items-center gap-4">
                                  <div className={`w-3 h-3 rounded-full ${rd.status === "active" ? "bg-emerald-500 animate-pulse" : rd.status === "error" ? "bg-red-500" : "bg-muted-foreground"}`}></div>
                                  <div>
                                    <div className="text-sm font-medium text-foreground">{rd.name}</div>
                                    <div className="text-xs text-muted-foreground mt-0.5">
                                      {rd.host} &middot; {rd.type.toUpperCase()} &middot; SSH: {rd.ssh_user}@{rd.host}:{rd.ssh_port}
                                    </div>
                                  </div>
                                </div>
                                <div className="flex items-center gap-2">
                                  <Badge variant={rd.status === "active" ? "success" : rd.status === "error" ? "destructive" : "secondary"} className="text-xs px-2 py-0.5 rounded-full font-medium">{rd.status}</Badge>
                                  <Button onClick={() => {
                                    setEditingRd(rd.id); setRdName(rd.name); setRdHost(rd.host); setRdType(rd.type);
                                    setRdSSHUser(rd.ssh_user || "root"); setRdSSHPort(rd.ssh_port || 22);
                                    setRdConfig(rd.config || "");
                                  }} variant="outline" size="sm">
                                    <Pencil className="w-4 h-4" />{t("common.edit")}
                                  </Button>
                                  <Button onClick={async () => {
                                    if (!(await confirm({ message: t("infra.delete_redirector_confirm") }))) return;
                                    setDeleting(rd.id);
                                    try { await api.del(paths.redirectors.one(rd.id)); loadRedirectors(); } catch { toast.error(t("infrastructure.toast.delete_redirector_failed")); }
                                    setDeleting(null);
                                  }} variant="destructive" size="sm">
                                    {deleting === rd.id ? <Spinner size="sm" className="mr-1" /> : <Trash2 className="w-4 h-4 mr-1" />}{deleting === rd.id ? t("infra.deleting") : t("common.delete")}
                                  </Button>
                                  <Button onClick={() => { setRdGenerateHost(rd.host); setDeployResult(null); setShowGenModal(true); }} size="sm">
                                    <Rocket className="w-4 h-4" />{t("infra.deploy")}
                                  </Button>
                                </div>
                              </div>
                            </CardContent>
                          </Card>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            <div>
              <Card className="overflow-hidden">
                <CardHeader className="px-6 py-4 border-b">
                  <div className="w-8 h-8 bg-emerald-100 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center text-emerald-600 dark:text-emerald-400"><Server className="w-4 h-4" /></div>
                  <div><CardTitle>{editingRd ? t("infra.edit_redirector") : t("infra.new_redirector")}</CardTitle></div>
                </CardHeader>
                <CardContent className="p-4 sm:p-5 space-y-4">
                  <div>
                    <Label className="text-xs">{t("common.name")}</Label>
                    <Input aria-label={t("infra.name_label")} name="input-7" type="text" value={rdName} onChange={e => setRdName(e.target.value)} placeholder={t("infra.name_label")} />
                  </div>
                  <div>
                    <Label className="text-xs">{t("infra.host")}</Label>
                    <Input aria-label="192.168.1.100" name="input-8" type="text" value={rdHost} onChange={e => setRdHost(e.target.value)} placeholder="192.168.1.100" />
                  </div>
                  <div>
                    <Label className="text-xs">{t("common.type")}</Label>
                    <Select value={rdType} onValueChange={(v) => { if (v) setRdType(v); }}>
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="nginx">Nginx</SelectItem>
                        <SelectItem value="apache">Apache</SelectItem>
                        <SelectItem value="haproxy">HAProxy</SelectItem>
                        <SelectItem value="caddy">Caddy</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <Label className="text-xs">{t("infra.ssh_user")}</Label>
                      <Input aria-label={t("infra.ssh_user")} name="input-10" type="text" value={rdSSHUser} onChange={e => setRdSSHUser(e.target.value)} placeholder="root" />
                    </div>
                    <div>
                      <Label className="text-xs">{t("infra.ssh_port")}</Label>
                      <Input aria-label={t("infra.ssh_port")} name="input-11" type="number" value={rdSSHPort} onChange={e => setRdSSHPort(Number(e.target.value))} />
                    </div>
                  </div>
                  <div>
                    <Label className="text-xs">{t("infra.ssh_key")}</Label>
                    <Textarea aria-label={t("infra.ssh_key")} name="textarea-12" value={rdSSHKey} onChange={e => setRdSSHKey(e.target.value)} placeholder={"-----BEGIN OPENSSH PRIVATE KEY-----\n..."} rows={3} className="font-mono text-xs" />
                  </div>
                  <div>
                    <Label className="text-xs">{t("infra.ssh_password")}</Label>
                    <Input aria-label={t("infra.ssh_password")} name="input-13" type="password" value={rdSSHPassword} onChange={e => setRdSSHPassword(e.target.value)} placeholder="password" />
                  </div>
                  <div className="flex gap-2">
                    <Button onClick={async () => {
                      setSaving(true);
                      try {
                        const payload = editingRd
                          ? { name: rdName, host: rdHost, type: rdType, ssh_user: rdSSHUser, ssh_port: rdSSHPort, ssh_key: rdSSHKey, ssh_password: rdSSHPassword, config: rdConfig }
                          : { name: rdName, host: rdHost, type: rdType, ssh_user: rdSSHUser, ssh_port: rdSSHPort, ssh_key: rdSSHKey, ssh_password: rdSSHPassword };
                        if (editingRd) {
                          await api.putJson(paths.redirectors.one(editingRd), payload);
                        } else {
                          await api.postJson(paths.redirectors.list, payload);
                        }
                        loadRedirectors();
                        setEditingRd(null);
                      } catch { toast.error(t("infrastructure.toast.save_redirector_failed")); }
                      setSaving(false);
                    }} disabled={saving} className="flex-1">
                      <Save className="w-4 h-4" /> {saving ? t("infra.saving") : editingRd ? t("infra.update") : t("common.save")}
                    </Button>
                    <Button onClick={async () => {
                      setTestingSSH(true);
                      try {
                        const data = await api.postJson(paths.redirectors.testSsh, { host: rdHost, port: rdSSHPort, user: rdSSHUser, password: rdSSHPassword, ssh_key: rdSSHKey });
                        if (data.success) { toast.success(t("infrastructure.toast.ssh_ok", { stdout: String(data.stdout) })); } else { toast.error(t("infrastructure.toast.ssh_failed", { message: String(data.message) })); }
                      } catch { toast.error(t("infrastructure.toast.ssh_test_failed")); }
                      setTestingSSH(false);
                    }} disabled={testingSSH} variant="outline" aria-label={t("infra.test_ssh")}>
                      {testingSSH ? <Spinner size="sm" /> : <Plug className="w-4 h-4" />}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>

          <Dialog open={showGenModal} onOpenChange={setShowGenModal}>
            <DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto p-6">
              <DialogHeader>
                <DialogTitle>{t("infra.deploy_redirector_title")}</DialogTitle>
              </DialogHeader>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <Label className="text-xs">{t("infra.domain")}</Label>
                  <Input aria-label="c2.example.com" name="input-14" type="text" value={rdGenerateDomain} onChange={e => setRdGenerateDomain(e.target.value)} placeholder="c2.example.com" />
                </div>
                <div>
                  <Label className="text-xs">{t("infra.backend_url")}</Label>
                  <Input aria-label="https://127.0.0.1:8080" name="input-15" type="text" value={backendURL} onChange={e => setBackendURL(e.target.value)} placeholder="https://127.0.0.1:8080" />
                </div>
                <div>
                  <Label className="text-xs">{t("infra.listen_port")}</Label>
                  <Input aria-label={t("infra.redirector_listen_port")} name="input-16" type="number" value={rdGeneratePort} onChange={e => setRdGeneratePort(Number(e.target.value))} />
                </div>
                <div className="flex items-center gap-4 pt-6">
                  <Label className="flex items-center gap-2 cursor-pointer">
                    <Checkbox checked={rdGenerateTLS} onCheckedChange={(v) => setRdGenerateTLS(!!v)} />
                    <span className="text-xs text-muted-foreground">{t("infrastructure.tls")}</span>
                  </Label>
                  <Label className="flex items-center gap-2 cursor-pointer">
                    <Checkbox checked={rdGenerateWS} onCheckedChange={(v) => setRdGenerateWS(!!v)} />
                    <span className="text-xs text-muted-foreground">{t("infrastructure.websocket")}</span>
                  </Label>
                </div>
              </div>

              <div className="flex gap-2">
                <Button onClick={async () => {
                  setGeneratingModal(true);
                  try {
                    const data = await api.postJson<{ config?: string }>(`/redirectors/generate/${rdType}`, {
                      domain: rdGenerateDomain,
                      listen_port: rdGeneratePort,
                      backend_url: backendURL,
                      ws_enabled: rdGenerateWS,
                    });
                    if (data.config) {
                      setRdConfig(data.config as string);
                      const payload = {
                        name: rdName || rdGenerateHost,
                        host: rdGenerateHost,
                        type: rdType,
                        ssh_user: rdSSHUser,
                        ssh_port: rdSSHPort,
                        ssh_key: rdSSHKey,
                        ssh_password: rdSSHPassword,
                        config: data.config,
                      };
                      if (editingRd) {
                        await api.putJson(paths.redirectors.one(editingRd), payload);
                      } else {
                        await api.postJson(paths.redirectors.list, payload);
                      }
                    }
                  } catch { toast.error(t("infrastructure.toast.generate_config_failed")); }
                  setGeneratingModal(false);
                }} disabled={generatingModal}>
                  {generatingModal ? <Spinner size="sm" className="mr-1" /> : <WandSparkles className="w-4 h-4 mr-1" />} {generatingModal ? t("infra.generating") : t("infra.generate_config")}
                </Button>
                <Button disabled={!rdConfig || deploying} onClick={async () => {
                  setDeploying(true);
                  setDeployResult(null);
                  try {
                    const data = await api.postJson<{ success: boolean; message?: string; stdout?: string; stderr?: string }>("/redirectors/deploy-ssh", {
                      host: rdGenerateHost,
                      port: rdSSHPort,
                      user: rdSSHUser,
                      password: rdSSHPassword,
                      ssh_key: rdSSHKey,
                      config: rdConfig,
                      config_type: rdType,
                    });
                    setDeployResult(data as { success: boolean; message?: string; stdout?: string; stderr?: string });
                    loadRedirectors();
                  } catch (e: unknown) {
                    setDeployResult({ success: false, message: e instanceof Error ? e.message : t("infra.unknown_error") });
                  }
                  setDeploying(false);
                }}>
                  {deploying ? <Spinner size="xs" className="mr-1" /> : <Rocket className="w-4 h-4" />} {deploying ? t("infra.deploying") : t("infra.deploy_via_ssh")}
                </Button>
              </div>

              {rdConfig && (
                <div>
                  <Label className="text-xs">{t("infra.generated_config")} ({rdType.toUpperCase()})</Label>
                  <pre className="bg-card text-emerald-400 p-4 rounded-2xl text-xs overflow-auto max-h-[300px] whitespace-pre-wrap font-mono leading-relaxed border border-border">{rdConfig}</pre>
                </div>
              )}

              {deployResult && (
                <div className={`p-4 rounded-2xl text-sm ${deployResult.success ? "bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-400" : "bg-destructive/10 text-destructive"}`}>
                  <div className="flex items-center gap-2 font-medium mb-1">
                    {deployResult.success ? <CheckCircle className="w-4 h-4" /> : <CircleAlert className="w-4 h-4" />}
                    {deployResult.message}
                  </div>
                  {deployResult.stdout && <pre className="text-xs mt-1 text-muted-foreground">{deployResult.stdout}</pre>}
                  {deployResult.stderr && <pre className="text-xs mt-1 text-muted-foreground">{deployResult.stderr}</pre>}
                </div>
              )}
            </DialogContent>
          </Dialog>
        </TabsContent>

        <TabsContent value="acme" className="mt-0">
        <Card className="overflow-hidden">
          <CardHeader className="px-6 py-4 border-b">
            <div className="w-8 h-8 bg-cyan-100 dark:bg-cyan-900/30 rounded-xl flex items-center justify-center text-cyan-600 dark:text-cyan-400"><Award className="w-4 h-4" /></div>
            <div><CardTitle>{t("infra.acme_title")}</CardTitle><CardDescription>{t("infra.acme_desc")}</CardDescription></div>
          </CardHeader>
          <CardContent className="p-4 sm:p-5 space-y-4">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div>
                <Label className="text-xs"><Globe className="w-4 h-4" />{t("infra.domain")}</Label>
                <Input aria-label="c2.example.com" name="input-19" type="text" value={acmeDomain} onChange={e => setAcmeDomain(e.target.value)} placeholder="c2.example.com" />
              </div>
              <div>
                <Label className="text-xs"><Mail className="w-4 h-4" />{t("infra.email")}</Label>
                <Input aria-label="admin@example.com" name="input-20" type="email" value={acmeEmail} onChange={e => setAcmeEmail(e.target.value)} placeholder="admin@example.com" />
              </div>
              <div>
                <Label className="text-xs"><Plug className="w-4 h-4" />{t("infra.http01_port")}</Label>
                <Input aria-label={t("infra.http01_port")} name="input-21" type="number" value={acmePort} onChange={e => setAcmePort(Number(e.target.value))} />
              </div>
            </div>
            <div className="flex items-center gap-4">
              <Label className="flex items-center gap-2 cursor-pointer">
                <Checkbox checked={acmeStaging} onCheckedChange={(v) => setAcmeStaging(!!v)} />
                <span className="text-xs text-muted-foreground">{t("infra.use_staging")}</span>
              </Label>
              <Button onClick={provisionCert} disabled={acmeProvisioning}>
                {acmeProvisioning ? <Spinner size="xs" /> : <Award className="w-4 h-4" />} {acmeProvisioning ? t("infra.provisioning") : t("infra.auto_provision")}
              </Button>
            </div>
            {certPath && keyPath && (
              <div className="p-3 bg-emerald-50 dark:bg-emerald-900/20 rounded-xl text-xs text-emerald-700 dark:text-emerald-400">
                <CheckCircle className="w-4 h-4" />
                <b>{t("infra.cert_success")}</b> {t("infra.cert_set_to")} {certPath}
              </div>
            )}
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="export" className="mt-0">
        <Card className="overflow-hidden">
          <CardHeader className="px-6 py-4 border-b">
            <div className="w-8 h-8 bg-primary/10 rounded-xl flex items-center justify-center text-primary"><FileUp className="w-4 h-4" /></div>
            <div><CardTitle>{t("infra.c2_profile_export")}</CardTitle><CardDescription>{t("infra.c2_profile_export_desc")}</CardDescription></div>
          </CardHeader>
          <CardContent className="p-4 sm:p-5 space-y-4">
            <div className="mb-4">
              <Label className="text-xs mb-2 block">{t("infra.export_format")}</Label>
              <div className="flex gap-3">
                {([
                  { value: "json", label: "JSON", icon: <Code className="w-4 h-4" />, active: "border-primary bg-primary/10 text-primary" },
                  { value: "nginx", label: "Nginx", icon: <Server className="w-4 h-4" />, active: "border-emerald-500 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400" },
                  { value: "env", label: "ENV", icon: <Terminal className="w-4 h-4" />, active: "border-secondary bg-secondary/50 text-muted-foreground" },
                ] as const).map((opt) => (
                  <Button key={opt.value} onClick={() => setExportFormat(opt.value)}
                    variant={exportFormat === opt.value ? "default" : "outline"}
                    className={exportFormat === opt.value ? opt.active : ""}>
                    {opt.icon}{opt.label}
                  </Button>
                ))}
              </div>
            </div>
            <Button onClick={async () => { setExporting(true); await exportProfile(); setExporting(false); }} disabled={exporting} className="bg-primary hover:bg-primary/90 text-primary-foreground">
              {exporting ? <Spinner size="sm" /> : <Download className="w-4 h-4" />} {exporting ? t("infra.exporting") : `${t("infra.export_as")} ${exportFormat.toUpperCase()}`}
            </Button>
            <Card className="bg-muted">
              <CardContent>
                <Info className="w-4 h-4" />
                <b>{t("infra.about")}</b> {t("infra.about_desc")}
              </CardContent>
            </Card>
          </CardContent>
        </Card>
      </TabsContent>
      </Tabs>
      {modal}
      </DataState>
    </div>
  );
}

