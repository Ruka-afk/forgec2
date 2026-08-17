import { SettingsData } from "./types";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Badge } from "@/components/ui/badge";
import { Crown, User } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function ProfileSection({ data }: { data: SettingsData }) {
  const { t } = useI18n();
  const currentUsername = data.current_username || "";
  const currentRole = data.current_user_role || "user";
  const getRoleBadge = () => {
    if (currentRole === "admin") return { icon: <Crown className="w-2.5 h-2.5" />, text: t("settings.profile.admin"), cls: "bg-primary/10 text-primary dark:bg-primary/20 dark:text-primary" };
    return { icon: <User className="w-2.5 h-2.5" />, text: t("settings.profile.user"), cls: "bg-info/15 text-info" };
  };
  const roleBadge = getRoleBadge();

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={User} tone="primary" title={t("settings.profile.title")} description={t("settings.profile.subtitle")} />
      <div className="p-4 sm:p-5">
        <div className="flex items-center gap-4">
          <div className="w-16 h-16 bg-primary/10 text-primary ring-1 ring-border/50 rounded-lg flex items-center justify-center text-2xl font-bold">
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
          <div className="bg-muted rounded-lg p-4 text-center">
            <div className="text-xs text-muted-foreground">{t("settings.profile.user_id")}</div>
            <div className="text-xl font-bold text-foreground mt-1 font-mono text-sm">{data.current_user_id ?? "-"}</div>
          </div>
          <div className="bg-muted rounded-lg p-4 text-center">
            <div className="text-xs text-muted-foreground">{t("settings.profile.role")}</div>
            <div className="text-xl font-bold text-foreground mt-1">{currentRole.toUpperCase()}</div>
          </div>
          <div className="bg-muted rounded-lg p-4 text-center">
            <div className="text-xs text-muted-foreground">{t("settings.profile.server_version")}</div>
            <div className="text-xl font-bold text-foreground mt-1 font-mono text-sm">v{data.server_version ?? "2.0.0"}</div>
          </div>
        </div>
      </div>
    </Card>
  );
}
