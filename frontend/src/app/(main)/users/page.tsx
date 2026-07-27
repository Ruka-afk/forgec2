"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { ConfirmModal, EmptyState, PageHeader, StatCard, StatusBadge, PageSpinner } from "@/components/UI";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Ban, Check, Crown, Key, LogOut, Pencil, Plus, Trash2, User as UserIcon, Users } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface User {
  id?: string;
  username?: string;
  role?: string;
  is_active?: boolean;
  last_activity?: string;
  last_login?: string;
  created_at?: string;
}

function getRoleBadge(role: string) {
  if (role === "admin")
    return { icon: <Crown className="w-2.5 h-2.5" />, key: "users.role_admin", cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300" };
  return { icon: <UserIcon className="w-2.5 h-2.5" />, key: "users.role_user", cls: "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300" };
}

export default function UsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [search, setSearch] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [form, setForm] = useState({ username: "", password: "", role: "user" });
  const [role, setUserRole] = useState("admin");
  const [customRoles, setCustomRoles] = useState<string[]>([]);
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [passwordUserId, setPasswordUserId] = useState<string>("");
  const [newPassword, setNewPassword] = useState("");
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const { t } = useI18n();

  const loadUsers = useCallback(async () => {
    try {
      const data = await api.get("/users") as Record<string, unknown>;
      setUsers((data.users || []) as User[]);
      if (data.UserRole) setUserRole(data.UserRole as string);
    } catch {
      setUsers([]);
      toast.error(t("users.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadUsers();
    api.get("/api/roles")
      .then((d: Record<string, unknown>) => { if (d.success) setCustomRoles(((d.data as unknown[]) || []).map((r: unknown) => (r as { name: string }).name as string)); })
      .catch(() => toast.error(t("users.toast.load_roles_failed")));
  }, [loadUsers, t]);

  const filtered = users.filter((u) => {
    const name = (u.username || "").toLowerCase();
    const urole = (u.role || "").toLowerCase();
    const term = search.toLowerCase();
    return name.includes(term) || urole.includes(term);
  });

  const totalUsers = users.length;
  const adminCount = users.filter((u) => (u.role || "") === "admin").length;
  const userCount = users.filter((u) => (u.role || "") !== "admin").length;
  const activeCount = users.filter((u) => u.is_active === true).length;

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await api.post("/users/add", form);
      toast.success(t("users.toast.created"));
      setShowAdd(false);
      setForm({ username: "", password: "", role: "user" });
      loadUsers();
    } catch { toast.error(t("users.toast.create_failed")); }
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editUser) return;
    try {
      const uid = editUser.id;
      await api.post(`/users/${uid}/edit`, { username: form.username, role: form.role, ...(form.password ? { password: form.password } : {}) });
      toast.success(t("users.toast.updated"));
      setShowEdit(false);
      setEditUser(null);
      loadUsers();
    } catch { toast.error(t("users.toast.update_failed")); }
  };

  const handleToggle = (id: string) => {
    setCfm({msg: t("users.confirm.toggle"), cb: async () => {
      setActionLoading(id + "_toggle");
      try {
        await api.post(`/users/${id}/toggle`, {});
        toast.success(t("users.toast.toggle_success"));
        loadUsers();
      } catch { toast.error(t("users.toast.toggle_failed")); }
      finally { setActionLoading(null); }
    }});
  };

  const handleDelete = (id: string) => {
    setCfm({msg: t("users.confirm.delete"), cb: async () => {
      setActionLoading(id + "_delete");
      try {
        await api.del(`/users/${id}`);
        toast.success(t("users.toast.deleted"));
        loadUsers();
      } catch { toast.error(t("users.toast.delete_failed")); }
      finally { setActionLoading(null); }
    }});
  };

  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!passwordUserId) return;
    try {
      await api.post(`/users/${passwordUserId}/password`, { password: newPassword });
      toast.success(t("users.toast.password_updated"));
      setShowPasswordModal(false);
      setNewPassword("");
      setPasswordUserId("");
    } catch { toast.error(t("users.toast.password_failed")); }
  };

  const handleForceLogout = (id: string) => {
    setCfm({msg: t("users.confirm.force_logout"), cb: async () => {
      try {
        await api.post(`/users/${id}/force-logout`);
        toast.success(t("users.toast.force_logout_success"));
      } catch { toast.error(t("users.toast.force_logout_failed")); }
    }});
  };

  const handleKick = (id: string) => {
    setCfm({msg: t("users.confirm.kick"), cb: async () => {
      try {
        await api.post(`/users/${id}/kick`);
        toast.success(t("users.toast.kick_success"));
      } catch { toast.error(t("users.toast.kick_failed")); }
    }});
  };

  if (loading)
    return <PageSpinner />;

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Users className="w-4 h-4" />{t("users.title")}</>} subtitle={t("users.subtitle")}>
        <div className="flex items-center gap-2">
          <Input placeholder={t("users.search_placeholder")} value={search} onChange={(e) => setSearch(e.target.value)} className="w-48" />
          {role === "admin" && (
            <Button onClick={() => setShowAdd(true)}>
              <Plus className="w-4 h-4" /> {t("users.add_user")}
            </Button>
          )}
        </div>
      </PageHeader>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
        <StatCard label={t("users.stat_total")} value={totalUsers} color="indigo" style={{ animationDelay: "0ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_admins")} value={adminCount} color="indigo" style={{ animationDelay: "40ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_users")} value={userCount} color="blue" style={{ animationDelay: "80ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_active")} value={activeCount} color="emerald" style={{ animationDelay: "120ms" }} className="opacity-0 animate-fade-slide-up" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <Card className="p-4">
          <div className="flex items-center gap-2 mb-2">
            <Crown className="w-4 h-4" />
            <span className="text-sm font-semibold text-indigo-800 dark:text-indigo-300">{t("users.role_admin")}</span>
          </div>
          <ul className="text-xs text-indigo-700 dark:text-indigo-400 space-y-1">
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.admin_desc_1")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.admin_desc_2")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.admin_desc_3")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.admin_desc_4")}</li>
          </ul>
        </Card>
        <Card className="p-4">
          <div className="flex items-center gap-2 mb-2">
            <UserIcon className="w-4 h-4" />
            <span className="text-sm font-semibold text-sky-800 dark:text-sky-300">{t("users.role_user")}</span>
          </div>
          <ul className="text-xs text-sky-700 dark:text-sky-400 space-y-1">
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.user_desc_1")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.user_desc_2")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.user_desc_3")}</li>
            <li className="flex items-start gap-1.5"><Check className="w-4 h-4" />{t("users.user_desc_4")}</li>
          </ul>
        </Card>
      </div>

      <div className="mb-4 flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <div className="text-xs text-muted-foreground">
          {t("users.showing", { filtered: String(filtered.length), total: String(totalUsers) })}
        </div>
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow className="bg-muted border-b border-border text-xs uppercase tracking-wider text-muted-foreground font-semibold">
              <TableHead className="py-3 px-4 sm:py-3.5 font-semibold">{t("users.col_username")}</TableHead>
              <TableHead className="py-3 px-4 sm:py-3.5 font-semibold">{t("users.col_role")}</TableHead>
              <TableHead className="py-3 px-4 sm:py-3.5 font-semibold">{t("users.col_active")}</TableHead>
              <TableHead className="py-3 px-4 sm:py-3.5 font-semibold">{t("users.col_last_activity")}</TableHead>
              <TableHead className="py-3 px-4 sm:py-3.5 font-semibold">{t("users.col_created_at")}</TableHead>
              {role === "admin" && <TableHead className="py-3 px-4 sm:py-3.5 font-semibold text-center">{t("users.col_actions")}</TableHead>}
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.map((u) => {
              const uid = u.id || "";
              const name = u.username || "-";
              const urole = u.role || "user";
              const isActive = u.is_active === true;
              const badge = getRoleBadge(urole);
              return (
                <TableRow key={uid}>
                  <TableCell className="py-3 px-4 sm:py-3.5">
                    <div className="font-medium text-foreground">{name}</div>
                  </TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5">
                    <Badge variant="outline" className={badge.cls}>
                      {badge.icon} {t(badge.key)}
                    </Badge>
                  </TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5">
                    <StatusBadge status={isActive ? "online" : "offline"} />
                  </TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 font-mono text-xs text-muted-foreground">
                    {u.last_activity || u.last_login || "-"}
                  </TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 font-mono text-xs text-muted-foreground">
                    {u.created_at || "-"}
                  </TableCell>
                  {role === "admin" && (
                    <TableCell className="py-3 px-4 sm:py-3.5 text-center">
                      <div className="flex items-center justify-center gap-1">
                        <Button variant="ghost" size="icon-sm" onClick={() => handleToggle(uid)} disabled={actionLoading === uid + "_toggle"} className={`${isActive ? "text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/30" : "text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/30"}`} title={isActive ? t("users.disable") : t("users.enable")} aria-label={isActive ? t("users.disable") : t("users.enable")}>
                          {isActive ? <Ban className="w-4 h-4" /> : <Check className="w-4 h-4" />}
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => { setEditUser(u); setForm({ username: name, password: "", role: urole }); setShowEdit(true); }} className="text-sky-600 dark:text-sky-400 hover:bg-sky-50 dark:hover:bg-sky-900/30" title={t("common.edit")} aria-label={t("common.edit")}>
                          <Pencil className="w-4 h-4" />
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => { setPasswordUserId(uid); setNewPassword(""); setShowPasswordModal(true); }} className="text-primary hover:bg-primary/10" title={t("users.set_password")} aria-label={t("users.set_password")}>
                          <Key className="w-4 h-4" />
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => handleForceLogout(uid)} className="text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/30" title={t("users.force_logout")} aria-label={t("users.force_logout")}>
                          <LogOut className="w-4 h-4" />
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => handleKick(uid)} className="text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-900/30" title={t("users.kick_user")} aria-label={t("users.kick_user")}>
                          <LogOut className="w-4 h-4" />
                        </Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(uid)} disabled={actionLoading === uid + "_delete"} className="text-destructive hover:bg-destructive/10" title={t("common.delete")} aria-label={t("common.delete")}>
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </TableCell>
                  )}
                </TableRow>
              );
            })}
            {filtered.length === 0 && (
              <TableRow>
                <TableCell colSpan={role === "admin" ? 6 : 5} className="py-20 text-center text-muted-foreground">
                  <EmptyState icon={UserIcon} title={t("users.empty_title")} message={t("users.empty_message")} />
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
      </Card>

      <Dialog open={showAdd} onOpenChange={setShowAdd}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("users.add_user")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleAdd} className="space-y-4">
            <div>
              <Label>{t("users.label_username")}</Label>
              <Input type="text" required minLength={3} value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            </div>
            <div>
              <Label>{t("users.label_password")}</Label>
              <Input type="password" required minLength={8} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </div>
            <div>
              <Label>{t("users.label_role")}</Label>
              <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v ?? "user" })}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("users.label_role")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">{t("users.role_user")}</SelectItem>
                  <SelectItem value="admin">{t("users.role_admin")}</SelectItem>
                  {customRoles.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <Button type="submit" className="w-full h-11">{t("users.btn.create_user")}</Button>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showEdit} onOpenChange={setShowEdit}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("users.edit_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleEdit} className="space-y-4">
            <div>
              <Label>{t("users.label_username")}</Label>
              <Input type="text" required minLength={3} value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
            </div>
            <div>
              <Label>{t("users.label_password")}</Label>
              <Input type="password" placeholder={t("users.placeholder.leave_blank")} value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
            </div>
            <div>
              <Label>{t("users.label_role")}</Label>
              <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v ?? "user" })}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("users.label_role")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">{t("users.role_user")}</SelectItem>
                  <SelectItem value="admin">{t("users.role_admin")}</SelectItem>
                  {customRoles.map(r => <SelectItem key={r} value={r}>{r}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button variant="outline" type="button" onClick={() => setShowEdit(false)}>{t("users.btn.cancel")}</Button>
              <Button type="submit">{t("users.btn.save")}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showPasswordModal} onOpenChange={setShowPasswordModal}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>{t("users.password_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleSetPassword} className="space-y-4">
            <div>
              <Label>{t("users.password_label")}</Label>
              <Input type="password" required minLength={8} placeholder={t("users.password_placeholder")} value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
            </div>
            <DialogFooter>
              <Button variant="outline" type="button" onClick={() => setShowPasswordModal(false)}>{t("users.btn.cancel")}</Button>
              <Button type="submit">{t("users.btn.update_password")}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.confirm")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}


