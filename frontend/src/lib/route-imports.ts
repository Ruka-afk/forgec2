/**
 * Route id -> lazy page import, shared by the router table (src/router.tsx)
 * and nav prefetching (src/lib/route-prefetch.ts).
 *
 * Keeping the map in its own module matters: prefetch must request the exact
 * same dynamic import the router's lazy() will use, otherwise Vite would emit
 * (and the browser would fetch) a second chunk for the same page. This module
 * holds only import stubs, so it stays tiny in the entry chunk.
 */
export const ROUTE_PAGE_IMPORTS: Record<string, () => Promise<{ default: unknown }>> = {
  beacons: () => import("@/pages/beacons/BeaconsPageContent"),
  "asset-management": () => import("@/pages/asset-management/AssetManagementPage"),
  ai: () => import("@/pages/ai/AIPageContent"),
  attack: () => import("@/pages/attack/AttackPage"),
  audit: () => import("@/pages/audit/AuditPage"),
  automation: () => import("@/pages/automation/AutomationPageContent"),
  autotag: () => import("@/pages/autotag/AutoTagPage"),
  bloodhound: () => import("@/pages/bloodhound/BloodHoundPage"),
  bof: () => import("@/pages/bof/BOFPage"),
  campaign: () => import("@/pages/campaign/CampaignPageContent"),
  "proxy-chains": () => import("@/pages/proxy-chains/ProxyChainsPage"),
  "circuit-breaker": () => import("@/pages/circuit-breaker/CircuitBreakerPage"),
  cloud: () => import("@/pages/cloud/CloudPage"),
  credentials: () => import("@/pages/credentials/CredentialsPageContent"),
  dashboard: () => import("@/pages/dashboard/DashboardPage"),
  dns: () => import("@/pages/dns/DnsPage"),
  "domain-fronting": () => import("@/pages/domain-fronting/DomainFrontingPage"),
  "payload-generator": () => import("@/pages/payload-generator/PayloadGeneratorPage"),
  groups: () => import("@/pages/groups/GroupsPage"),
  infrastructure: () => import("@/pages/infrastructure/InfrastructurePageContent"),
  integrations: () => import("@/pages/integrations/IntegrationsPage"),
  lateral: () => import("@/pages/lateral/LateralPageContent"),
  listeners: () => import("@/pages/listeners/ListenersPageContent"),
  loot: () => import("@/pages/loot/LootPage"),
  macros: () => import("@/pages/macros/MacrosPageContent"),
  ntlm: () => import("@/pages/ntlm/NtlmPage"),
  opsec: () => import("@/pages/opsec/OpsecPage"),
  phishing: () => import("@/pages/phishing/PhishingPageContent"),
  pivoting: () => import("@/pages/pivoting/PivotingPageContent"),
  plugins: () => import("@/pages/plugins/PluginsPageContent"),
  "privilege-escalation": () => import("@/pages/privilege-escalation/PrivilegeEscalationPage"),
  report: () => import("@/pages/report/ReportPageContent"),
  roles: () => import("@/pages/roles/RolesPage"),
  scanner: () => import("@/pages/scanner/ScannerPage"),
  scripting: () => import("@/pages/scripting/ScriptingPage"),
  settings: () => import("@/pages/settings/SettingsPage"),
  tags: () => import("@/pages/tags/TagsPage"),
  events: () => import("@/pages/events/components/EventsPageContent"),
  tokens: () => import("@/pages/tokens/TokensPage"),
  toolkit: () => import("@/pages/toolkit/ToolkitPage"),
  topology: () => import("@/pages/topology/TopologyPage"),
  traffic: () => import("@/pages/traffic/TrafficPage"),
  users: () => import("@/pages/users/UsersPage"),
};
