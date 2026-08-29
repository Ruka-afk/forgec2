"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { normalizeListEnvelope } from "@/lib/envelope";
import { useAppStore } from "@/lib/store";
import { ErrorState } from "@/components/ui/error-state";
import { FieldError } from "@/components/ui/field-error";
import { Permission } from "@/components/ui/permission";
import { StatCard } from "@/components/ui/animated-stat-card";
import { StatusBadge } from "@/components/ui/status-indicator";
import { PageContainer } from "@/components/ui/page-container";
import { IconBadge } from "@/components/ui/icon-badge";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { usePermissions } from "@/lib/hooks/usePermissions";
import { DataTable } from "@/components/ui/data-table";
import type { DataTableColumn } from "@/components/ui/data-table";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/framework/SearchInput";
import { Label } from "@/components/ui/label";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Skeleton } from "@/components/ui/skeleton";
import { Ban, Check, Crown, Key, Laptop, LogOut, Pencil, Plus, Trash2, User as UserIcon, Users } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { formatTime } from "@/lib/utils";

interface User {
  id?: string;
  username?: string;
  role?: string;
  is_active?: boolean;
  last_activity?: string;
  last_login?: string;
  created_at?: string;
}

interface UserSession {
  id: number;
  ip?: string;
  user_agent?: string;
  device_fingerprint?: string;
  created_at?: string;
  expires_at?: string;
}

function getRoleBadge(role: string) {
  if (role === "admin")
    return { icon: <Crown className="size-2.5" />, key: "users.role_admin", cls: "bg-primary/10 text-primary dark:bg-primary/25 dark:text-primary" };
  return { icon: <UserIcon className="size-2.5" />, key: "users.role_user", cls: "bg-info/10 text-info" };
}

export default function UsersPage() {
  const [search, setSearch] = useState("");
  const [showAdd, setShowAdd] = useState(false);
  const [showEdit, setShowEdit] = useState(false);
  const [editUser, setEditUser] = useState<User | null>(null);
  const [form, setForm] = useState({ username: "", password: "", role: "user" });
  const [formErrors, setFormErrors] = useState<{ username?: string; password?: string }>({});
  const [passwordError, setPasswordError] = useState("");
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [passwordUserId, setPasswordUserId] = useState<string>("");
  const [newPassword, setNewPassword] = useState("");
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const { confirm, modal } = useConfirm();
  const [sessionsUser, setSessionsUser] = useState<User | null>(null);
  const [sessions, setSessions] = useState<UserSession[]>([]);
  const [sessionsLoading, setSessionsLoading] = useState(false);
  const [sessionsError, setSessionsError] = useState<string | null>(null);
  const [revokeLoading, setRevokeLoading] = useState<string | null>(null);
  const { t } = useI18n();

  const { data, loading, error, refresh } = useApiResource<{ users: User[]; customRoles: string[] }>({
    fetcher: async () => {
      const data = await api.get(paths.users.list);
      let customRoles: string[] = [];
      try {
        const d = await api.get<{ success?: boolean; data?: unknown }>(paths.roles.list);
        if (d.success) customRoles = ((d.data as unknown[]) || []).map((r: unknown) => (r as { name: string }).name as string);
      } catch {
        toast.error(t("users.toast.load_roles_failed"));
      }
      return {
        users: normalizeListEnvelope(data, ["users", "Users", "data"]) as User[],
        customRoles,
      };
    },
    toastThrottleMs: 0,
    errorMessage: t("users.toast.load_failed"),
  });
  const users = data?.users ?? [];
  const customRoles = data?.customRoles ?? [];
  const { can } = usePermissions();
  const canManageUsers = can("users.write");
  // Self-protection: demoting/disabling/deleting the logged-in admin's own
  // row is a one-way lockout — hide the destructive controls on it.
  const currentUsername = useAppStore((s) => s.currentUsername);
  const isSelf = (u: User) =>
    !!currentUsername && (u.username || "").toLowerCase() === currentUsername.toLowerCase();

  const columns: DataTableColumn<User>[] = [
    {
      id: "username",
      header: t("users.col_username"),
      sortValue: (u) => u.username || "",
      cell: (u) => <div className="font-medium text-foreground">{u.username || "-"}</div>,
    },
    {
      id: "role",
      header: t("users.col_role"),
      sortValue: (u) => u.role || "user",
      cell: (u) => {
        const badge = getRoleBadge(u.role || "user");
        return (
          <Badge variant="outline" className={badge.cls}>
            {badge.icon} {t(badge.key)}
          </Badge>
        );
      },
    },
    {
      id: "active",
      header: t("users.col_active"),
      cell: (u) => <StatusBadge status={u.is_active === true ? "online" : "offline"} />,
    },
    {
      id: "last_activity",
      header: t("users.col_last_activity"),
      sortValue: (u) => u.last_activity || u.last_login || "",
      cell: (u) => (
        <span className="font-mono text-xs text-muted-foreground">{formatTime(u.last_activity || u.last_login)}</span>
      ),
    },
    {
      id: "created_at",
      header: t("users.col_created_at"),
      sortValue: (u) => u.created_at || "",
      cell: (u) => (
        <span className="font-mono text-xs text-muted-foreground">{formatTime(u.created_at)}</span>
      ),
    },
    ...(canManageUsers
      ? [{
          id: "actions",
          header: t("users.col_actions"),
          align: "center" as const,
          cell: (u: User) => {
            const uid = u.id || "";
            const name = u.username || "-";
            const urole = u.role || "user";
            const isActive = u.is_active === true;
            const self = isSelf(u);
            return (
              <div className="flex items-center justify-center gap-1">
                {!self && (
                  <Button variant="ghost" size="icon-sm" onClick={() => handleToggle(uid)} disabled={actionLoading === uid + "_toggle"} className={`${isActive ? "text-warning hover:bg-warning/10" : "text-success hover:bg-success/10"}`} title={isActive ? t("users.disable") : t("users.enable")} aria-label={isActive ? t("users.disable") : t("users.enable")}>
                    {isActive ? <Ban className="size-4" /> : <Check className="size-4" />}
                  </Button>
                )}
                {!self && (
                  <Button variant="ghost" size="icon-sm" onClick={() => { setEditUser(u); setForm({ username: name, password: "", role: urole }); setShowEdit(true); }} className="text-info hover:bg-info/10" title={t("common.edit")} aria-label={t("common.edit")}>
                    <Pencil className="size-4" />
                  </Button>
                )}
                <Button variant="ghost" size="icon-sm" onClick={() => { setPasswordUserId(uid); setNewPassword(""); setShowPasswordModal(true); }} className="text-primary hover:bg-primary/10" title={t("users.set_password")} aria-label={t("users.set_password")}>
                  <Key className="size-4" />
                </Button>
                <Button variant="ghost" size="icon-sm" onClick={() => openSessions(u)} className="text-info hover:bg-info/10" title={t("users.sessions")} aria-label={t("users.sessions")}>
                  <Laptop className="size-4" />
                </Button>
                {!self && (
                  <>
                    <Button variant="ghost" size="icon-sm" onClick={() => handleForceLogout(uid)} className="text-warning hover:bg-warning/10" title={t("users.force_logout")} aria-label={t("users.force_logout")}>
                      <LogOut className="size-4" />
                    </Button>
                    <Button variant="ghost" size="icon-sm" onClick={() => handleKick(uid)} className="text-destructive hover:bg-destructive/10" title={t("users.kick_user")} aria-label={t("users.kick_user")}>
                      <LogOut className="size-4" />
                    </Button>
                    <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(uid, name)} disabled={actionLoading === uid + "_delete"} className="text-destructive hover:bg-destructive/10" title={t("common.delete")} aria-label={t("common.delete")}>
                      <Trash2 className="size-4" />
                    </Button>
                  </>
                )}
              </div>
            );
          },
        }]
      : []),
  ] as DataTableColumn<User>[];

  const loadUsers = () => {
    void refresh();
  };

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
    const errors: { username?: string; password?: string } = {};
    if (form.username.trim().length < 3) errors.username = t("users.err_username_min");
    if (form.password.length < 8) errors.password = t("users.err_password_min");
    setFormErrors(errors);
    if (Object.keys(errors).length > 0) return;
    try {
      await api.post(paths.users.add, { ...form, username: form.username.trim() });
      toast.success(t("users.toast.created"));
      setShowAdd(false);
      setForm({ username: "", password: "", role: "user" });
      loadUsers();
    } catch { toast.error(t("users.toast.create_failed")); }
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editUser) return;
    const errors: { username?: string; password?: string } = {};
    if (form.username.trim().length < 3) errors.username = t("users.err_username_min");
    if (form.password && form.password.length < 8) errors.password = t("users.err_password_min");
    setFormErrors(errors);
    if (Object.keys(errors).length > 0) return;
    try {
      const uid = editUser.id;
      await api.post(paths.users.edit(String(uid)), { username: form.username, role: form.role, ...(form.password ? { password: form.password } : {}) });
      toast.success(t("users.toast.updated"));
      setShowEdit(false);
      setEditUser(null);
      loadUsers();
    } catch { toast.error(t("users.toast.update_failed")); }
  };

  const handleToggle = async (id: string) => {
    if (!(await confirm({ message: t("users.confirm.toggle") }))) return;
    setActionLoading(id + "_toggle");
    try {
      await api.post(paths.users.toggle(id), {});
      toast.success(t("users.toast.toggle_success"));
      loadUsers();
    } catch { toast.error(t("users.toast.toggle_failed")); }
    finally { setActionLoading(null); }
  };

  const handleDelete = async (id: string, name: string) => {
    if (!(await confirm({ message: t("users.confirm.delete"), requireText: name }))) return;
    setActionLoading(id + "_delete");
    try {
      await api.del(paths.users.one(id));
      toast.success(t("users.toast.deleted"));
      loadUsers();
    } catch { toast.error(t("users.toast.delete_failed")); }
    finally { setActionLoading(null); }
  };

  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!passwordUserId) return;
    if (newPassword.length < 8) {
      setPasswordError(t("users.err_password_min"));
      return;
    }
    try {
      await api.post(paths.users.password(passwordUserId), { password: newPassword });
      toast.success(t("users.toast.password_updated"));
      setShowPasswordModal(false);
      setNewPassword("");
      setPasswordUserId("");
    } catch { toast.error(t("users.toast.password_failed")); }
  };

  const handleForceLogout = async (id: string) => {
    if (!(await confirm({ message: t("users.confirm.force_logout") }))) return;
    try {
      await api.post(paths.users.forceLogout(id));
      toast.success(t("users.toast.force_logout_success"));
    } catch { toast.error(t("users.toast.force_logout_failed")); }
  };

  const handleKick = async (id: string) => {
    if (!(await confirm({ message: t("users.confirm.kick") }))) return;
    try {
      await api.post(paths.users.forceLogout(id));
      toast.success(t("users.toast.kick_success"));
    } catch { toast.error(t("users.toast.kick_failed")); }
  };

  const loadSessions = async (user: User) => {
    const uid = user.id;
    if (!uid) return;
    setSessionsLoading(true);
    setSessionsError(null);
    try {
      const data = await api.get(paths.users.sessions(uid));
      setSessions((data.sessions || data.data || []) as UserSession[]);
    } catch {
      setSessions([]);
      setSessionsError(t("users.toast.sessions_load_failed"));
    } finally {
      setSessionsLoading(false);
    }
  };

  const openSessions = (user: User) => {
    setSessionsUser(user);
    setSessions([]);
    void loadSessions(user);
  };

  const handleRevokeSession = (sessionId: number) => {
    if (!sessionsUser?.id) return;
    setRevokeLoading("s" + sessionId);
    api.post(paths.users.revokeSession(sessionsUser.id, sessionId), {})
      .then(() => {
        toast.success(t("users.toast.session_revoked"));
        setSessions((prev) => prev.filter((s) => s.id !== sessionId));
      })
      .catch(() => toast.error(t("users.toast.session_revoke_failed")))
      .finally(() => setRevokeLoading(null));
  };

  const handleRevokeAll = async () => {
    const uid = sessionsUser?.id;
    if (!uid || !sessionsUser?.username) return;
    if (!(await confirm({ message: t("users.confirm.revoke_all", { name: sessionsUser.username }) }))) return;
    setRevokeLoading("all");
    try {
      await api.post(paths.users.revokeAllSessions(uid), {});
      toast.success(t("users.toast.sessions_revoked"));
      setSessions([]);
    } catch { toast.error(t("users.toast.sessions_revoke_failed")); }
    finally { setRevokeLoading(null); }
  };

  return (
    <Permission perms="users.read" fallback={
      <PageContainer title={t("users.title")} icon={<Users className="size-4" />} subtitle={t("users.subtitle")}>
        <ErrorState title={t("common.denied_title")} message={t("common.denied_desc")} />
      </PageContainer>
    }>
      <PageContainer title={t("users.title")} icon={<Users className="size-4" />} subtitle={t("users.subtitle")} actions={<>
        <div className="flex items-center gap-2 flex-wrap">
          <SearchInput value={search} onChange={setSearch} placeholder={t("users.search_placeholder")} className="w-40 sm:w-48" label={t("common.search")} />
          {canManageUsers && (
            <Button onClick={() => setShowAdd(true)}>
              <Plus className="size-4" /> {t("users.add_user")}
            </Button>
          )}
        </div>
      </>}>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
        <StatCard label={t("users.stat_total")} value={totalUsers} color="indigo" style={{ animationDelay: "0ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_admins")} value={adminCount} color="indigo" style={{ animationDelay: "40ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_users")} value={userCount} color="blue" style={{ animationDelay: "80ms" }} className="opacity-0 animate-fade-slide-up" />
        <StatCard label={t("users.stat_active")} value={activeCount} color="emerald" style={{ animationDelay: "120ms" }} className="opacity-0 animate-fade-slide-up" />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <Card className="p-4 shadow-sm hover:shadow-md transition-shadow duration-200">
          <div className="flex items-center gap-2 mb-2">
            <Crown className="size-4" />
            <span className="text-sm font-semibold text-primary dark:text-primary">{t("users.role_admin")}</span>
          </div>
          <ul className="text-xs text-primary space-y-1">
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.admin_desc_1")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.admin_desc_2")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.admin_desc_3")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.admin_desc_4")}</li>
          </ul>
        </Card>
        <Card className="p-4 shadow-sm hover:shadow-md transition-shadow duration-200">
          <div className="flex items-center gap-2 mb-2">
            <UserIcon className="size-4" />
            <span className="text-sm font-semibold text-info">{t("users.role_user")}</span>
          </div>
          <ul className="text-xs text-muted-foreground space-y-1">
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.user_desc_1")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.user_desc_2")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.user_desc_3")}</li>
            <li className="flex items-start gap-1.5"><Check className="size-4" />{t("users.user_desc_4")}</li>
          </ul>
        </Card>
      </div>

      <div className="mb-4 flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <div className="text-xs text-muted-foreground">
          {t("users.showing", { filtered: String(filtered.length), total: String(totalUsers) })}
        </div>
      </div>

      <Card className="overflow-hidden">
        <DataTable<User>
          data={filtered}
          loading={loading}
          error={error}
          onRetry={() => void loadUsers()}
          emptyTitle={t("users.empty_title")}
          emptyMessage={t("users.empty_message")}
          emptyIcon={UserIcon}
          columns={columns}
          rowKey={(u) => u.id || u.username || ""}
          defaultSort={{ column: "username", dir: "asc" }}
        />
      </Card>

      <Dialog open={showAdd} onOpenChange={setShowAdd}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("users.add_user")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleAdd} className="space-y-4">
            <div>
              <Label htmlFor="add-username">{t("users.label_username")}</Label>
              <Input id="add-username" type="text" required minLength={3} value={form.username} onChange={(e) => { setForm({ ...form, username: e.target.value }); if (formErrors.username) setFormErrors({ ...formErrors, username: undefined }); }} />
              <FieldError>{formErrors.username}</FieldError>
            </div>
            <div>
              <Label htmlFor="add-password">{t("users.label_password")}</Label>
              <Input id="add-password" type="password" required minLength={8} value={form.password} onChange={(e) => { setForm({ ...form, password: e.target.value }); if (formErrors.password) setFormErrors({ ...formErrors, password: undefined }); }} />
              <FieldError>{formErrors.password}</FieldError>
            </div>
            <div>
              <Label htmlFor="add-role">{t("users.label_role")}</Label>
              <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v ?? "user" })}>
                <SelectTrigger id="add-role" className="w-full">
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
              <Label htmlFor="edit-username">{t("users.label_username")}</Label>
              <Input id="edit-username" type="text" required minLength={3} value={form.username} onChange={(e) => { setForm({ ...form, username: e.target.value }); if (formErrors.username) setFormErrors({ ...formErrors, username: undefined }); }} />
              <FieldError>{formErrors.username}</FieldError>
            </div>
            <div>
              <Label htmlFor="edit-password">{t("users.label_password")}</Label>
              <Input id="edit-password" type="password" placeholder={t("users.placeholder.leave_blank")} value={form.password} onChange={(e) => { setForm({ ...form, password: e.target.value }); if (formErrors.password) setFormErrors({ ...formErrors, password: undefined }); }} />
              <FieldError>{formErrors.password}</FieldError>
            </div>
            <div>
              <Label htmlFor="edit-role">{t("users.label_role")}</Label>
              <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v ?? "user" })}>
                <SelectTrigger id="edit-role" className="w-full">
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
              <Label htmlFor="pw-new">{t("users.password_label")}</Label>
              <Input id="pw-new" type="password" required minLength={8} placeholder={t("users.password_placeholder")} value={newPassword} onChange={(e) => { setNewPassword(e.target.value); if (passwordError) setPasswordError(""); }} />
              <FieldError>{passwordError}</FieldError>
            </div>
            <DialogFooter>
              <Button variant="outline" type="button" onClick={() => setShowPasswordModal(false)}>{t("users.btn.cancel")}</Button>
              <Button type="submit">{t("users.btn.update_password")}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={!!sessionsUser} onOpenChange={(v) => { if (!v) { setSessionsUser(null); setSessions([]); } }}>
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>{t("users.sessions_title")}</DialogTitle>
            {sessionsUser?.username && <p className="text-xs text-muted-foreground">{t("users.sessions_subtitle", { name: sessionsUser.username })}</p>}
          </DialogHeader>
          <div className="flex items-center justify-between">
            <span className="text-xs text-muted-foreground">{t("users.showing", { filtered: String(sessions.length), total: String(sessions.length) })}</span>
            <Button variant="destructive" size="sm" onClick={handleRevokeAll} disabled={sessions.length === 0 || revokeLoading !== null}>
              <LogOut className="size-4" /> {t("users.revoke_all_sessions")}
            </Button>
          </div>
          {sessionsError && <p className="text-xs text-destructive">{sessionsError}</p>}
          {sessionsLoading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} className="h-12 w-full" />)}
            </div>
          ) : sessions.length === 0 ? (
            <EmptyState compact icon={Laptop} title={t("users.sessions_empty")} />
          ) : (
            <ScrollArea className="max-h-80">
            <div className="space-y-2">
              {sessions.map((s) => (
                <div key={s.id} className="flex items-center justify-between gap-3 p-3 rounded-lg border border-border bg-card/60">
                  <div className="flex items-center gap-3 min-w-0">
                    <IconBadge icon={Laptop} color="muted" size="md" />
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-medium text-foreground font-mono">{s.ip || "-"}</span>
                        <span className="text-xs text-muted-foreground">{t("users.col_created")}: {formatTime(s.created_at)}</span>
                      </div>
                      <div className="text-xs text-muted-foreground truncate max-w-md">
                        {(s.user_agent || "-")}{s.device_fingerprint ? ` · ${s.device_fingerprint}` : ""}
                      </div>
                      <div className="text-xs text-muted-foreground/100">
                        {t("users.col_expires")}: {formatTime(s.expires_at)}
                      </div>
                    </div>
                  </div>
                  <Button variant="ghost" size="sm" onClick={() => handleRevokeSession(s.id)} disabled={revokeLoading !== null} className="text-destructive hover:bg-destructive/10 shrink-0">
                    <LogOut className="size-4" /> {t("users.revoke_session")}
                  </Button>
                </div>
              ))}
            </div>
            </ScrollArea>
          )}
        </DialogContent>
      </Dialog>
      {modal}
    </PageContainer>
    </Permission>
  );
}


