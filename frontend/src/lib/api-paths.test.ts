import { describe, it, expect } from "vitest";
import { paths, DUAL_USE_PREFIXES } from "./api-paths";

describe("api paths", () => {
  it("agents list is under /api", () => {
    expect(paths.agents.list()).toBe("/api/agents");
    expect(paths.agents.list("page=1")).toBe("/api/agents?page=1");
  });
  it("listeners and credentials use /api prefix", () => {
    expect(paths.listeners.list).toMatch(/^\/api\//);
    expect(paths.credentials.list()).toMatch(/^\/api\//);
    expect(paths.credentials.byAgent("abc", 1)).toBe("/credentials?agent_id=abc&limit=1");
  });
  it("agent detail and tasks are dual-use /agents/:id layout", () => {
    expect(paths.agents.one("x")).toBe("/agents/x");
    expect(paths.agents.task("x", 9)).toBe("/agents/x/tasks/9");
    expect(paths.agents.screenshot("x")).toBe("/agents/x/screenshot?format=json");
    expect(paths.agents.remoteInput("x")).toBe("/api/agents/x/input");
    expect(paths.agents.batch).toBe("/agents/batch");
    expect(paths.agents.bulkResults()).toMatch(/^\/agents\/bulk\/results/);
    expect(paths.agents.cmd("x", "shell")).toBe("/agents/x/shell");
    expect(paths.agents.cmd("x", "/files/ls")).toBe("/agents/x/files/ls");
    expect(paths.agents.tokenList("x")).toBe("/agents/x/token/list?format=json");
    expect(paths.agents.socksRelayStart("x")).toBe("/agents/x/socks_relay/start");
  });
  it("loot is dual-use under /loot not /api/loot", () => {
    expect(paths.loot.page).toBe("/loot");
    expect(paths.loot.bulkDelete).toBe("/loot/bulk-delete");
    expect(DUAL_USE_PREFIXES).toContain("/loot");
  });
  it("dual-use list pages are not under /api", () => {
    expect(paths.notifications.list()).toMatch(/^\/notifications\?/);
    expect(paths.groups.list).toBe("/groups");
    expect(paths.users.list).toBe("/users");
    expect(paths.builds.list()).toBe("/builds");
    expect(paths.builds.list("page=1")).toBe("/builds?page=1");
    expect(paths.settings.root).toBe("/settings");
    expect(paths.settings.dbBackups).toBe("/settings/db/backups");
    expect(paths.settings.dbRestore).toBe("/settings/db/restore");
    expect(paths.settings.dbBackupsDownload("x.db")).toContain("name=x.db");
    expect(paths.settings.configDownload).toBe("/settings/config/download");
    expect(paths.settings.maintenancePurge).toBe("/settings/maintenance/purge");
    expect(paths.settings.purge("tasks")).toBe("/settings/purge/tasks");
    expect(paths.audit.logs("page=1")).toBe("/audit/logs?page=1");
  });
  it("dashboard stats live under /api/v1", () => {
    expect(paths.dashboard.v1).toBe("/api/v1/dashboard");
  });
  it("report paths mix dual-use overview and /api/report sections", () => {
    expect(paths.report.overview).toBe("/report");
    expect(paths.report.history).toBe("/api/report/history");
    expect(paths.report.agents("start=1")).toBe("/api/report/agents?start=1");
  });
  it("credentials mutations are dual-use /credentials while list is /api", () => {
    expect(paths.credentials.list()).toMatch(/^\/api\/credentials/);
    expect(paths.credentials.add).toBe("/credentials/add");
    expect(paths.credentials.one(9)).toBe("/credentials/9");
    expect(paths.credentials.batchTags).toBe("/credentials/batch/tags");
  });
  it("listeners enable/disable under /api/listeners", () => {
    expect(paths.listeners.enable("1")).toBe("/api/listeners/1/enable");
    expect(paths.listeners.disable("1")).toBe("/api/listeners/1/disable");
  });
  it("lab paths are centralized", () => {
    expect(paths.socks.sessions).toBe("/socks/sessions");
    expect(paths.rportfwd.status).toBe("/rportfwd/status");
    expect(paths.opsec.rules).toBe("/opsec/rules");
    expect(paths.lateral.execute).toBe("/api/lateral/execute");
    expect(paths.phishing.campaigns).toBe("/phishing/campaigns");
    expect(paths.scheduler.toggle("x")).toBe("/scheduler/tasks/x/toggle");
  });
  it("ai/plugins/bof helpers", () => {
    expect(paths.ai.sessionMessages("s1")).toBe("/ai/sessions/s1/messages");
    expect(paths.plugins.install("p")).toBe("/api/plugins/p/install");
    expect(paths.bof.upload).toContain("/api/bof/upload");
    expect(paths.bloodhound.list).toBe("/bloodhound/list");
    expect(paths.chat.history("ops")).toBe("/chat/history?channel=ops");
    expect(paths.mitre.phases).toBe("/mitre/phases");
  });
});
