"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { FileText, Play, Plus, Save, Trash2 } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface Template {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  command?: string;
  Command?: string;
  description?: string;
  Description?: string;
  category?: string;
  Category?: string;
}


export default function CommandTemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(true);
  const [showAdd, setShowAdd] = useState(false);
  const [form, setForm] = useState({ name: "", category: "recon", command: "", description: "" });
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const { t } = useI18n();

  const categories = [
    { key: "recon", label: t("templates.cat_recon"), emoji: "\u{1f50d}" },
    { key: "privesc", label: t("templates.cat_privesc"), emoji: "\u{1f6e1}\u{fe0f}" },
    { key: "lateral", label: t("templates.cat_lateral"), emoji: "\u2194\ufe0f" },
    { key: "exfil", label: t("templates.cat_exfil"), emoji: "\u{1f4e4}" },
    { key: "persist", label: t("templates.cat_persist"), emoji: "\u{1f4be}" },
  ];

  const loadTemplates = useCallback(async () => {
    try {
      const data = await api.get<{ templates?: Template[]; Templates?: Template[] } | Template[]>("/api/templates");
      setTemplates(Array.isArray(data) ? data : data.templates || []);
    } catch {
      setTemplates([]);
      toast.error(t("templates.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { loadTemplates(); }, [loadTemplates]);

  const handleSave = async () => {
    try {
      await api.postJson(paths.templates.list, form);
      setShowAdd(false);
      setForm({ name: "", category: "recon", command: "", description: "" });
      loadTemplates();
    } catch { toast.error(t("templates.toast.save_failed")); }
  };

  const handleDelete = (id: string) => {
    setCfm({msg: t("templates.confirm_delete"), cb: async () => {
      try {
        await api.del(paths.templates.one(id));
        loadTemplates();
      } catch { toast.error(t("templates.toast.delete_failed")); }
    }});
  };

  const grouped: Record<string, Template[]> = {};
  templates.forEach((tmpl) => {
    const cat = tmpl.category || "other";
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(tmpl);
  });

  const getCatInfo = (cat: string) => categories.find((c) => c.key === cat) || { key: cat, label: cat, emoji: "\u{1f4c4}" };

  if (loading)
    return (
      <PageSpinner />
    );

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("templates.title")} subtitle={t("templates.subtitle")}>
        <Button onClick={() => setShowAdd(true)}>
          <Plus className="w-4 h-4" />
          <span>{t("templates.add_template")}</span>
        </Button>
      </PageHeader>

      {templates.length === 0 ? (
        <Card className="p-12 text-center">
          <EmptyState icon={FileText} title={t("templates.empty_title")} message={t("templates.empty_desc")} />
          <Button onClick={() => setShowAdd(true)}>
            <Plus className="w-4 h-4" />{t("templates.add_template")}
          </Button>
        </Card>
      ) : (
        <div className="space-y-6">
        {Object.entries(grouped).map(([cat, temps]) => {
          const info = getCatInfo(cat);
          return (
            <div key={cat}>
              <h2 className="text-xl font-semibold tracking-tight text-foreground leading-tight mb-4 flex items-center gap-2">
                <span>{info.emoji}</span>
                {info.label}
                <span className="text-sm font-normal text-muted-foreground">({temps.length} {t("templates.count_unit")}</span>
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {temps.map((tmpl) => {
                  const id = tmpl.id || "";
                  const name = tmpl.name || "";
                  const cmd = tmpl.command || "";
                  const desc = tmpl.description || "";
                  return (
                    <Card key={id} className="p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow group">
                      <div className="flex items-start justify-between mb-3">
                        <h3 className="font-semibold text-foreground">{name}</h3>
                        <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(id)} className="text-muted-foreground hover:text-destructive opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 transition-opacity" aria-label={t("common.delete")}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                      {desc && <p className="text-xs text-muted-foreground mb-3">{desc}</p>}
                      <div className="bg-muted border border-border rounded-xl p-3 mb-3">
                        <code className="text-xs font-mono text-foreground break-all">{cmd}</code>
                      </div>
                       <Button className="w-full" variant="outline" onClick={() => { navigator.clipboard.writeText(cmd); toast.success(t("templates.toast.copied")); }}>
                        <Play className="w-4 h-4" />{t("templates.use_template")}
                      </Button>
                    </Card>
                  );
                })}
              </div>
            </div>
          );
        })}
        </div>
      )}

      <Dialog open={showAdd} onOpenChange={setShowAdd}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("templates.add_title")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label className="mb-2">{t("templates.field_name")}</Label>
              <Input placeholder={t("templates.ph_name")} value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </div>
            <div>
              <Label className="mb-2">{t("templates.field_category")}</Label>
              <Select value={form.category} onValueChange={(v) => setForm({ ...form, category: v ?? "" })}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("command_templates.category_ph")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="recon">{t("templates.opt_recon")}</SelectItem>
                  <SelectItem value="privesc">{t("templates.opt_privesc")}</SelectItem>
                  <SelectItem value="lateral">{t("templates.opt_lateral")}</SelectItem>
                  <SelectItem value="exfil">{t("templates.opt_exfil")}</SelectItem>
                  <SelectItem value="persist">{t("templates.opt_persist")}</SelectItem>
                  <SelectItem value="other">{t("templates.opt_other")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="mb-2">{t("templates.field_command")}</Label>
              <Textarea rows={4} placeholder={t("command_templates.cmd_ph")} value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} />
            </div>
            <div>
              <Label className="mb-2">{t("templates.field_description")}</Label>
              <Input placeholder={t("templates.ph_desc")} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowAdd(false)}>
              {t("templates.cancel")}
            </Button>
            <Button onClick={handleSave}>
              <Save className="w-4 h-4" />{t("templates.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!cfm} onOpenChange={(open) => { if (!open) setCfm(null); }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("common.confirm")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{cfm?.msg || ""}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCfm(null)}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={() => { cfm?.cb(); setCfm(null); }}>
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
