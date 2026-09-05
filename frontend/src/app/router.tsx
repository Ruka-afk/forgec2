import { lazy, Suspense, type ComponentType, type ReactNode } from "react";
import { createBrowserRouter, Outlet, Navigate, useLocation } from "react-router-dom";
import { PageSpinner } from "@/components/ui/spinner";
import AppLayout from "@/components/AppLayout";
import RouterErrorView from "@/components/framework/RouterErrorView";
import NotFound from "@/app/not-found";
import Forbidden from "@/app/forbidden/page";
import HomePage from "@/app/page";
import LoginPage from "@/app/login/page";
import { NAV_ITEMS } from "@/lib/navigation";
import { useAppStore } from "@/lib/store";
import { canAny } from "@/lib/permissions";
import type { PermissionKey } from "@/lib/permission-keys";

const lazyPage = (imp: () => Promise<{ default: unknown }>) =>
  lazy(imp as () => Promise<{ default: ComponentType }>);

const MAIN_PAGES: Record<string, ReturnType<typeof lazyPage>> = {
  agents: lazyPage(() => import("@/app/(main)/agents/page")),
  ai: lazyPage(() => import("@/app/(main)/ai/page")),
  attack: lazyPage(() => import("@/app/(main)/attack/page")),
  audit: lazyPage(() => import("@/app/(main)/audit/page")),
  automation: lazyPage(() => import("@/app/(main)/automation/page")),
  autotag: lazyPage(() => import("@/app/(main)/autotag/page")),
  bloodhound: lazyPage(() => import("@/app/(main)/bloodhound/page")),
  bof: lazyPage(() => import("@/app/(main)/bof/page")),
  builds: lazyPage(() => import("@/app/(main)/builds/page")),
  campaign: lazyPage(() => import("@/app/(main)/campaign/page")),
  chain: lazyPage(() => import("@/app/(main)/chain/page")),
  chat: lazyPage(() => import("@/app/(main)/chat/page")),
  "circuit-breaker": lazyPage(() => import("@/app/(main)/circuit-breaker/page")),
  cloud: lazyPage(() => import("@/app/(main)/cloud/page")),
  command_templates: lazyPage(() => import("@/app/(main)/command_templates/page")),
  container: lazyPage(() => import("@/app/(main)/container/page")),
  credentials: lazyPage(() => import("@/app/(main)/credentials/page")),
  dashboard: lazyPage(() => import("@/app/(main)/dashboard/page")),
  dns: lazyPage(() => import("@/app/(main)/dns/page")),
  docs: lazyPage(() => import("@/app/(main)/docs/page")),
  "domain-fronting": lazyPage(() => import("@/app/(main)/domain-fronting/page")),
  files: lazyPage(() => import("@/app/(main)/files/page")),
  generate: lazyPage(() => import("@/app/(main)/generate/page")),
  groups: lazyPage(() => import("@/app/(main)/groups/page")),
  infrastructure: lazyPage(() => import("@/app/(main)/infrastructure/page")),
  integrations: lazyPage(() => import("@/app/(main)/integrations/page")),
  lateral: lazyPage(() => import("@/app/(main)/lateral/page")),
  listeners: lazyPage(() => import("@/app/(main)/listeners/page")),
  loot: lazyPage(() => import("@/app/(main)/loot/page")),
  macros: lazyPage(() => import("@/app/(main)/macros/page")),
  notifications: lazyPage(() => import("@/app/(main)/notifications/page")),
  ntlm: lazyPage(() => import("@/app/(main)/ntlm/page")),
  opsec: lazyPage(() => import("@/app/(main)/opsec/page")),
  packer: lazyPage(() => import("@/app/(main)/packer/page")),
  "password-spray": lazyPage(() => import("@/app/(main)/password-spray/page")),
  phishing: lazyPage(() => import("@/app/(main)/phishing/page")),
  pivoting: lazyPage(() => import("@/app/(main)/pivoting/page")),
  plugins: lazyPage(() => import("@/app/(main)/plugins/page")),
  privesc: lazyPage(() => import("@/app/(main)/privesc/page")),
  profiles: lazyPage(() => import("@/app/(main)/profiles/page")),
  report: lazyPage(() => import("@/app/(main)/report/page")),
  roles: lazyPage(() => import("@/app/(main)/roles/page")),
  scanner: lazyPage(() => import("@/app/(main)/scanner/page")),
  scheduler: lazyPage(() => import("@/app/(main)/scheduler/page")),
  screenshots: lazyPage(() => import("@/app/(main)/screenshots/page")),
  scripting: lazyPage(() => import("@/app/(main)/scripting/page")),
  settings: lazyPage(() => import("@/app/(main)/settings/page")),
  stager: lazyPage(() => import("@/app/(main)/stager/page")),
  tags: lazyPage(() => import("@/app/(main)/tags/page")),
  tasks: lazyPage(() => import("@/app/(main)/tasks/page")),
  timeline: lazyPage(() => import("@/app/(main)/timeline/page")),
  tokens: lazyPage(() => import("@/app/(main)/tokens/page")),
  toolkit: lazyPage(() => import("@/app/(main)/toolkit/page")),
  topology: lazyPage(() => import("@/app/(main)/topology/page")),
  traffic: lazyPage(() => import("@/app/(main)/traffic/page")),
  users: lazyPage(() => import("@/app/(main)/users/page")),
  workflows: lazyPage(() => import("@/app/(main)/workflows/page")),
};

const AgentDetailPage = lazyPage(() => import("@/app/(main)/agents/[id]/page"));
const AgentConfigPage = lazyPage(() => import("@/app/(main)/agents/[id]/config/page"));
const AgentFilesPage = lazyPage(() => import("@/app/(main)/agents/[id]/files/page"));
const AgentPersistencePage = lazyPage(() => import("@/app/(main)/agents/[id]/persistence/page"));
const AgentRemoteDesktopPage = lazyPage(() => import("@/app/(main)/agents/[id]/remote-desktop/page"));
const AgentScreenPage = lazyPage(() => import("@/app/(main)/agents/[id]/screen/page"));
const AgentShellPage = lazyPage(() => import("@/app/(main)/agents/[id]/shell/page"));
const AgentTokenPage = lazyPage(() => import("@/app/(main)/agents/[id]/token/page"));
const AgentTrafficPage = lazyPage(() => import("@/app/(main)/agents/[id]/traffic/page"));
const ListenerDetailPage = lazyPage(() => import("@/app/(main)/listeners/[id]/page"));

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
      ...Object.entries(MAIN_PAGES).map(([name, Comp]) => ({
        path: `/${name}`,
        element: guard(name, Comp),
        errorElement: <RouterErrorView />,
      })),
      {
        path: "/agents/:id",
        element: guard("agents/:id", AgentDetailPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/config",
        element: guard("agents/:id/config", AgentConfigPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/files",
        element: guard("agents/:id/files", AgentFilesPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/persistence",
        element: guard("agents/:id/persistence", AgentPersistencePage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/remote-desktop",
        element: guard("agents/:id/remote-desktop", AgentRemoteDesktopPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/screen",
        element: guard("agents/:id/screen", AgentScreenPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/shell",
        element: guard("agents/:id/shell", AgentShellPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/token",
        element: guard("agents/:id/token", AgentTokenPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/agents/:id/traffic",
        element: guard("agents/:id/traffic", AgentTrafficPage),
        errorElement: <RouterErrorView />,
      },
      {
        path: "/listeners/:id",
        element: guard("listeners/:id", ListenerDetailPage),
        errorElement: <RouterErrorView />,
      },
    ],
  },
  {
    path: "*",
    element: withSuspense(<NotFound />),
    errorElement: <RouterErrorView />,
  },
]);