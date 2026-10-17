import { lazy, Suspense, type ComponentType, type ReactNode } from "react";
import { createBrowserRouter, Outlet, Navigate, useLocation } from "react-router-dom";
import { PageSpinner } from "@/components/ui/spinner";
import ErrorBoundary from "@/components/ErrorBoundary";
import AppLayout from "@/components/AppLayout";
import RouterErrorView from "@/components/RouterErrorView";
import NotFound from "@/pages/NotFound";
import Forbidden from "@/pages/forbidden/ForbiddenPage";
import { NAV_ITEMS } from "@/lib/navigation";
import { ROUTE_PAGE_IMPORTS } from "@/lib/route-imports";
import { useAppStore } from "@/lib/store";
import { canAny } from "@/lib/permissions";
import type { PermissionKey } from "@/lib/permission-keys";

const lazyPage = (imp: () => Promise<{ default: unknown }>) =>
  lazy(imp as () => Promise<{ default: ComponentType }>);

// One lazy chunk per page. The import map lives in src/lib/route-imports.ts so
// nav prefetch (src/lib/route-prefetch.ts) asks for the exact same chunks.
const MAIN_PAGES: Record<string, ReturnType<typeof lazyPage>> = Object.fromEntries(
  Object.entries(ROUTE_PAGE_IMPORTS).map(([name, load]) => [name, lazyPage(load)]),
);

// The login route is lazy like every other page. The sign-in form is the only
// first-paint route that pulled in zod (its validation schema), which kept the
// whole zod vendor chunk in the critical path for operators who never saw the
// form; splitting it drops that chunk from first paint. The 401 handler warms
// this chunk before redirecting (src/lib/login-preload.ts), so an expired
// session still lands on an instantly painting form.
const LoginPage = lazyPage(() => import("@/pages/login/LoginPage"));
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
  // Chat is now a mode of the AI workspace. Keep the old URL as a
  // compatibility redirect so bookmarks and external links continue to work
  // without shipping a second chat application bundle.
  { path: "/chat", redirect: "/ai" },
  { path: "/agents/:id", comp: AgentDetailPage },
  { path: "/agents/:id/config", comp: AgentConfigPage },
  { path: "/agents/:id/files", comp: AgentFilesPage },
  { path: "/agents/:id/persistence", comp: AgentPersistencePage },
  { path: "/agents/:id/remote-desktop", comp: AgentRemoteDesktopPage },
  { path: "/agents/:id/screen", comp: AgentScreenPage },
  { path: "/agents/:id/shell", comp: AgentShellPage },
  { path: "/agents/:id/token", comp: AgentTokenPage },
  { path: "/agents/:id/traffic", comp: AgentTrafficPage },
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
    <RouteErrorBoundary routeKey={name}>
      <PermissionRoute key={name}>
        <Comp />
      </PermissionRoute>
    </RouteErrorBoundary>,
  );
}

/** Per-route crash isolation: a rendering error takes down only its own
 *  page (with retry), never the nav sidebar or sibling routes. resetKey on
 *  the live pathname also recovers same-pattern navigation
 *  (e.g. /agents/1 -> /agents/2). */
function RouteErrorBoundary({ routeKey, children }: { routeKey: string; children: ReactNode }) {
  const pathname = useLocation().pathname;
  return (
    <ErrorBoundary key={routeKey} resetKey={pathname}>
      {children}
    </ErrorBoundary>
  );
}

export const router = createBrowserRouter([
  {
    path: "/",
    // HomePage was a spinner + navigate("/dashboard") stub; the redirect is
    // expressed directly so no chunk loads before the destination.
    element: <Navigate to="/dashboard" replace />,
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
