"use client";

import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { ConfirmModal, EmptyState, PageHeader, PageSpinner } from "@/components/UI";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { FileText, Key, Pencil, Play, Plus, Save, Send, Square, Trash2 } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

interface PhishingTemplate {
  id: number;
  ID: number;
  name: string;
  subject: string;
  body: string;
  from_name: string;
  from_email: string;
  type: string;
  created_by: string;
  created_at: string;
}

interface PhishingCampaign {
  id: number;
  ID: number;
  name: string;
  template_id: number;
  target_list: string;
  smtp_host: string;
  smtp_port: number;
  smtp_user: string;
  smtp_pass: string;
  status: string;
  sent_count: number;
  open_count: number;
  cred_count: number;
  created_by: string;
  created_at: string;
}

interface CaptureEntry {
  id: number;
  ID: number;
  username: string;
  password: string;
  domain: string;
  source: string;
  type: string;
  created_at: string;
}

type Tab = "templates" | "campaigns" | "captures";

export default function PhishingPageContent() {
  const { t } = useI18n();
  const [templates, setTemplates] = useState<PhishingTemplate[]>([]);
  const [campaigns, setCampaigns] = useState<PhishingCampaign[]>([]);
  const [captures, setCaptures] = useState<CaptureEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [cfm, setCfm] = useState<{ msg: string; cb: () => void } | null>(null);

  // Template form
  const [showTplForm, setShowTplForm] = useState(false);
  const [tplForm, setTplForm] = useState({ name: "", subject: "", body: "", from_name: "", from_email: "", type: "html" });
  const [editTplId, setEditTplId] = useState<number | null>(null);

  // Campaign form
  const [showCampForm, setShowCampForm] = useState(false);
  const [campForm, setCampForm] = useState({ name: "", template_id: 0, target_list: "", smtp_host: "", smtp_port: 587, smtp_user: "", smtp_pass: "" });

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: "templates", label: t("phishing.tab_templates"), icon: <FileText className="w-4 h-4" /> },
    { key: "campaigns", label: t("phishing.tab_campaigns"), icon: <Send className="w-4 h-4" /> },
    { key: "captures", label: t("phishing.tab_captures"), icon: <Key className="w-4 h-4" /> },
  ];

  const loadTemplates = useCallback(async () => {
    try {
      const d = await api.get("/phishing/templates");
      setTemplates((d.data as PhishingTemplate[]) || []);
    } catch {
      setTemplates([]);
      toast.error(t("phishing.toast.load_failed"));
    }
  }, [t]);

  const loadCampaigns = useCallback(async () => {
    try {
      const d = await api.get("/phishing/campaigns");
      setCampaigns((d.data as PhishingCampaign[]) || []);
    } catch {
      setCampaigns([]);
      toast.error(t("phishing.toast.load_failed"));
    }
  }, [t]);

  const loadCaptures = useCallback(async () => {
    try {
      const d = await api.get("/phishing/captures");
      setCaptures((d.data as CaptureEntry[]) || []);
    } catch {
      setCaptures([]);
      toast.error(t("phishing.toast.load_failed"));
    }
  }, [t]);

  useEffect(() => {
    setLoading(true);
    Promise.all([loadTemplates(), loadCampaigns(), loadCaptures()]).finally(() => setLoading(false));
  }, [loadTemplates, loadCampaigns, loadCaptures]);

  // ── Template CRUD ──────────────────────────────────────────────────────

  const handleSaveTpl = async () => {
    try {
      if (editTplId) {
        await api.putJson(`/phishing/templates/${editTplId}`, tplForm);
      } else {
        await api.postJson("/phishing/templates", tplForm);
      }
      setShowTplForm(false);
      setEditTplId(null);
      setTplForm({ name: "", subject: "", body: "", from_name: "", from_email: "", type: "html" });
      loadTemplates();
    } catch { toast.error(t("phishing.toast.save_template_failed")); }
  };

  const handleEditTpl = (t: PhishingTemplate) => {
    setEditTplId(t.id);
    setTplForm({ name: t.name, subject: t.subject, body: t.body, from_name: t.from_name, from_email: t.from_email, type: t.type || "html" });
    setShowTplForm(true);
  };

  const handleDeleteTpl = (id: number) => {
    setCfm({ msg: t("phishing.delete_template"), cb: async () => {
      await api.del(`/phishing/templates/${id}`);
      loadTemplates();
    }});
  };

  // ── Campaign CRUD ──────────────────────────────────────────────────────

  const handleSaveCamp = async () => {
    try {
      await api.postJson("/phishing/campaigns", campForm);
      setShowCampForm(false);
      setCampForm({ name: "", template_id: 0, target_list: "", smtp_host: "", smtp_port: 587, smtp_user: "", smtp_pass: "" });
      loadCampaigns();
    } catch { toast.error(t("phishing.toast.save_campaign_failed")); }
  };

  const handleLaunch = async (id: number) => {
    try {
      const res = await api.postJson<{ success?: boolean; queued?: number; message?: string; error?: string }>(
        `/phishing/campaigns/${id}/launch`,
        {},
      );
      if (res.message) toast.success(res.message);
      else toast.success(t("phishing.toast.launched", { count: res.queued ?? 0 }));
      loadCampaigns();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("phishing.toast.launch_failed"));
    }
  };

  const handleStop = async (id: number) => {
    try {
      await api.post(`/phishing/campaigns/${id}/stop`);
      loadCampaigns();
    } catch { toast.error(t("phishing.toast.save_campaign_failed")); }
  };

  const handleDeleteCamp = (id: number) => {
    setCfm({ msg: t("phishing.delete_campaign"), cb: async () => {
      await api.del(`/phishing/campaigns/${id}`);
      loadCampaigns();
    }});
  };

  if (loading) return <PageSpinner />;

  // ── Templates Tab ──────────────────────────────────────────────────────

  function renderTemplates() {
    return (
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-foreground">{t("phishing.templates_title")}</h2>
          <Button onClick={() => { setEditTplId(null); setTplForm({ name: "", subject: "", body: "", from_name: "", from_email: "", type: "html" }); setShowTplForm(true); }}>
            <Plus className="w-4 h-4" /> {t("phishing.new_template")}
          </Button>
        </div>
        {templates.length === 0 ? (
          <EmptyState icon={FileText} title={t("phishing.empty_templates")} message={t("phishing.empty_templates_hint")} />
        ) : (
          <Card>
            <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="text-muted-foreground text-xs uppercase tracking-wider font-semibold">
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_name")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_subject")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_from")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_type")}</TableHead>
                  <TableHead className="text-right px-4 py-3 sm:py-3.5">{t("phishing.col_actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {templates.map((t) => {
                  const tid = t.id;
                  return (
                    <TableRow key={tid}>
                      <TableCell className="px-4 py-3 sm:py-3.5 font-medium text-foreground">{t.name}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-muted-foreground max-w-[200px] truncate">{t.subject}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-muted-foreground">{t.from_name || t.from_email}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5"><Badge variant="outline">{t.type}</Badge></TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-right">
                        <Button variant="ghost" size="icon-xs" onClick={() => handleEditTpl(t)} className="text-muted-foreground hover:text-indigo-600 dark:hover:text-indigo-400 mr-3" aria-label="Edit template"><Pencil className="w-4 h-4" /></Button>
                        <Button variant="ghost" size="icon-xs" onClick={() => handleDeleteTpl(tid)} className="text-muted-foreground hover:text-destructive" aria-label="Delete template"><Trash2 className="w-4 h-4" /></Button>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            </div>
          </Card>
        )}
        <Dialog open={showTplForm} onOpenChange={(open) => {
          setShowTplForm(open);
          if (!open) setEditTplId(null);
        }}>
          <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto" showCloseButton={false}>
            <DialogHeader className="bg-gradient-to-r from-indigo-500 to-purple-500 -m-4 mb-0 px-6 py-5 rounded-t-xl">
              <DialogTitle className="text-white">{editTplId ? t("phishing.dialog_edit_template") : t("phishing.dialog_new_template")}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label>{t("phishing.field_template_name")}</Label>
                  <Input value={tplForm.name} onChange={(e) => setTplForm({ ...tplForm, name: e.target.value })} />
                </div>
                <div>
                  <Label>{t("common.type")}</Label>
                  <Select value={tplForm.type} onValueChange={(v) => setTplForm({ ...tplForm, type: v || "html" })}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="html">{t("phishing.type_html")}</SelectItem>
                      <SelectItem value="text">{t("phishing.type_plain")}</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div>
                <Label>{t("phishing.field_subject")}</Label>
                <Input value={tplForm.subject} onChange={(e) => setTplForm({ ...tplForm, subject: e.target.value })} />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label>{t("phishing.field_from_name")}</Label>
                  <Input value={tplForm.from_name} onChange={(e) => setTplForm({ ...tplForm, from_name: e.target.value })} />
                </div>
                <div>
                  <Label>{t("phishing.field_from_email")}</Label>
                  <Input value={tplForm.from_email} onChange={(e) => setTplForm({ ...tplForm, from_email: e.target.value })} />
                </div>
              </div>
              <div>
                <Label>
                  {t("phishing.field_body")}
                  <span className="text-xs text-muted-foreground ml-2">Supports {'{{.Link}}'}, {'{{.Token}}'}, {'{{.Email}}'}, {'{{.Username}}'}</span>
                </Label>
                <Textarea rows={12} className="font-mono" value={tplForm.body} onChange={(e) => setTplForm({ ...tplForm, body: e.target.value })} />
              </div>
            </div>
            <div className="flex gap-3">
              <Button onClick={handleSaveTpl} className="flex-1"><Save className="w-4 h-4" />{t("common.save")}</Button>
              <Button variant="outline" onClick={() => { setShowTplForm(false); setEditTplId(null); }}>{t("common.cancel")}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    );
  }

  // ── Campaigns Tab ──────────────────────────────────────────────────────

  function renderCampaigns() {
    const statusBadge = (s: string) => {
      const variant = s === "running" ? "warning" as const : s === "completed" ? "success" as const : "secondary" as const;
      return <Badge variant={variant}>{s}</Badge>;
    };
    return (
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-foreground">{t("phishing.campaigns_title")}</h2>
          <Button onClick={() => setShowCampForm(true)}>
            <Plus className="w-4 h-4" /> {t("phishing.new_campaign")}
          </Button>
        </div>
        {campaigns.length === 0 ? (
          <EmptyState icon={Send} title={t("campaign.empty")} />
        ) : (
          <Card>
            <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="text-muted-foreground text-xs uppercase tracking-wider font-semibold">
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_name")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_status")}</TableHead>
                  <TableHead className="text-right px-4 py-3 sm:py-3.5">{t("common.total")}</TableHead>
                  <TableHead className="text-right px-4 py-3 sm:py-3.5">{t("phishing.col_from")}</TableHead>
                  <TableHead className="text-right px-4 py-3 sm:py-3.5">{t("phishing.col_subject")}</TableHead>
                  <TableHead className="text-right px-4 py-3 sm:py-3.5">{t("phishing.col_actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {campaigns.map((c) => {
                  const cid = c.id;
                  return (
                    <TableRow key={cid}>
                      <TableCell className="px-4 py-3 sm:py-3.5 font-medium text-foreground">{c.name}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5">{statusBadge(c.status)}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-right text-muted-foreground">{c.sent_count}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-right text-muted-foreground">{c.open_count}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-right text-muted-foreground">{c.cred_count}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-right">
                        {c.status === "draft" && <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" onClick={() => handleLaunch(cid)} className="text-indigo-600 dark:text-indigo-400 hover:text-indigo-800 dark:hover:text-indigo-300 mr-3" aria-label="Launch" />}><Play className="w-4 h-4" /></TooltipTrigger><TooltipContent>Launch</TooltipContent></Tooltip>}
                        {c.status === "running" && <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" onClick={() => handleStop(cid)} className="text-amber-600 dark:text-amber-400 hover:text-amber-800 dark:hover:text-amber-300 mr-3" aria-label="Stop" />}><Square className="w-4 h-4" /></TooltipTrigger><TooltipContent>Stop</TooltipContent></Tooltip>}
                        {(c.status === "draft" || c.status === "completed") && <Tooltip><TooltipTrigger render={<Button variant="ghost" size="icon-xs" onClick={() => handleDeleteCamp(cid)} className="text-muted-foreground hover:text-destructive" aria-label="Delete" />}><Trash2 className="w-4 h-4" /></TooltipTrigger><TooltipContent>Delete</TooltipContent></Tooltip>}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            </div>
          </Card>
        )}
        <Dialog open={showCampForm} onOpenChange={setShowCampForm}>
          <DialogContent className="sm:max-w-2xl max-h-[90vh] overflow-y-auto" showCloseButton={false}>
            <DialogHeader className="bg-gradient-to-r from-indigo-500 to-purple-500 -m-4 mb-0 px-6 py-5 rounded-t-xl">
              <DialogTitle className="text-white">{t("phishing.new_campaign")}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <div>
                <Label>{t("phishing.campaign_name")}</Label>
                <Input value={campForm.name} onChange={(e) => setCampForm({ ...campForm, name: e.target.value })} />
              </div>
              <div>
                <Label>{t("phishing.field_template")}</Label>
                <Select value={String(campForm.template_id)} onValueChange={(v) => {
                  if (v !== null) setCampForm({ ...campForm, template_id: parseInt(v) });
                }}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("phishing.field_template")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">{t("phishing.field_template")}</SelectItem>
                    {templates.map((t) => (<SelectItem key={t.id} value={String(t.id)}>{t.name}</SelectItem>))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label>{t("phishing.field_targets")}</Label>
                <Textarea rows={4} className="font-mono" value={campForm.target_list} onChange={(e) => setCampForm({ ...campForm, target_list: e.target.value })} placeholder='["user1@example.com","user2@example.com"]' />
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label>{t("phishing.field_smtp_host")}</Label>
                  <Input value={campForm.smtp_host} onChange={(e) => setCampForm({ ...campForm, smtp_host: e.target.value })} />
                </div>
                <div>
                  <Label>{t("phishing.field_smtp_port")}</Label>
                  <Input type="number" value={campForm.smtp_port} onChange={(e) => setCampForm({ ...campForm, smtp_port: parseInt(e.target.value) })} />
                </div>
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <Label>{t("phishing.field_smtp_user")}</Label>
                  <Input value={campForm.smtp_user} onChange={(e) => setCampForm({ ...campForm, smtp_user: e.target.value })} />
                </div>
                <div>
                  <Label>{t("phishing.field_smtp_pass")}</Label>
                  <Input type="password" value={campForm.smtp_pass} onChange={(e) => setCampForm({ ...campForm, smtp_pass: e.target.value })} />
                </div>
              </div>
            </div>
            <div className="flex gap-3">
              <Button onClick={handleSaveCamp} className="flex-1"><Save className="w-4 h-4" />{t("phishing.create_campaign")}</Button>
              <Button variant="outline" onClick={() => setShowCampForm(false)}>{t("common.cancel")}</Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    );
  }

  // ── Captures Tab ───────────────────────────────────────────────────────

  function renderCaptures() {
    const typeBadge = (t: string) => {
      const variant = t === "cleartext" ? "success" as const : t === "ntlm" ? "warning" as const : "secondary" as const;
      return <Badge variant={variant}>{t}</Badge>;
    };
    return (
      <div>
        <h2 className="text-lg font-semibold text-foreground mb-4">{t("phishing.captures_title")}</h2>
        {captures.length === 0 ? (
          <EmptyState icon={Key} title={t("phishing.empty_captures")} message={t("phishing.empty_captures_hint")} />
        ) : (
          <Card>
            <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="text-muted-foreground text-xs uppercase tracking-wider font-semibold">
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("users.label_username")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_password")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_type")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_from")}</TableHead>
                  <TableHead className="text-left px-4 py-3 sm:py-3.5">{t("phishing.col_date")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {captures.map((cr) => {
                  const cid = cr.id;
                  return (
                    <TableRow key={cid}>
                      <TableCell className="px-4 py-3 sm:py-3.5 font-medium text-foreground">{cr.username}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-muted-foreground font-mono text-xs">{cr.password ? cr.password.substring(0, 40) + (cr.password.length > 40 ? "..." : "") : "-"}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5">{typeBadge(cr.type)}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-muted-foreground text-xs">{cr.source}</TableCell>
                      <TableCell className="px-4 py-3 sm:py-3.5 text-muted-foreground text-xs">{cr.created_at ? formatTime(cr.created_at) : ""}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
            </div>
          </Card>
        )}
      </div>
    );
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("phishing.title")} subtitle={t("phishing.subtitle")} />

      {/* Tabs */}
      <Tabs defaultValue="templates">
        <TabsList className="mb-6">
          {tabs.map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-1.5">
              {tab.icon}
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="templates">
        {renderTemplates()}
      </TabsContent>

      <TabsContent value="campaigns">
        {renderCampaigns()}
      </TabsContent>

      <TabsContent value="captures">
        {renderCaptures()}
      </TabsContent>
      </Tabs>

      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.delete")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

