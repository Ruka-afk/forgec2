"use client";

import { usePathname } from "next/navigation";
import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { Home, ChevronRight as ChevronSep } from "lucide-react";

// Re-export all components from their individual files
export { Spinner, PageSpinner } from "@/components/ui/spinner";
export { PageHeader } from "@/components/ui/page-header";
export { EmptyState } from "@/components/ui/empty-state";
export { StatusBadge } from "@/components/ui/status-badge";
export { Pagination } from "@/components/ui/pagination";
export { ConfirmModal } from "@/components/ui/confirm-modal";
export { CopyButton } from "@/components/ui/copy-button";
export { MdContent } from "@/components/ui/md-content";

/* ── TableCard (2 consumers — keep here) ── */

export function TableCard({ header, children }: { header?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
      {header && <div className="px-4 py-3 border-b border-border text-sm font-medium text-foreground">{header}</div>}
      <div className="overflow-x-auto">
        {children}
      </div>
    </div>
  );
}

/* ── Breadcrumb (1 consumer — keep here) ── */

const SEGMENT_LABELS: Record<string, string> = {
  dashboard: "nav.dashboard",
  agents: "nav.beacons",
  listeners: "nav.listeners",
  tasks: "nav.tasks",
  settings: "nav.settings",
  credentials: "nav.credentials",
  files: "nav.files",
  screenshots: "nav.screenshots",
  loot: "nav.loot",
  plugins: "nav.plugins",
  audit: "nav.audit",
  tags: "nav.tags",
  groups: "nav.groups",
  builds: "nav.builds",
  generate: "nav.generate",
  phishing: "nav.phishing",
  automation: "nav.automation",
  topology: "nav.topology",
  ai: "nav.ai",
  report: "nav.report",
  campaign: "nav.campaign",
  traffic: "nav.traffic",
  opsec: "nav.opsec",
  chain: "nav.chain",
  workflow: "nav.workflows",
  workflows: "nav.workflows",
  toolkit: "nav.toolkit",
  scripting: "nav.scripting",
  pivoting: "nav.pivoting",
  tokens: "nav.token_store",
  scanner: "nav.scanner",
  privesc: "nav.privesc",
  lateral: "nav.lateral",
  bof: "nav.bof",
  command_templates: "nav.templates",
  chrome: "nav.chrome_c2",
  cloud: "nav.cloud",
  ntlm: "nav.ntlm",
  container: "nav.container",
  infrastructure: "nav.infrastructure",
  domain_fronting: "nav.domain_fronting",
  profiles: "nav.profiles",
  packer: "nav.packer",
  stager: "nav.stager",
  dns: "nav.dns",
  users: "nav.users",
  roles: "nav.roles",
  docs: "nav.docs",
  autotag: "nav.autotag",
  scheduler: "nav.scheduler",
  notifications: "nav.notifications",
  chat: "nav.chat",
  integrations: "nav.integrations",
  attack: "nav.attack",
  circuit_breaker: "nav.circuit_breaker",
};

const SUB_ROUTE_LABELS: Record<string, string> = {
  config: "agents.config",
  shell: "agents.shell",
  files: "nav.files",
  screen: "agents.screen",
  processes: "agents.processes",
  screenshot: "agents.screenshot",
  history: "agents.history",
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
    const navKey = SEGMENT_LABELS[seg];
    const label = subKey ? t(subKey) : navKey ? t(navKey) : seg.charAt(0).toUpperCase() + seg.slice(1).replace(/-/g, " ");

    items.push({ label, href: i < segments.length - 1 ? accPath : undefined });
  }

  if (items.length <= 1) return null;

  return (
    <nav aria-label="Breadcrumb" className="mb-4 sm:mb-5">
      <ol className="flex items-center gap-1 text-xs text-muted-foreground/70">
        {items.map((item, i) => {
          const isLast = i === items.length - 1;
          return (
            <li key={i} className="flex items-center gap-1">
              {i > 0 && <ChevronSep className="w-3 h-3 shrink-0 text-muted-foreground/40" />}
              {isLast ? (
                <span className="font-medium text-foreground/80 truncate max-w-[160px]">{item.label}</span>
              ) : item.href ? (
                <Link href={item.href} className="hover:text-foreground transition-colors truncate max-w-[120px]">
                  {i === 0 ? <Home className="w-3.5 h-3.5" /> : item.label}
                </Link>
              ) : (
                <span>{item.label}</span>
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

export { Breadcrumb };
