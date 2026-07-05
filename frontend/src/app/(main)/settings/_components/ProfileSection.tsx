import { SettingsData } from "./types";

export default function ProfileSection({ data }: { data: SettingsData }) {
  const currentUsername = data.CurrentUsername || data.current_username || "";
  const currentRole = data.CurrentUserRole || data.current_user_role || "user";
  const getRoleBadge = () => {
    if (currentRole === "admin") return { icon: "fa-crown", text: "Admin", cls: "bg-indigo-100 text-indigo-700" };
    return { icon: "fa-user", text: "User", cls: "bg-sky-100 text-sky-700" };
  };
  const roleBadge = getRoleBadge();

  return (
    <section className="ui-card overflow-hidden">
      <div className="bg-gradient-to-r from-indigo-600 to-indigo-800 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-white/10 rounded-xl flex items-center justify-center"><i className="fa-solid fa-user text-white"></i></div>
          <div><h2 className="text-lg font-semibold text-white">Profile</h2><p className="text-xs text-indigo-200">Current account info</p></div>
        </div>
      </div>
      <div className="p-6">
        <div className="flex items-center gap-6">
          <div className="w-16 h-16 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-2xl flex items-center justify-center text-white text-2xl font-bold shadow-sm">
            <i className="fa-solid fa-user"></i>
          </div>
          <div className="space-y-1.5">
            <div className="text-lg font-semibold text-slate-900 dark:text-slate-100">{currentUsername}</div>
            <div>
              <span className={`inline-flex items-center px-2.5 py-0.5 text-xs font-medium ${roleBadge.cls} rounded-full`}>
                <i className={`fa-solid ${roleBadge.icon} mr-1 text-[10px]`}></i> {roleBadge.text}
              </span>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-6 pt-6 border-t border-slate-100 dark:border-slate-700">
          <div className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400">User ID</div>
            <div className="text-xl font-bold text-slate-800 dark:text-slate-100 mt-1 font-mono text-sm">{data.CurrentUserId ?? data.current_user_id ?? "-"}</div>
          </div>
          <div className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400">Role</div>
            <div className="text-xl font-bold text-slate-800 dark:text-slate-100 mt-1">{currentRole.toUpperCase()}</div>
          </div>
          <div className="bg-slate-50 dark:bg-slate-700/50 rounded-xl p-4 text-center">
            <div className="text-xs text-slate-500 dark:text-slate-400">Server Version</div>
            <div className="text-xl font-bold text-slate-800 dark:text-slate-100 mt-1 font-mono text-sm">v{data.ServerVersion ?? data.server_version ?? "2.0.0"}</div>
          </div>
        </div>
      </div>
    </section>
  );
}
