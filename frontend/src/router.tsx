import { lazy, Suspense, type ComponentType, type ReactNode } from "react";
import { createBrowserRouter, Outlet, Navigate, useLocation } from "react-router-dom";
import { PageSpinner } from "@/components/ui/spinner";
import AppLayout from "@/components/AppLayout";
import RouterErrorView from "@/components/RouterErrorView";
import NotFound from "@/pages/NotFound";
import Forbidden from "@/pages/forbidden/ForbiddenPage";
import HomePage from "@/pages/home/HomePage";
import LoginPage from "@/pages/login/LoginPage";
import { NAV_ITEMS } from "@/lib/navigation";
import { useAppStore } from "@/lib/store";
import { canAny } from "@/lib/permissions";
import type { PermissionKey } from "@/lib/permission-keys";

const lazyPage = (imp: () => Promise<{ default: unknown }>) =>
  lazy(imp as () => Promise<{ default: ComponentType }>);

const MAIN_PAGES: Record<string, ReturnType<typeof lazyPage>> = {
  agents: lazyPage(() => import("@/pages/agents/AgentsPageContent")),
  ai: lazyPage(() => import("@/pages/ai/AIPageContent")),
  attack: lazyPage(() => import("@/pages/attack/AttackPage")),
  audit: lazyPage(() => import("@/pages/audit/AuditPage")),
  automation: lazyPage(() => import("@/pages/automation/AutomationPageContent")),
  autotag: lazyPage(() => import("@/pages/autotag/AutoTagPage")),
  bloodhound: lazyPage(() => import("@/pages/bloodhound/BloodHoundPage")),
  bof: lazyPage(() => import("@/pages/bof/BOFPage")),
  builds: lazyPage(() => import("@/pages/builds/BuildsPage")),
  campaign: lazyPage(() => import("@/pages/campaign/CampaignPageContent")),
  chain: lazyPage(() => import("@/pages/chain/ChainPage")),
  chat: lazyPage(() => import("@/pages/chat/ChatPage")),
  "circuit-breaker": lazyPage(() => import("@/pages/circuit-breaker/CircuitBreakerPage")),
  cloud: lazyPage(() => import("@/pages/cloud/CloudPage")),
  container: lazyPage(() => import("@/pages/container/ContainerPage")),
  credentials: lazyPage(() => import("@/pages/credentials/CredentialsPageContent")),
  dashboard: lazyPage(() => import("@/pages/dashboard/DashboardPage")),
  dns: lazyPage(() => import("@/pages/dns/DnsPage")),
  "domain-fronting": lazyPage(() => import("@/pages/domain-fronting/DomainFrontingPage")),
  files: lazyPage(() => import("@/pages/files/FilesPage")),
  generate: lazyPage(() => import("@/pages/generate/GeneratePage")),
  groups: lazyPage(() => import("@/pages/groups/GroupsPage")),
  infrastructure: lazyPage(() => import("@/pages/infrastructure/InfrastructurePageContent")),
  integrations: lazyPage(() => import("@/pages/integrations/IntegrationsPage")),
  lateral: lazyPage(() => import("@/pages/lateral/LateralPageContent")),
  listeners: lazyPage(() => import("@/pages/listeners/ListenersPageContent")),
  loot: lazyPage(() => import("@/pages/loot/LootPage")),
  macros: lazyPage(() => import("@/pages/macros/MacrosPageContent")),
  notifications: lazyPage(() => import("@/pages/notifications/NotificationsPage")),
  ntlm: lazyPage(() => import("@/pages/ntlm/NtlmPage")),
  opsec: lazyPage(() => import("@/pages/opsec/OpsecPage")),
  packer: lazyPage(() => import("@/pages/packer/PackerPage")),
  "password-spray": lazyPage(() => import("@/pages/password-spray/PasswordSprayPage")),
  phishing: lazyPage(() => import("@/pages/phishing/PhishingPageContent")),
  pivoting: lazyPage(() => import("@/pages/pivoting/PivotingPageContent")),
  plugins: lazyPage(() => import("@/pages/plugins/PluginsPageContent")),
  privesc: lazyPage(() => import("@/pages/privesc/PrivescPage")),
  profiles: lazyPage(() => import("@/pages/profiles/ProfilesPage")),
  report: lazyPage(() => import("@/pages/report/ReportPageContent")),
  roles: lazyPage(() => import("@/pages/roles/RolesPage")),
  scanner: lazyPage(() => import("@/pages/scanner/ScannerPage")),
  scripting: lazyPage(() => import("@/pages/scripting/ScriptingPage")),
  settings: lazyPage(() => import("@/pages/settings/SettingsPage")),
  stager: lazyPage(() => import("@/pages/stager/StagerPage")),
  tags: lazyPage(() => import("@/pages/tags/TagsPage")),
  tasks: lazyPage(() => import("@/pages/tasks/TasksPage")),
  timeline: lazyPage(() => import("@/pages/timeline/components/EventsPageContent")),
  tokens: lazyPage(() => import("@/pages/tokens/TokensPage")),
  toolkit: lazyPage(() => import("@/pages/toolkit/ToolkitPage")),
  topology: lazyPage(() => import("@/pages/topology/TopologyPage")),
  traffic: lazyPage(() => import("@/pages/traffic/TrafficPage")),
  users: lazyPage(() => import("@/pages/users/UsersPage")),
};

const AgentDetailPage = lazyPage(() => import("@/pages/agents/detail/AgentDetailPage"));
const AgentConfigPage = lazyPage(() => import("@/pages/agents/detail/config/ConfigPage"));
const AgentFilesPage = lazyPage(() => import("@/pages/agents/detail/files/FilesPage"));
const AgentPersistencePage = lazyPage(() => import("@/pages/agents/detail/persistence/PersistencePage"));
const AgentRemoteDesktopPage = lazyPage(() => import("@/pages/agents/detail/remote-desktop/RemoteDesktopPage"));
const AgentScreenPage = lazyPage(() => import("@/pages/agents/detail/screen/ScreenPage"));
const AgentShellPage = lazyPage(() => import("@/pages/agents/detail/shell/ShellPage"));
const AgentTokenPage = lazyPage(() => import("@/pages/agents/detail/token/TokenPage"));
const AgentTrafficPage = lazyPage(() => import("@/pages/agents/detail/traffic/TrafficPage"));
const ListenerDetailPage = lazyPage(() => import("@/pages/listeners/detail/ListenerDetailPage"));

type MainRoute =
  | { path: string; comp: ReturnType<typeof lazyPage> }
  | { path: string; redirect: string };

const MAIN_ROUTES: MainRoute[] = [
  ...Object.keys(MAIN_PAGES).map((name) => ({ path: `/${name}`, comp: MAIN_PAGES[name] })),
  { path: "/agents/:id", comp: AgentDetailPage },
  { path: "/agents/:id/config", comp: AgentConfigPage },
  { path: "/agents/:id/files", comp: AgentFilesPage },
  { path: "/agents/:id/persistence", comp: AgentPersistencePage },
  { path: "/agents/:id/remote-desktop", comp: AgentRemoteDesktopPage },
  { path: "/agents/:id/screen", comp: AgentScreenPage },
  { path: "/agents/:id/shell", comp: AgentShellPage },
  { path: "/agents/:id/token", comp: AgentTokenPage },
  { path: "/agents/:id/traffic", comp: AgentTrafficPage },
  { path: "/command_templates", redirect: "/toolkit" },
  { path: "/docs", redirect: "/settings#tab=about" },
  { path: "/scheduler", redirect: "/automation#tab=scheduled" },
  { path: "/screenshots", redirect: "/loot?tab=screenshots" },
  { path: "/workflows", redirect: "/automation#tab=workflows" },
  { path: "/listeners/:id", comp: ListenerDetailPage },
];

/** Route-local suspense: each lazy page shows the standard spinner while its
 *  chunk loads (replaces the single Root-level Suspense from the declarative
 *  router). */
function withSuspense(node: ReactNode): ReactNode {
  return <Suspense fallback={<PageSpinner />}>{node}</Suspense>;
}

function MainLayout() {
  return (
    <AppLayout>
      <Outlet />
    </AppLayout>
  );
}

/** href -> any-of perms from the nav registry (longest prefix match wins). */
const ROUTE_PERMS: Record<string, PermissionKey[] | undefined> = Object.fromEntries(
  NAV_ITEMS.map((i) => [i.href, i.perms]),
);

function requiredPerms(pathname: string): PermissionKey[] | undefined {
  let best = "";
  for (const [href, perms] of Object.entries(ROUTE_PERMS)) {
    if (perms && pathname.startsWith(href) && href.length > best.length) best = href;
  }
  return best ? ROUTE_PERMS[best] : undefined;
}

/** Route-level permission gate. Fail-open while permissions are still
 *  loading — the backend remains authoritative for enforcement. */
function PermissionRoute({ children }: { children: React.ReactNode }) {
  const pathname = useLocation().pathname;
  const permissions = useAppStore((s) => s.currentPermissions);
  const perms = requiredPerms(pathname);
  if (!perms || permissions == null) return <>{children}</>;
  if (!canAny(permissions, perms)) return <Navigate to="/forbidden" replace />;
  return <>{children}</>;
}

function guard(name: string, Comp: ComponentType): ReactNode {
  return withSuspense(
    <PermissionRoute key={name}>
      <Comp />
    </PermissionRoute>,
  );
}

export const router = createBrowserRouter([
  {
    path: "/",
    element: withSuspense(<HomePage />),
    errorElement: <RouterErrorView />,
  },
  {
    path: "/login",
    element: withSuspense(<LoginPage />),
    errorElement: <RouterErrorView />,
  },
  {
    path: "/forbidden",
    element: withSuspense(<Forbidden />),
    errorElement: <RouterErrorView />,
  },
  {
    element: <MainLayout />,
    errorElement: <RouterErrorView />,
    children: [
      // Single route table: path -> lazy page or redirect target. Adding a
      // page means adding one entry here (plus its lazy import above) —
      // paths, guards and redirects can no longer drift apart.
      ...MAIN_ROUTES.map((r) => ("redirect" in r
        ? {
            path: r.path,
            element: <Navigate to={r.redirect} replace />,
            errorElement: <RouterErrorView />,
          }
        : {
            path: r.path,
            element: guard(r.path, r.comp),
            errorElement: <RouterErrorView />,
          })),
    ],
  },
  {
    path: "*",
    element: withSuspense(<NotFound />),
    errorElement: <RouterErrorView />,
  },
]);