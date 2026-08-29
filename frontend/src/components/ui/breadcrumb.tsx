"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { NAV_SEGMENT_LABELS } from "@/lib/navigation";
import { Home, ChevronRight as ChevronSep } from "lucide-react";

function normalizeKey(seg: string): string {
  return seg.replace(/-/g, "_");
}

const SUB_ROUTE_LABELS: Record<string, string> = {
  config: "agents.config_title",
  shell: "agents.shell",
  files: "nav.files",
  screen: "agents.screen_title",
  "remote-desktop": "agents.rdp_title",
  token: "agents.token_title",
  traffic: "nav.traffic",
  persistence: "agents.persistence_title",
};

function Breadcrumb() {
  const pathname = usePathname();
  const { t } = useI18n();

  if (!pathname || pathname === "/dashboard" || pathname === "/login") return null;

  const segments = pathname.split("/").filter(Boolean);
  if (segments.length === 0) return null;

  const items: { label: string; href?: string }[] = [
    { label: t("nav.dashboard"), href: "/dashboard" },
  ];

  let accPath = "";
  for (let i = 0; i < segments.length; i++) {
    accPath += "/" + segments[i];
    const seg = segments[i];

    if (seg.match(/^[0-9a-f]{8}-[0-9a-f]{4}-/i) || seg.match(/^\d+$/)) {
      items.push({ label: seg.slice(0, 8) + "...", href: i < segments.length - 1 ? accPath : undefined });
      continue;
    }

    const subKey = SUB_ROUTE_LABELS[seg];
    const navKey = NAV_SEGMENT_LABELS[seg] || NAV_SEGMENT_LABELS[normalizeKey(seg)];
    const label = subKey ? t(subKey) : navKey ? t(navKey) : seg.charAt(0).toUpperCase() + seg.slice(1).replace(/-/g, " ");

    items.push({ label, href: i < segments.length - 1 ? accPath : undefined });
  }

  if (items.length <= 1) return null;

  return (
    <nav aria-label={t("common.breadcrumb")}>
      <ol className="flex items-center gap-1.5 text-xs text-muted-foreground/100">
        {items.map((item, i) => {
          const isLast = i === items.length - 1;
          return (
            <li key={item.label} className="flex items-center gap-1.5">
              {i > 0 && <ChevronSep className="size-3.5 shrink-0 text-muted-foreground/85" />}
              {isLast ? (
                <span className="mono-cell font-medium text-foreground truncate max-w-[240px]">{item.label}</span>
              ) : item.href ? (
                <Link href={item.href} className="mono-eyebrow hover:text-foreground transition-colors truncate max-w-[140px]">
                  {i === 0 ? <Home className="size-3.5" aria-hidden="true" /> : item.label}
                </Link>
              ) : (
                <span className="truncate max-w-[140px]">{item.label}</span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

export { Breadcrumb };
