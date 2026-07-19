import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Crown, User, User as UserIcon } from "lucide-react";

export default function ProfileSection({ data }: { data: SettingsData }) {
  const currentUsername = data.current_username || "";
  const currentRole = data.current_user_role || "user";
  const getRoleBadge = () => {
    if (currentRole === "admin") return { icon: <Crown className="w-2.5 h-2.5" />, text: "Admin", cls: "bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300" };
    return { icon: <UserIcon className="w-2.5 h-2.5" />, text: "User", cls: "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300" };
  };
  const roleBadge = getRoleBadge();

  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-indigo-600 to-indigo-800 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><User className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Profile</h2><p className="text-xs text-indigo-200">Current account info</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-xl flex items-center justify-center text-white text-2xl font-bold shadow-sm">
            <User className="w-4 h-4" />
          </div>
          <div className="space-y-1.5">
            <div className="text-lg font-semibold text-foreground">{currentUsername}</div>
            <div>
              <Badge variant="outline" className={roleBadge.cls}>
                {roleBadge.icon} {roleBadge.text}
              </Badge>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-6 pt-6 border-t border-border">
          <div className="bg-muted rounded-xl p-4 text-center">
            <div className="text-xs text-muted-foreground">User ID</div>
            <div className="text-xl font-bold text-foreground mt-1 font-mono text-sm">{data.current_user_id ?? "-"}</div>
          </div>
          <div className="bg-muted rounded-xl p-4 text-center">
            <div className="text-xs text-muted-foreground">Role</div>
            <div className="text-xl font-bold text-foreground mt-1">{currentRole.toUpperCase()}</div>
          </div>
          <div className="bg-muted rounded-xl p-4 text-center">
            <div className="text-xs text-muted-foreground">Server Version</div>
            <div className="text-xl font-bold text-foreground mt-1 font-mono text-sm">v{data.server_version ?? "2.0.0"}</div>
          </div>
        </div>
      </div>
    </Card>
  );
}

