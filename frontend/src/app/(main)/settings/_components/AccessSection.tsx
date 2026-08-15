"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Layers, Shield, Tags, Users, Wand2 } from "lucide-react";

const LINKS = [
  { href: "/users", labelKey: "settings.access_users", descKey: "settings.access_users_desc", icon: Users },
  { href: "/roles", labelKey: "settings.access_roles", descKey: "settings.access_roles_desc", icon: Shield },
  { href: "/tags", labelKey: "settings.access_tags", descKey: "settings.access_tags_desc", icon: Tags },
  { href: "/groups", labelKey: "settings.access_groups", descKey: "settings.access_groups_desc", icon: Layers },
  { href: "/autotag", labelKey: "settings.access_autotag", descKey: "settings.access_autotag_desc", icon: Wand2 },
] as const;

export default function AccessSection() {
  const { t } = useI18n();
  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t("settings.access_desc")}</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        {LINKS.map((item) => {
          const Icon = item.icon;
          return (
            <Link key={item.href} href={item.href}>
              <Card className="flex items-start gap-3 p-4 hover:bg-secondary/50 transition-colors">
                <Icon className="mt-0.5 h-4 w-4 text-primary shrink-0" />
                <div className="min-w-0">
                  <div className="text-sm font-medium">{t(item.labelKey)}</div>
                  <div className="text-xs text-muted-foreground mt-0.5">{t(item.descKey)}</div>
                </div>
              </Card>
            </Link>
          );
        })}
      </div>
    </div>
  );
}
