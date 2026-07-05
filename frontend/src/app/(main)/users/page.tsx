"use client";

import { useEffect, useState, useCallback } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal, PageHeader, SearchInput } from "@/components/UI";

interface User {
  ID?: string;
  id?: string;
  Username?: string;
  username?: string;
  Role?: string;
  role?: string;
  IsActive?: boolean | string;
  is_active?: boolean;
  LastActivity?: string;
  LastLogin?: string;
  CreatedAt?: string;
}

interface ToastNotification {
  msg: string;
  type: "success" | "error" | "info";
}

function getRoleBadge(role: string) {
  if (role === "admin")
    return { icon: "fa-crown", text: "Admin", cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300" };
  return { icon: "fa-user", text: "User", cls: "bg-sky-100 text-sky-700 dark:bg-sky-900/40 dark:text-sky-300" };
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
  const [showPasswordModal, setShowPasswordModal] = useState(false);
  const [passwordUserId, setPasswordUserId] = useState<string>("");
  const [newPassword, setNewPassword] = useState("");
  const [toasts, setToasts] = useState<ToastNotification[]>([]);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const showToast = useCallback((msg: string, type: "success" | "error" | "info" = "info") => {
    setToasts((prev) => [...prev, { msg, type }]);
    setTimeout(() => {
      setToasts((prev) => prev.slice(1));
    }, 3000);
  }, []);

  const loadUsers = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/users&format=json`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      setUsers(data.users || data.Users || []);
      if (data.UserRole) setUserRole(data.UserRole);
    } catch {
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { Promise.resolve().then(() => loadUsers()); }, [loadUsers]);

  const filtered = users.filter((u) => {
    const name = (u.Username || u.username || "").toLowerCase();
    const urole = (u.Role || u.role || "").toLowerCase();
    const term = search.toLowerCase();
    return name.includes(term) || urole.includes(term);
  });

  const totalUsers = users.length;
  const adminCount = users.filter((u) => (u.Role || u.role || "") === "admin").length;
  const userCount = users.filter((u) => (u.Role || u.role || "") !== "admin").length;
  const activeCount = users.filter((u) => u.IsActive === true || u.IsActive === "true" || u.is_active === true).length;

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await fetch(`${API_BASE}?p=/users/add&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams(form).toString(),
      });
      showToast("User created successfully", "success");
      setShowAdd(false);
      setForm({ username: "", password: "", role: "user" });
      loadUsers();
    } catch { showToast("Failed to create user", "error"); }
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editUser) return;
    try {
      const uid = editUser.ID || editUser.id;
      await fetch(`${API_BASE}?p=/users/${uid}&format=json`, {
        method: "PUT",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ username: form.username, role: form.role, ...(form.password ? { password: form.password } : {}) }).toString(),
      });
      showToast("User updated successfully", "success");
      setShowEdit(false);
      setEditUser(null);
      loadUsers();
    } catch { showToast("Failed to update user", "error"); }
  };

  const handleToggle = (id: string) => {
    setCfm({msg: "确认切换用户状态？", cb: async () => {
      setActionLoading(id + "_toggle");
      try {
        await fetch(`${API_BASE}?p=/users/${id}/toggle&format=json`, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: new URLSearchParams({}).toString(),
        });
        showToast("User status toggled", "success");
        loadUsers();
      } catch { showToast("Failed to toggle user status", "error"); }
      finally { setActionLoading(null); }
    }});
  };

  const handleDelete = (id: string) => {
    setCfm({msg: "确认删除该用户？此操作不可恢复！", cb: async () => {
      setActionLoading(id + "_delete");
      try {
        await fetch(`${API_BASE}?p=/users/${id}&format=json`, { method: "DELETE" });
        showToast("User deleted", "success");
        loadUsers();
      } catch { showToast("Failed to delete user", "error"); }
      finally { setActionLoading(null); }
    }});
  };

  const handleSetPassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!passwordUserId) return;
    try {
      await fetch(`${API_BASE}?p=/users/${passwordUserId}/password&format=json`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: new URLSearchParams({ password: newPassword }).toString(),
      });
      showToast("Password updated", "success");
      setShowPasswordModal(false);
      setNewPassword("");
      setPasswordUserId("");
    } catch { showToast("Failed to set password", "error"); }
  };

  const handleForceLogout = (id: string) => {
    setCfm({msg: "Force logout this user?", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/users/${id}/force-logout&format=json`, { method: "POST" });
        showToast("User force logged out", "success");
      } catch { showToast("Failed to force logout", "error"); }
    }});
  };

  const handleKick = (id: string) => {
    setCfm({msg: "Kick this user from all sessions?", cb: async () => {
      try {
        await fetch(`${API_BASE}?p=/users/${id}/kick&format=json`, { method: "POST" });
        showToast("User kicked", "success");
      } catch { showToast("Failed to kick user", "error"); }
    }});
  };

  if (loading)
    return (
      <div className="flex items-center justify-center h-64">
        <i className="fa-solid fa-circle-notch fa-spin text-3xl text-indigo-500"></i>
      </div>
    );

  return (
    <div className="max-w-6xl mx-auto mb-20 md:mb-0">
      {toasts.length > 0 && (
        <div className="fixed top-4 right-4 z-[100] space-y-2">
          {toasts.map((t, i) => (
            <div key={i} className={`px-4 py-3 rounded-2xl shadow-lg text-sm font-medium text-white ${t.type === "success" ? "bg-emerald-600" : t.type === "error" ? "bg-red-600" : "bg-indigo-600"}`}>
              <i className={`fa-solid ${t.type === "success" ? "fa-check-circle" : t.type === "error" ? "fa-exclamation-circle" : "fa-info-circle"} mr-2`}></i>
              {t.msg}
            </div>
          ))}
        </div>
      )}

      <PageHeader title={<><i className="fa-solid fa-users text-indigo-500 mr-2"></i>User Management</>} subtitle="Manage users and roles">
        {role === "admin" && (
          <button onClick={() => setShowAdd(true)} className="bg-indigo-600 hover:bg-indigo-700 text-white px-4 h-9 rounded-2xl text-sm font-medium flex items-center gap-x-2 transition-colors">
            <i className="fa-solid fa-plus"></i> Add User
          </button>
        )}
      </PageHeader>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-6">
        <div className="ui-card p-3 shadow-sm text-center">
          <div className="text-2xl font-bold text-slate-900 dark:text-slate-100">{totalUsers}</div>
          <div className="text-[10px] text-slate-500 dark:text-slate-400 uppercase tracking-wider">Total</div>
        </div>
        <div className="ui-card p-3 shadow-sm text-center">
          <div className="text-2xl font-bold text-indigo-700 dark:text-indigo-300">{adminCount}</div>
          <div className="text-[10px] text-indigo-600 dark:text-indigo-400 uppercase tracking-wider">Admins</div>
        </div>
        <div className="ui-card p-3 shadow-sm text-center">
          <div className="text-2xl font-bold text-sky-700 dark:text-sky-300">{userCount}</div>
          <div className="text-[10px] text-sky-600 dark:text-sky-400 uppercase tracking-wider">Users</div>
        </div>
        <div className="ui-card p-3 shadow-sm text-center">
          <div className="text-2xl font-bold text-emerald-700 dark:text-emerald-300">{activeCount}</div>
          <div className="text-[10px] text-emerald-600 dark:text-emerald-400 uppercase tracking-wider">Active</div>
        </div>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <div className="ui-card p-4">
          <div className="flex items-center gap-2 mb-2">
            <i className="fa-solid fa-crown text-indigo-600 text-sm"></i>
            <span className="text-sm font-semibold text-indigo-800 dark:text-indigo-300">Admin</span>
          </div>
          <ul className="text-xs text-indigo-700 dark:text-indigo-400 space-y-1">
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Full control over all features</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Manage users, roles, permissions</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Server configuration and maintenance</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Lock/unlock agents</li>
          </ul>
        </div>
        <div className="ui-card p-4">
          <div className="flex items-center gap-2 mb-2">
            <i className="fa-solid fa-user text-sky-600 text-sm"></i>
            <span className="text-sm font-semibold text-sky-800 dark:text-sky-300">User</span>
          </div>
          <ul className="text-xs text-sky-700 dark:text-sky-400 space-y-1">
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Send commands, execute modules</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>File browsing, credential harvesting</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Generate payloads, create listeners</li>
            <li className="flex items-start gap-1.5"><i className="fa-solid fa-check mt-0.5 text-[10px]"></i>Lock agents for exclusive ops</li>
          </ul>
        </div>
      </div>

      <div className="mb-4 flex flex-col sm:flex-row gap-3 items-start sm:items-center">
        <SearchInput value={search} onChange={setSearch} placeholder="Search by username or role..." className="flex-1 max-w-md" />
        <div className="text-xs text-slate-500 dark:text-slate-400">
          Showing {filtered.length} of {totalUsers} users
        </div>
      </div>

      <div className="ui-card rounded-3xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-700/50 border-b border-[var(--border)]">
              <tr className="text-xs text-slate-500 dark:text-slate-400">
                <th className="text-left py-4 px-6 font-normal">Username</th>
                <th className="text-left py-4 px-4 font-normal">Role</th>
                <th className="text-left py-4 px-4 font-normal">Active</th>
                <th className="text-left py-4 px-4 font-normal">Last Activity</th>
                <th className="text-left py-4 px-4 font-normal">Created At</th>
                {role === "admin" && <th className="text-center py-4 px-4 font-normal">Actions</th>}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-700">
              {filtered.map((u) => {
                const uid = u.ID || u.id || "";
                const name = u.Username || u.username || "-";
                const urole = u.Role || u.role || "user";
                const isActive = u.IsActive === true || u.IsActive === "true" || u.is_active === true;
                const badge = getRoleBadge(urole);
                return (
                  <tr key={uid} className="hover:bg-slate-50 dark:hover:bg-slate-700/50 transition-colors">
                    <td className="py-4 px-6">
                      <div className="font-medium text-slate-900 dark:text-slate-100">{name}</div>
                    </td>
                    <td className="py-4 px-4">
                      <span className={`inline-flex items-center px-2.5 py-1 text-xs font-medium ${badge.cls} rounded-full`}>
                        <i className={`fa-solid ${badge.icon} mr-1 text-[10px]`}></i> {badge.text}
                      </span>
                    </td>
                    <td className="py-4 px-4">
                      {isActive ? (
                        <span className="inline-flex items-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400">
                          <span className="w-2 h-2 bg-emerald-500 rounded-full"></span> Yes
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1.5 text-xs text-red-600 dark:text-red-400">
                          <span className="w-2 h-2 bg-red-500 rounded-full"></span> No
                        </span>
                      )}
                    </td>
                    <td className="py-4 px-4 font-mono text-xs text-slate-500 dark:text-slate-400">
                      {u.LastActivity || u.LastLogin || "-"}
                    </td>
                    <td className="py-4 px-4 font-mono text-xs text-slate-500 dark:text-slate-400">
                      {u.CreatedAt || "-"}
                    </td>
                    {role === "admin" && (
                      <td className="py-4 px-4 text-center">
                        <div className="flex items-center justify-center gap-1">
                          <button onClick={() => handleToggle(uid)} disabled={actionLoading === uid + "_toggle"} className={`p-1.5 rounded-lg transition-colors ${isActive ? "text-amber-600 dark:text-amber-400 hover:bg-amber-50 dark:hover:bg-amber-900/30" : "text-emerald-600 dark:text-emerald-400 hover:bg-emerald-50 dark:hover:bg-emerald-900/30"}`} title={isActive ? "Disable" : "Enable"}>
                            <i className={`fa-solid ${isActive ? "fa-ban" : "fa-check"}`}></i>
                          </button>
                          <button onClick={() => { setEditUser(u); setForm({ username: name, password: "", role: urole }); setShowEdit(true); }} className="p-1.5 text-sky-600 dark:text-sky-400 hover:bg-sky-50 dark:hover:bg-sky-900/30 rounded-lg transition-colors" title="Edit">
                            <i className="fa-solid fa-pen"></i>
                          </button>
                          <button onClick={() => { setPasswordUserId(uid); setNewPassword(""); setShowPasswordModal(true); }} className="p-1.5 text-purple-600 dark:text-purple-400 hover:bg-purple-50 dark:hover:bg-purple-900/30 rounded-lg transition-colors" title="Set Password">
                            <i className="fa-solid fa-key"></i>
                          </button>
                          <button onClick={() => handleForceLogout(uid)} className="p-1.5 text-orange-600 dark:text-orange-400 hover:bg-orange-50 dark:hover:bg-orange-900/30 rounded-lg transition-colors" title="Force Logout">
                            <i className="fa-solid fa-right-from-bracket"></i>
                          </button>
                          <button onClick={() => handleKick(uid)} className="p-1.5 text-rose-600 dark:text-rose-400 hover:bg-rose-50 dark:hover:bg-rose-900/30 rounded-lg transition-colors" title="Kick User">
                            <i className="fa-solid fa-person-walking-arrow-loop-left"></i>
                          </button>
                          <button onClick={() => handleDelete(uid)} disabled={actionLoading === uid + "_delete"} className="p-1.5 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-lg transition-colors" title="Delete">
                            <i className="fa-solid fa-trash"></i>
                          </button>
                        </div>
                      </td>
                    )}
                  </tr>
                );
              })}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={role === "admin" ? 6 : 5} className="py-20 text-center text-slate-400 dark:text-slate-500">
                    No users found
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      {showAdd && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowAdd(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md mx-4 p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">Add User</div>
              <button onClick={() => setShowAdd(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <form onSubmit={handleAdd} className="space-y-4">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Username</label>
                <input type="text" required minLength={3} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Password</label>
                <input type="password" required minLength={8} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Role</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <button type="submit" className="w-full h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl font-medium transition-colors">Create User</button>
            </form>
          </div>
        </div>
      )}

      {showEdit && editUser && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowEdit(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md mx-4 p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">Edit User</div>
              <button onClick={() => setShowEdit(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <form onSubmit={handleEdit} className="space-y-4">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Username</label>
                <input type="text" minLength={3} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Password</label>
                <input type="password" className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" placeholder="Leave blank to keep unchanged" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
              </div>
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">Role</label>
                <select className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })}>
                  <option value="user">User</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <div className="flex gap-3">
                <button type="button" onClick={() => setShowEdit(false)} className="flex-1 h-11 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 text-slate-600 dark:text-slate-300 rounded-2xl font-medium text-sm transition-colors">Cancel</button>
                <button type="submit" className="flex-1 h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl font-medium text-sm transition-colors">Save</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showPasswordModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 p-4" onClick={() => setShowPasswordModal(false)}>
          <div className="bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-sm mx-4 p-6" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">Set Password</div>
              <button onClick={() => setShowPasswordModal(false)} className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-700"><i className="fa-solid fa-xmark"></i></button>
            </div>
            <form onSubmit={handleSetPassword} className="space-y-4">
              <div>
                <label className="block text-xs text-slate-500 dark:text-slate-400 mb-1">New Password</label>
                <input type="password" required minLength={8} className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] focus:border-indigo-500 rounded-2xl px-4 h-11 text-sm focus:outline-none" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="Minimum 8 characters" />
              </div>
              <div className="flex gap-3">
                <button type="button" onClick={() => setShowPasswordModal(false)} className="flex-1 h-11 bg-slate-100 dark:bg-slate-700 hover:bg-slate-200 text-slate-600 dark:text-slate-300 rounded-2xl font-medium text-sm transition-colors">Cancel</button>
                <button type="submit" className="flex-1 h-11 bg-indigo-600 hover:bg-indigo-700 text-white rounded-2xl font-medium text-sm transition-colors">Update Password</button>
              </div>
            </form>
          </div>
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Confirm" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
