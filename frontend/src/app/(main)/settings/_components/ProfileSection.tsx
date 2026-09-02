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
    if (currentRole === "admin") return { icon: <Crown className="size-2.5" />, text: t("settings.profile.admin"), cls: "bg-primary/10 text-primary dark:bg-primary/20 dark:text-primary" };
    return { icon: <User className="size-2.5" />, text: t("settings.profile.user"), cls: "bg-info/15 text-info" };
  };
  const roleBadge = getRoleBadge();

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={User} tone="primary" title={t("settings.profile.title")} description={t("settings.profile.subtitle")} />
      <div className="p-(--card-spacing)">
        <div className="flex items-center gap-4">
          <div className="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary ring-1 ring-primary/15">
            <User className="size-7" />
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
        <div className="mt-6 grid grid-cols-1 gap-3 border-t border-border pt-6 sm:grid-cols-3">
          <div className="rounded-xl border border-border/70 bg-muted/45 p-4 text-left">
            <div className="text-xs text-muted-foreground">{t("settings.profile.user_id")}</div>
            <div className="mt-1 font-mono text-sm font-semibold text-foreground">{data.current_user_id ?? "-"}</div>
          </div>
          <div className="rounded-xl border border-border/70 bg-muted/45 p-4 text-left">
            <div className="text-xs text-muted-foreground">{t("settings.profile.role")}</div>
            <div className="mt-1 text-sm font-semibold text-foreground">{currentRole.toUpperCase()}</div>
          </div>
          <div className="rounded-xl border border-border/70 bg-muted/45 p-4 text-left">
            <div className="text-xs text-muted-foreground">{t("settings.profile.server_version")}</div>
            <div className="mt-1 font-mono text-sm font-semibold text-foreground">v{data.server_version ?? "2.0.0"}</div>
          </div>
        </div>
      </div>
    </Card>
  );
}
