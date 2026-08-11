"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EmptyState, PageHeader } from "@/components/UI";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Pencil, Play, Plus, Trash2, Wand2, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface AgentTag {
  id: string;
  name: string;
  color: string;
}

interface AutoTagRule {
  id: string;
  name: string;
  enabled: boolean;
  condition: string;
  tag_id: string;
  priority: number;
  created_at: string;
  updated_at: string;
  tag?: AgentTag;
}

interface TagCondition {
  _key?: number;
  field: string;
  op: string;
  value: string;
}

let conditionKeyCounter = 0;

const FIELDS = ["hostname", "os", "arch", "ip", "username", "domain", "process_name", "external_ip"];
const OPS = ["contains", "equals", "starts_with", "regex", "not_equals"];

export default function AutoTagPage() {
  const { t } = useI18n();
  const [rules, setRules] = useState<AutoTagRule[]>([]);
  const [tags, setTags] = useState<AgentTag[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [tagId, setTagId] = useState("");
  const [priority, setPriority] = useState(0);
  const [conditions, setConditions] = useState<TagCondition[]>(() => {
    conditionKeyCounter++;
    return [{ _key: conditionKeyCounter, field: "hostname", op: "contains", value: "" }];
  });
  const { confirm, modal } = useConfirm();
  const [message, setMessage] = useState("");

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [r, t] = await Promise.all([
        api.get<{ rules: AutoTagRule[] }>(paths.autotag.rules),
        api.get<{ tags: AgentTag[] }>(paths.tags.list),
      ]);
      setRules(r.rules || []);
      setTags(t.tags || []);
    } catch { setMessage(t("autotag.load_failed")); }
    finally { setLoading(false); }
  }, [t]);

  useEffect(() => { fetchData(); }, [fetchData]);

  function resetForm() {
    conditionKeyCounter++;
    setName(""); setTagId(""); setPriority(0); setConditions([{ _key: conditionKeyCounter, field: "hostname", op: "contains", value: "" }]); setEditingId(null);
  }

  async function handleSave() {
    if (!name || !tagId || conditions.length === 0 || conditions.some(c => !c.value)) {
      setMessage(t("autotag.fill_fields")); return;
    }
    const body = { name, enabled: true, condition: conditions, tag_id: tagId, priority };
    try {
      if (editingId) {
        await api.putJson(paths.autotag.rule(editingId), body);
      } else {
        await api.postJson(paths.autotag.rules, body);
      }
      resetForm(); setShowForm(false); setMessage(t("autotag.saved")); fetchData();
    } catch { setMessage(t("autotag.save_failed")); }
  }

  async function handleToggle(id: string) {
      try { await api.postJson(paths.autotag.toggle(id), {}); fetchData(); }
    catch { setMessage(t("autotag.toggle_failed")); }
  }

  async function handleDelete(id: string) {
    if (!(await confirm({ message: t("autotag.delete_confirm") }))) return;
    try { await api.del(paths.autotag.rule(id)); fetchData(); }
    catch { setMessage(t("autotag.delete_failed")); }
  }

  async function handleApplyAll() {
    try {
      const res = await api.postJson<{ data?: { applied: number } }>(paths.autotag.apply, {});
      setMessage(t("autotag.applied", { count: res.data?.applied ?? 0 }));
    } catch { setMessage(t("autotag.apply_failed")); }
  }

  function editRule(rule: AutoTagRule) {
    setEditingId(rule.id);
    setName(rule.name);
    setTagId(rule.tag_id);
    setPriority(rule.priority);
    try { setConditions(JSON.parse(rule.condition)); }
    catch { setConditions([{ field: "hostname", op: "contains", value: "" }]); }
    setShowForm(true);
  }

  function addCondition() {
    conditionKeyCounter++;
    setConditions([...conditions, { _key: conditionKeyCounter, field: "hostname", op: "contains", value: "" }]);
  }

  function removeCondition(i: number) {
    if (conditions.length <= 1) return;
    setConditions(conditions.filter((_, idx) => idx !== i));
  }

  function updateCondition(i: number, key: keyof TagCondition, val: string) {
    const c = [...conditions];
    c[i] = { ...c[i], [key]: val };
    setConditions(c);
  }

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      {message && (
        <div className="mb-4 px-4 py-2 rounded-xl bg-info/8 text-info text-sm border border-info/20 flex items-center justify-between animate-fade-in">
          <span>{message}</span>
          <Button variant="ghost" size="icon-sm" onClick={() => setMessage("")} aria-label={t("common.dismiss")}>
            <X className="w-4 h-4" />
          </Button>
        </div>
      )}

      <PageHeader title={t("autotag.title")} subtitle={t("autotag.subtitle")}>
        <Button onClick={handleApplyAll}>
          <Play className="w-4 h-4" /> {t("autotag.apply_all")}
        </Button>
        <Button onClick={() => { resetForm(); setShowForm(true); }}>
          <Plus className="w-4 h-4" /> {t("autotag.new_rule")}
        </Button>
      </PageHeader>

      <Dialog open={showForm} onOpenChange={(open) => { if (!open) { setShowForm(false); resetForm(); } }}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>{editingId ? t("autotag.edit_rule") : t("autotag.new_rule")}</DialogTitle>
          </DialogHeader>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <Label htmlFor="autotag-name" className="mb-1 text-xs">{t("autotag.name")}</Label>
              <Input id="autotag-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div>
              <Label htmlFor="autotag-tag" className="mb-1 text-xs">{t("autotag.tag")}</Label>
              <Select value={tagId} onValueChange={(v) => setTagId(v ?? "")}>
                <SelectTrigger id="autotag-tag" className="w-full">
                  <SelectValue placeholder={t("autotag.select_tag")} />
                </SelectTrigger>
                <SelectContent>
                  {tags.map((t) => (
                    <SelectItem key={t.id} value={t.id}>{t.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="autotag-priority" className="mb-1 text-xs">{t("autotag.priority")}</Label>
              <Input id="autotag-priority" type="number" value={priority} onChange={(e) => setPriority(parseInt(e.target.value) || 0)} />
            </div>
          </div>

          <div>
            <Label className="mb-2 text-xs">{t("autotag.conditions")}</Label>
            {conditions.map((c, i) => (
              <div key={c._key ?? i} className="flex flex-col sm:flex-row gap-2 mb-2">
                <Select value={c.field} onValueChange={(v) => updateCondition(i, "field", v ?? "")}>
                  <SelectTrigger className="w-full sm:w-40">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FIELDS.map((f) => <SelectItem key={f} value={f}>{f}</SelectItem>)}

                  </SelectContent>
                </Select>
                <Select value={c.op} onValueChange={(v) => updateCondition(i, "op", v ?? "")}>
                  <SelectTrigger className="w-full sm:w-36">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {OPS.map((o) => <SelectItem key={o} value={o}>{o}</SelectItem>)}

                  </SelectContent>
                </Select>
                <Input
                  value={c.value}
                  onChange={(e) => updateCondition(i, "value", e.target.value)}
                  placeholder={t("autotag.value_ph")}
                  className="flex-1"
                />
                <Button variant="ghost" size="icon-sm" onClick={() => removeCondition(i)} className="text-destructive hover:text-destructive hover:bg-destructive/10" aria-label={t("common.remove")}>
                  <X className="w-4 h-4" />
                </Button>
              </div>
            ))}
            <Button variant="ghost" size="sm" onClick={addCondition} className="text-primary mt-1">
              <Plus className="w-4 h-4" /> {t("autotag.add_condition")}
            </Button>
          </div>

          <DialogFooter>
            <Button onClick={handleSave}>
              {editingId ? t("autotag.update") : t("autotag.create")}
            </Button>
            <Button variant="outline" onClick={() => { setShowForm(false); resetForm(); }}>
              {t("common.cancel")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <Card key={i} className="p-4">
              <div className="flex items-center gap-4">
                <Skeleton className="w-8 h-5 rounded-full" />
                <div className="flex-1">
                  <Skeleton className="h-4 w-32 mb-2" />
                  <Skeleton className="h-3 w-64" />
                </div>
              </div>
            </Card>
          ))}
        </div>
      ) : rules.length === 0 ? (
        <div className="text-center py-16 sm:py-20 text-muted-foreground">
          <EmptyState icon={Wand2} title={t("autotag.empty_title")} message={t("autotag.empty_message")} />
        </div>
      ) : (
        <div className="space-y-2">
          {rules.map((rule) => (
            <Card key={rule.id} className="p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
              <div className="flex items-center gap-4">
                <Switch checked={rule.enabled} onCheckedChange={() => handleToggle(rule.id)} aria-label={t("autotag.toggle_rule")} />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium text-sm text-foreground">{rule.name}</span>
                    {rule.tag && (
                      <Badge variant="outline" style={{ backgroundColor: rule.tag.color + '20', color: rule.tag.color, borderColor: rule.tag.color + '40' }}>
                        {rule.tag.name}
                      </Badge>
                    )}
                    <span className="text-(--fs-xs-sm) text-muted-foreground">{t("autotag.priority_label")} {rule.priority}</span>
                  </div>
                  <div className="text-(--fs-compact) text-muted-foreground mt-0.5 font-mono truncate">
                    {(() => { try { const c = JSON.parse(rule.condition); return c.map((cc: TagCondition) => `${cc.field} ${cc.op} "${cc.value}"`).join(" AND "); } catch { return rule.condition; } })()}
                  </div>
                </div>
                <div className="flex gap-1 shrink-0">
                  <Button variant="outline" size="icon" onClick={() => editRule(rule)} aria-label={t("common.edit")}>
                    <Pencil className="w-4 h-4" />
                  </Button>
                  <Button variant="destructive" size="icon" onClick={() => handleDelete(rule.id)} aria-label={t("common.delete")}>
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
      {modal}
    </div>
  );
}
