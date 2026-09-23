import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { api, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { usePermissions } from "@/lib/hooks/usePermissions";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from "@/components/ui/dialog";
import { FileCode2, Pencil, Play, Plus, Trash2 } from "lucide-react";

export interface CommandTemplate {
  id: number;
  name: string;
  category: string;
  command: string;
  description?: string;
  created_by?: string;
  created_at?: string;
}

const emptyForm = () => ({ name: "", category: "recon", command: "", description: "" });

/**
 * Command template library CRUD (GET/POST/PUT/DELETE /api/templates).
 * "Run" dispatches the template command to the currently selected toolkit
 * agent via the dual-use /agents/:id/command endpoint.
 */
export default function TemplatesPanel({
  selectedAgent,
  onRun,
}: {
  selectedAgent?: string;
  /** Optional external run hook (parent Toolkit runAction / command dispatch). */
  onRun?: (command: string) => void;
}) {
  const { t } = useI18n();
  const { confirm, modal } = useConfirm();
  const { can } = usePermissions();
  const canWrite = can("agents.write");

  const [templates, setTemplates] = useState<CommandTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState(emptyForm());
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    try {
      const d = await api.get<{ templates?: CommandTemplate[] }>(paths.templates.list);
      setTemplates(d.templates || []);
    } catch {
      setTemplates([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const grouped = useMemo(() => {
    const map = new Map<string, CommandTemplate[]>();
    for (const tpl of templates) {
      const key = tpl.category || "misc";
      const list = map.get(key);
      if (list) list.push(tpl);
      else map.set(key, [tpl]);
    }
    return [...map.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [templates]);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm());
    setEditorOpen(true);
  };

  const openEdit = (tpl: CommandTemplate) => {
    setEditingId(tpl.id);
    setForm({
      name: tpl.name,
      category: tpl.category,
      command: tpl.command,
      description: tpl.description || "",
    });
    setEditorOpen(true);
  };

  const handleSave = async () => {
    if (!form.name.trim() || !form.category.trim() || !form.command.trim()) {
      toast.error(t("templates.toast.fields_required"));
      return;
    }
    setSaving(true);
    try {
      const body = {
        name: form.name.trim(),
        category: form.category.trim(),
        command: form.command.trim(),
        description: form.description.trim(),
      };
      if (editingId != null) await api.putJson(paths.templates.one(editingId), body);
      else await api.postJson(paths.templates.list, body);
      toast.success(editingId != null ? t("templates.toast.updated") : t("templates.toast.created"));
      setEditorOpen(false);
      await load();
    } catch (e) {
      toast.error(formatThrownError(e));
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (tpl: CommandTemplate) => {
    if (!(await confirm({ message: t("templates.confirm_delete", { name: tpl.name }), danger: true }))) return;
    try {
      await api.del(paths.templates.one(tpl.id));
      toast.success(t("templates.toast.deleted"));
      await load();
    } catch (e) {
      toast.error(formatThrownError(e));
    }
  };

  const handleRun = async (tpl: CommandTemplate) => {
    if (onRun) {
      onRun(tpl.command);
      return;
    }
    if (!selectedAgent) {
      toast.error(t("toolkit.toast.select_agent_first"));
      return;
    }
    try {
      await api.postJson(paths.agents.command(selectedAgent), { command: tpl.command });
      toast.success(t("templates.toast.run_dispatched", { name: tpl.name }));
    } catch (e) {
      toast.error(formatThrownError(e));
    }
  };

  return (
    <div className="space-y-4">
      {modal}
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <div>
          <h2 className="text-sm font-semibold text-foreground">{t("templates.title")}</h2>
          <p className="text-xs text-muted-foreground">{t("templates.subtitle")}</p>
        </div>
        {canWrite && (
          <Button size="sm" onClick={openCreate} aria-label={t("templates.create")}>
            <Plus className="size-4" /> {t("templates.create")}
          </Button>
        )}
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16"><Spinner size="sm" /></div>
      ) : templates.length === 0 ? (
        <EmptyState
          icon={FileCode2}
          title={t("templates.empty_title")}
          message={t("templates.empty_message")}
          action={canWrite ? (
            <Button size="sm" onClick={openCreate}><Plus className="size-4" /> {t("templates.create")}</Button>
          ) : undefined}
        />
      ) : (
        <div className="space-y-4">
          {grouped.map(([category, list]) => (
            <Card key={category} className="overflow-hidden">
              <CardHeaderRow
                accent={false}
                title={category}
                action={<Badge variant="secondary" className="text-(--fs-micro-sm) px-2 py-0.5">{list.length}</Badge>}
              />
              <div className="px-4 pb-3 space-y-1">
                {list.map((tpl) => (
                  <div
                    key={tpl.id}
                    className="flex items-start gap-3 px-3 py-2.5 rounded-lg border border-transparent hover:border-border hover:bg-muted/50 transition-colors"
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-xs font-semibold text-foreground">{tpl.name}</span>
                        {tpl.description && (
                          <span className="text-(--fs-micro-sm) text-muted-foreground truncate">{tpl.description}</span>
                        )}
                      </div>
                      <code className="block mt-1 text-(--fs-micro-sm) font-mono text-muted-foreground truncate">
                        {tpl.command}
                      </code>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        onClick={() => void handleRun(tpl)}
                        disabled={!canWrite}
                        title={t("templates.run")}
                        aria-label={t("templates.run")}
                      >
                        <Play className="size-3.5" />
                      </Button>
                      {canWrite && (
                        <>
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            onClick={() => openEdit(tpl)}
                            title={t("common.edit")}
                            aria-label={t("common.edit")}
                          >
                            <Pencil className="size-3.5" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            onClick={() => void handleDelete(tpl)}
                            className="text-muted-foreground hover:text-destructive"
                            title={t("common.delete")}
                            aria-label={t("common.delete")}
                          >
                            <Trash2 className="size-3.5" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{editingId != null ? t("templates.edit_title") : t("templates.create_title")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label htmlFor="tpl-name">{t("common.name")}</Label>
              <Input
                id="tpl-name"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                autoComplete="off"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tpl-cat">{t("templates.field_category")}</Label>
              <Input
                id="tpl-cat"
                value={form.category}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))}
                list="tpl-categories"
                autoComplete="off"
              />
              <datalist id="tpl-categories">
                <option value="recon" />
                <option value="privesc" />
                <option value="lateral" />
                <option value="exfil" />
                <option value="persistence" />
                <option value="credaccess" />
              </datalist>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tpl-cmd">{t("common.command")}</Label>
              <Textarea
                id="tpl-cmd"
                value={form.command}
                onChange={(e) => setForm((f) => ({ ...f, command: e.target.value }))}
                rows={3}
                className="font-mono text-xs"
                spellCheck={false}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tpl-desc">{t("templates.field_description")}</Label>
              <Input
                id="tpl-desc"
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                autoComplete="off"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditorOpen(false)} disabled={saving}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => void handleSave()} disabled={saving}>
              {saving ? <Spinner size="sm" /> : t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
