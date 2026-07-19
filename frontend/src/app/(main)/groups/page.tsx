"use client";
import { useState, useEffect, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { PageHeader, EmptyState, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Layers, Plus } from "lucide-react";

interface AgentGroup {
  id: string;
  name: string;
  description: string;
  color: string;
  parent_id: string;
  agent_count: number;
  child_count: number;
  created_at: string;
  updated_at: string;
}

export default function GroupsPage() {
  const { t } = useI18n();
  const [groups, setGroups] = useState<AgentGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editGroup, setEditGroup] = useState<AgentGroup | null>(null);
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const [formName, setFormName] = useState("");
  const [formDesc, setFormDesc] = useState("");
  const [formColor, setFormColor] = useState("#2ecc71");
  const [formParent, setFormParent] = useState("");

  const fetchGroups = useCallback(async () => {
    setLoading(true);
    try {
      const data: { groups?: AgentGroup[] } = await api.json("/groups");
      setGroups(data.groups || []);
    } catch { setGroups([]); toast.error(t("groups.toast.load_failed")); }
    setLoading(false);
  }, [t]);

  useEffect(() => { fetchGroups(); }, [fetchGroups]);

  function openCreate() {
    setEditGroup(null);
    setFormName(""); setFormDesc(""); setFormColor("#2ecc71"); setFormParent("");
    setShowModal(true);
  }

  function openEdit(g: AgentGroup) {
    setEditGroup(g);
    setFormName(g.name); setFormDesc(g.description); setFormColor(g.color); setFormParent(g.parent_id);
    setShowModal(true);
  }

  async function handleSave() {
    if (!formName.trim()) return;
    try {
      const body = { name: formName.trim(), description: formDesc, color: formColor, parent_id: formParent };
      const data: { success?: boolean; error?: string } = editGroup
        ? await api.putJson(`/groups/${editGroup.id}`, body)
        : await api.postJson("/groups", body);
      if (data.success) { setShowModal(false); fetchGroups(); toast.success(editGroup ? t("groups.toast.updated") : t("groups.toast.created")); }
      else { toast.error(data.error || t("groups.toast.save_failed")); }
    } catch { toast.error(t("groups.toast.save_failed")); }
  }

  async function handleDelete() {
    if (!deleteId) return;
    const id = deleteId;
    setDeleteId(null);
    try {
      const data: { success?: boolean; error?: string } = await api.del(`/groups/${id}`);
      if (data.success) { fetchGroups(); toast.success(t("groups.toast.deleted")); }
      else { toast.error(data.error || t("groups.toast.delete_failed")); }
    } catch { toast.error(t("groups.toast.delete_failed")); }
  }

  const rootGroups = groups.filter(g => !g.parent_id);
  const hasChildren = (id: string) => groups.some(g => g.parent_id === id);

  function renderGroup(g: AgentGroup, depth = 0) {
    const children = groups.filter(c => c.parent_id === g.id);
    return (
      <div key={g.id}>
        <div
          className="flex justify-between items-center px-3.5 py-2.5 my-1 rounded-lg bg-card hover:bg-muted transition-colors border-l-[3px]"
          style={{ marginLeft: depth * 24, borderLeftColor: g.color }}
        >
          <div className="flex flex-col gap-0.5">
            <span className="text-sm font-semibold text-foreground">{g.name}</span>
            {g.description && <span className="text-xs text-muted-foreground">{g.description}</span>}
            <span className="text-[11px] text-muted-foreground">{g.agent_count} {t("groups.agent_count")}, {g.child_count} sub-groups</span>
          </div>
          <div className="flex gap-1.5">
                <Button variant="outline" size="sm" onClick={() => openEdit(g)}>{t("groups.edit")}</Button>
             <Button variant="destructive" size="sm" onClick={() => setDeleteId(g.id)}
               disabled={g.agent_count > 0 || hasChildren(g.id)}>{t("groups.delete")}</Button>
          </div>
        </div>
        {children.map(c => renderGroup(c, depth + 1))}
      </div>
    );
  }

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("groups.title")} subtitle={t("groups.subtitle")}>
        <Button onClick={openCreate}><Plus className="w-4 h-4" /> {t("groups.new")}</Button>
      </PageHeader>
      {loading ? (
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      ) : groups.length === 0 ? (
        <Card className="p-12 text-center">
          <EmptyState icon={Layers} title={t("groups.empty")} message={t("groups.empty_desc")} />
          <Button onClick={openCreate}>{t("groups.create")}</Button>
        </Card>
      ) : (
        <Card className="p-4 sm:p-5">
          {rootGroups.map(g => renderGroup(g))}
        </Card>
      )}
      <Dialog open={showModal} onOpenChange={setShowModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editGroup ? t("groups.dialog_edit") : t("groups.dialog_create")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="group-name">{t("groups.field_name")} *</Label>
              <Input id="group-name" aria-label="Group name" value={formName} onChange={e => setFormName(e.target.value)} placeholder={t("groups.name_placeholder")} />
            </div>
            <div>
              <Label htmlFor="group-desc">{t("groups.field_desc")}</Label>
              <Input id="group-desc" aria-label="Optional description" value={formDesc} onChange={e => setFormDesc(e.target.value)} placeholder={t("groups.desc_placeholder")} />
            </div>
            <div>
              <Label htmlFor="group-color">{t("groups.field_color")}</Label>
              <Input id="group-color" aria-label="color" type="color" value={formColor} onChange={e => setFormColor(e.target.value)} />
            </div>
            <div>
              <Label>{t("groups.field_parent")}</Label>
              <Select value={formParent || "__none__"} onValueChange={(v) => setFormParent(v === "__none__" || v === null ? "" : v)}>
                <SelectTrigger className="w-full" aria-label="Parent group">
                  <SelectValue placeholder={t("groups.none_root")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{t("groups.none_root")}</SelectItem>
                  {groups.filter(g => g.id !== editGroup?.id).map(g => (
                    <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowModal(false)}>{t("groups.cancel")}</Button>
            <Button onClick={handleSave}>{editGroup ? t("groups.save") : t("groups.create_btn")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <Dialog open={deleteId !== null} onOpenChange={(open) => { if (!open) setDeleteId(null); }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("groups.delete_title")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">{t("groups.delete_msg")}</p>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteId(null)}>{t("groups.cancel")}</Button>
            <Button variant="destructive" onClick={handleDelete}>{t("groups.delete")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
