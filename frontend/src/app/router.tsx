import { lazy, type ComponentType } from "react";
import { Outlet, Route, Routes } from "react-router-dom";
import AppLayout from "@/components/AppLayout";
import NotFound from "@/app/not-found";
import HomePage from "@/app/page";
import LoginPage from "@/app/login/page";

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
  chrome: lazyPage(() => import("@/app/(main)/chrome/page")),
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
  search: lazyPage(() => import("@/app/(main)/search/page")),
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

function MainLayout() {
  return (
    <AppLayout>
      <Outlet />
    </AppLayout>
  );
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<MainLayout />}>
        {Object.entries(MAIN_PAGES).map(([name, Comp]) => (
          <Route key={name} path={`/${name}`} element={<Comp />} />
        ))}
        <Route path="/agents/:id" element={<AgentDetailPage />} />
        <Route path="/agents/:id/config" element={<AgentConfigPage />} />
        <Route path="/agents/:id/files" element={<AgentFilesPage />} />
        <Route path="/agents/:id/persistence" element={<AgentPersistencePage />} />
        <Route path="/agents/:id/remote-desktop" element={<AgentRemoteDesktopPage />} />
        <Route path="/agents/:id/screen" element={<AgentScreenPage />} />
        <Route path="/agents/:id/shell" element={<AgentShellPage />} />
        <Route path="/agents/:id/token" element={<AgentTokenPage />} />
        <Route path="/agents/:id/traffic" element={<AgentTrafficPage />} />
        <Route path="/listeners/:id" element={<ListenerDetailPage />} />
      </Route>
      <Route path="*" element={<NotFound />} />
    </Routes>
  );
}
