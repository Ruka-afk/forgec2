"use client";

import { useRouter } from "next/navigation";
import { useI18n } from "@/lib/i18n";
import { useAppStore } from "@/lib/store";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { AvatarFallback } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Settings, Shield, LogOut, ChevronDown } from "lucide-react";

export function UserDropdown() {
  const router = useRouter();
  const { t } = useI18n();
  const currentUsername = useAppStore((s) => s.currentUsername);
  const currentUserRole = useAppStore((s) => s.currentUserRole);
  const name = currentUsername || "admin";
  const role = currentUserRole || "Admin";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger render={
        <Button variant="ghost" className="flex items-center gap-2 px-2 py-1.5" />
      }>
        <div className="transition-transform duration-150 hover:scale-105">
          <AvatarFallback
            name={name}
            size="md"
            shape="square"
            className="bg-gradient-to-br from-primary to-primary/80 shadow-sm shadow-primary/20 text-primary-foreground"
          />
        </div>
        <div className="hidden md:block text-left">
          <div className="text-xs font-medium text-foreground">{name}</div>
          <div className="text-(--fs-micro-sm) text-muted-foreground/100">{role}</div>
        </div>
        <span className="md:hidden text-xs font-medium text-foreground max-w-[60px] truncate">{name.slice(0, 6)}</span>
        <ChevronDown className="size-3 text-muted-foreground/100 hidden md:block" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <div className="px-3 py-2 border-b border-border text-sm font-medium">
          <div>{name}</div>
          <div className="text-(--fs-micro-sm) text-muted-foreground/100">{t("topbar.role", { role })}</div>
        </div>
        <DropdownMenuItem onClick={() => router.push("/settings")}>
          <Settings className="size-4" />{t("topbar.settings")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => router.push("/audit")}>
          <Shield className="size-4" />{t("topbar.audit_log")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem variant="destructive"
          onClick={() => { api.post(paths.auth.logout).catch(() =>         toast.error(t("topbar.toast.logout_failed"))).finally(() => router.push("/login")); }}>
          <LogOut className="size-4" />
          {t("topbar.logout")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}