"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Permission } from "@/components/ui/permission";
import { Layers, Shield, Tags, Users, Wand2 } from "lucide-react";

// perms mirror the per-route enforcement in internal/server/routes.go:
// users→users.read, roles→roles.read, tags→agents.write (tag API group),
// groups→groups.read, autotag→settings.read.
const LINKS = [
  { href: "/users", labelKey: "settings.access_users", descKey: "settings.access_users_desc", icon: Users, perms: ["users.read"] },
  { href: "/roles", labelKey: "settings.access_roles", descKey: "settings.access_roles_desc", icon: Shield, perms: ["roles.read"] },
  { href: "/tags", labelKey: "settings.access_tags", descKey: "settings.access_tags_desc", icon: Tags, perms: ["agents.write"] },
  { href: "/groups", labelKey: "settings.access_groups", descKey: "settings.access_groups_desc", icon: Layers, perms: ["groups.read"] },
  { href: "/autotag", labelKey: "settings.access_autotag", descKey: "settings.access_autotag_desc", icon: Wand2, perms: ["settings.read"] },
] as const;

export default function AccessSection() {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={Users} tone="primary" title={t("settings.access")} description={t("settings.access_desc")} />
      <div className="grid grid-cols-1 gap-3 p-(--card-spacing) sm:grid-cols-2">
        {LINKS.map((item) => {
          const Icon = item.icon;
          return (
            <Permission key={item.href} perms={item.perms}>
              <Link href={item.href} className="group rounded-xl border border-border/75 bg-muted/35 p-4 transition-colors hover:border-primary/30 hover:bg-primary/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50">
                <div className="flex items-start gap-3">
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-card text-primary shadow-xs"><Icon className="size-4" /></span>
                  <div className="min-w-0">
                    <div className="text-sm font-semibold text-foreground group-hover:text-primary">{t(item.labelKey)}</div>
                    <div className="mt-1 text-xs leading-5 text-muted-foreground">{t(item.descKey)}</div>
                  </div>
                </div>
              </Link>
            </Permission>
          );
        })}
      </div>
    </Card>
  );
}
