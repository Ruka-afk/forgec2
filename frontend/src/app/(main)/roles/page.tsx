"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EmptyState, PageSpinner } from "@/components/UI";
import { PageContainer } from "@/components/ui/page-container";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Pencil, Plus, ShieldUser, Trash2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";

interface Role {
  id: number;
  name: string;
  description: string;
  permissions: string[];
  created_at: string;
  updated_at: string;
}

const PERM_GROUPS: Record<string, string[]> = {
  "Agents": ["agents.read", "agents.write", "agents.delete"],
  "Listeners": ["listeners.read", "listeners.write", "listeners.delete"],
  "Tasks": ["tasks.read", "tasks.write", "tasks.delete"],
  "Credentials": ["credentials.read", "credentials.write", "credentials.delete"],
  "Files": ["files.read", "files.write"],
  "Users": ["users.read", "users.write", "users.delete"],
  "Settings": ["settings.read", "settings.write"],
  "Audit": ["audit.read"],
  "Groups": ["groups.read", "groups.write"],
  "Plugins": ["plugins.read", "plugins.write", "plugins.execute", "plugins.delete"],
  "Roles": ["roles.read", "roles.write"],
  "Campaigns": ["campaigns.read", "campaigns.write"],
  "OPSEC": ["opsec.read", "opsec.write"],
  "Intel": ["intel.read", "intel.write"],
  "Automation": ["automation.read", "automation.write"],
  "Notifications": ["notifications.read", "notifications.write"],
};

export default function RolesPage() {
  const { t } = useI18n();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [editRole, setEditRole] = useState<Role | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const { confirm, modal } = useConfirm();
  const [newPerms, setNewPerms] = useState<string[]>([]);

  const loadRoles = useCallback(async () => {
    try {
      const data = await api.get<{success: boolean; data?: Role[]}>(paths.roles.list);
      if (data.success) setRoles(data.data || []);
    } catch {
      toast.error(t("roles.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { loadRoles(); }, [loadRoles]);

  const handleCreate = async () => {
    if (!newName.trim()) return;
    await api.postJson(paths.roles.list, { name: newName, description: newDesc, permissions: newPerms });
    setNewName(""); setNewDesc(""); setNewPerms([]); setShowCreate(false);
    loadRoles();
    toast.success(t("roles.created"));
  };

  const handleUpdate = async (role: Role) => {
    await api.postJson(paths.roles.one(role.id), {
      name: role.name,
      description: role.description,
      permissions: role.permissions,
    });
    setEditRole(null);
    loadRoles();
    toast.success(t("roles.updated"));
  };

  const handleDelete = async (id: number) => {
    if (!(await confirm({ message: t("roles.delete_confirm") }))) return;
    try {
      await api.del(paths.roles.one(id));
      toast.success(t("roles.deleted"));
      loadRoles();
    } catch {
      toast.error(t("roles.delete_failed"));
    }
  };

  const togglePerm = (perms: string[], perm: string): string[] => {
    return perms.includes(perm) ? perms.filter(p => p !== perm) : [...perms, perm];
  };

  if (loading) return <PageSpinner />;

  return (
    <>
      <PageContainer title={<><ShieldUser className="w-4 h-4" />{t("roles.title")}</>} subtitle={t("roles.subtitle")} actions={<>
          <Button onClick={() => setShowCreate(true)}>
            <Plus className="w-4 h-4" />{t("roles.new_role")}
          </Button>
        </>}>

        <Dialog open={showCreate} onOpenChange={setShowCreate}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("roles.create_title")}</DialogTitle>
            </DialogHeader>
            <div className="space-y-3">
              <Input aria-label={t("roles.a11y_name")} placeholder={t("roles.name_ph")} value={newName}
                onChange={(e) => setNewName(e.target.value)} />
              <Textarea aria-label={t("roles.a11y_desc_short")} rows={2} placeholder={t("roles.desc_ph")} value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)} />
              <PermSelector selected={newPerms} onToggle={(p) => setNewPerms(togglePerm(newPerms, p))} />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setShowCreate(false)}>{t("common.cancel")}</Button>
              <Button onClick={handleCreate}>{t("roles.create")}</Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <div className="grid gap-4">
          {roles.length === 0 && (
            <EmptyState title={t("roles.empty")} />
          )}
          {roles.map((role) => (
            <Card key={role.id} className="p-4 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30 transition-all cursor-pointer">
              {editRole?.id === role.id ? (
                <div className="space-y-3">
                  <Input aria-label={t("roles.a11y_name")} value={editRole.name}
                    onChange={(e) => setEditRole({ ...editRole, name: e.target.value })} />
                  <Textarea aria-label={t("roles.a11y_desc")} rows={2} value={editRole.description}
                    onChange={(e) => setEditRole({ ...editRole, description: e.target.value })} />
                  <PermSelector selected={editRole.permissions}
                    onToggle={(p) => setEditRole({ ...editRole, permissions: togglePerm(editRole.permissions, p) })} />
                  <div className="flex gap-2">
                    <Button onClick={() => handleUpdate(editRole)}>{t("common.save")}</Button>
                    <Button variant="outline" onClick={() => setEditRole(null)}>{t("common.cancel")}</Button>
                  </div>
                </div>
              ) : (
                <>
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="font-semibold text-lg">{role.name}</h3>
                      {role.description && <p className="text-sm text-muted-foreground">{role.description}</p>}
                    </div>
                    <div className="flex gap-2">
                      <Button variant="outline" size="sm" onClick={() => {
                        setEditRole({ ...role, permissions: [...role.permissions] });
                      }}>
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button variant="destructive" size="sm" onClick={() => handleDelete(role.id)}>
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {role.permissions.map((p) => (
                      <Badge key={p} variant="secondary">{p}</Badge>
                    ))}
                  </div>
                </>
              )}
            </Card>
          ))}
        </div>
      </PageContainer>
      {modal}
    </>
  );
}

function PermSelector({ selected, onToggle }: { selected: string[]; onToggle: (perm: string) => void }) {
  const { t } = useI18n();
  return (
    <div className="space-y-2">
      <p className="text-sm font-medium">{t("roles.permissions")}</p>
      {Object.entries(PERM_GROUPS).map(([group, perms]) => (
        <div key={group}>
           <p className="text-xs text-muted-foreground mb-1">{group}</p>
           <div className="flex flex-wrap gap-2">
             {perms.map((p) => (
               <Label key={p} className="flex items-center gap-1.5 text-sm cursor-pointer">
                 <Checkbox checked={selected.includes(p)}
                   onCheckedChange={() => onToggle(p)} />
                {p}
              </Label>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

