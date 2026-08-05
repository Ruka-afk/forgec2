/**
 * Wire remaining bare API paths to paths.* (Master-10).
 * Run: node scripts/wire-remaining-paths.mjs
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, "..");

const files = [
  "src/app/(main)/ai/_components/useAISessions.ts",
  "src/app/(main)/ai/_components/useAIConfig.ts",
  "src/app/(main)/ai/AIPageContent.tsx",
  "src/app/(main)/plugins/PluginsPageContent.tsx",
  "src/app/(main)/bof/_components/useBOFData.ts",
  "src/app/(main)/bloodhound/page.tsx",
  "src/app/(main)/chrome/page.tsx",
  "src/app/(main)/chat/page.tsx",
  "src/app/(main)/circuit-breaker/page.tsx",
  "src/app/(main)/autotag/page.tsx",
  "src/app/(main)/domain-fronting/page.tsx",
  "src/app/(main)/topology/page.tsx",
  "src/app/(main)/stager/page.tsx",
  "src/app/(main)/attack/page.tsx",
  "src/app/(main)/tasks/page.tsx",
  "src/app/(main)/agents/[id]/AgentDetailPage.tsx",
  "src/app/(main)/settings/_components/NotificationsSection.tsx",
  "src/app/(main)/settings/_components/SyncSection.tsx",
  "src/app/(main)/settings/_components/useTOTP.ts",
  "src/app/(main)/settings/_components/ModulesSection.tsx",
  "src/app/(main)/settings/_components/SIEMSection.tsx",
  "src/components/TopBar.tsx",
];

function ensureImport(text) {
  if (text.includes('from "@/lib/api-paths"') || text.includes("from '@/lib/api-paths'")) return text;
  if (text.includes('from "@/lib/api"')) {
    return text.replace(
      'from "@/lib/api";',
      'from "@/lib/api";\nimport { paths } from "@/lib/api-paths";',
    );
  }
  return text;
}

const rules = [
  // AI
  [/api\.get\("\/ai\/sessions"\)/g, "api.get(paths.ai.sessions)"],
  [/api\.get\(`\/ai\/sessions\/\$\{id\}\/messages`\)/g, "api.get(paths.ai.sessionMessages(id))"],
  [/api\.del\(`\/ai\/sessions\/\$\{id\}`\)/g, "api.del(paths.ai.session(id))"],
  [/api\.get\("\/ai"\)/g, "api.get(paths.ai.root)"],
  [/api\.postJson\("\/ai\/config",/g, "api.postJson(paths.ai.config,"],
  [/api\.putJson\(`\/ai\/sessions\/\$\{renameTarget\.id\}`,/g, "api.putJson(paths.ai.session(renameTarget.id),"],
  [/api\.postJson\(`\/ai\/sessions\/\$\{sessionId\}\/messages`,/g, "api.postJson(paths.ai.sessionMessages(sessionId),"],

  // Plugins
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/install`, \{\}\)/g, "api.post(paths.plugins.install(pluginId), {})"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/toggle`, \{ enabled: "false" \}\)/g, 'api.post(paths.plugins.toggle(pluginId), { enabled: "false" })'],
  [/api\.del\(`\/api\/plugins\/\$\{pluginId\}`\)/g, "api.del(paths.plugins.one(pluginId))"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/toggle`, \{ enabled: String\(enabled\) \}\)/g, "api.post(paths.plugins.toggle(pluginId), { enabled: String(enabled) })"],
  [/api\.post\("\/api\/plugins", body\)/g, "api.post(paths.plugins.create, body)"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/execute`\)/g, "api.post(paths.plugins.execute(pluginId))"],
  [/api\.postFormData\("\/api\/plugins\/import\?format=json", formData\)/g, "api.postFormData(paths.plugins.importJson, formData)"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/update`\)/g, "api.post(paths.plugins.update(pluginId))"],
  [/api\.post\("\/api\/plugins\/check-updates"\)/g, "api.post(paths.plugins.checkUpdates)"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/reviews`, body\)/g, "api.post(paths.plugins.reviews(pluginId), body)"],
  [/api\.post\(`\/api\/plugins\/\$\{pluginId\}\/rating`,/g, "api.post(paths.plugins.rating(pluginId),"],

  // BOF
  [/api\.postFormData\("\/api\/bof\/upload\?format=json", formData\)/g, "api.postFormData(paths.bof.upload, formData)"],
  [/api\.del\(`\/api\/bof\/\$\{id\}`\)/g, "api.del(paths.bof.one(id))"],
  [/api\.post\(`\/api\/bof\/\$\{id\}\/run`,/g, "api.post(paths.bof.run(id),"],
  [/api\.post\(`\/api\/bof\/\$\{id\}\/edit`,/g, "api.post(paths.bof.edit(id),"],
  [/api\.postJson\("\/api\/bof\/repos\/import",/g, "api.postJson(paths.bof.reposImport,"],
  [/api\.postJson\(`\/api\/bof\/repos\/\$\{itemId\}\/rate`,/g, "api.postJson(paths.bof.reposRate(itemId),"],

  // BloodHound
  [/api\.get\("\/bloodhound\/list"\)/g, "api.get(paths.bloodhound.list)"],
  [/api\.postFormData\("\/bloodhound\/upload", form\)/g, "api.postFormData(paths.bloodhound.upload, form)"],
  [/api\.post\("\/bloodhound\/collect",/g, "api.post(paths.bloodhound.collect,"],
  [/api\.del\(`\/bloodhound\/\$\{id\}`\)/g, "api.del(paths.bloodhound.one(id))"],

  // Chrome / Chat / Circuit breaker
  [/api\.postJson\(`\/chrome\/agents\/\$\{selectedAgent\}\/tasks`,/g, "api.postJson(paths.chrome.agentTasks(selectedAgent),"],
  [/api\.get\(`\/chat\/history\?channel=\$\{currentChannel\}`,/g, "api.get(paths.chat.history(currentChannel),"],
  [/api\.get\("\/chat\/channels",/g, "api.get(paths.chat.channels,"],
  [/api\.postJson\("\/chat\/send",/g, "api.postJson(paths.chat.send,"],
  [/api\.postJson\("\/circuit-breaker\/config",/g, "api.postJson(paths.circuitBreaker.config,"],
  [/api\.post\(`\/circuit-breaker\/reset\/\$\{listenerId\}`\)/g, "api.post(paths.circuitBreaker.reset(listenerId))"],
  [/api\.postJson\(`\/circuit-breaker\/toggle\/\$\{listenerId\}`,/g, "api.postJson(paths.circuitBreaker.toggle(listenerId),"],

  // Autotag / domain front / mesh / stager / attack
  [/api\.putJson\(`\/api\/autotag\/rules\/\$\{editingId\}`,/g, "api.putJson(paths.autotag.rule(editingId),"],
  [/api\.postJson\("\/api\/autotag\/rules",/g, "api.postJson(paths.autotag.rules,"],
  [/api\.postJson\(`\/api\/autotag\/rules\/\$\{id\}\/toggle`, \{\}\)/g, "api.postJson(paths.autotag.toggle(id), {})"],
  [/api\.del\(`\/api\/autotag\/rules\/\$\{id\}`\)/g, "api.del(paths.autotag.rule(id))"],
  [/api\.postJson\("\/infra\/front\/list", \{\}\)/g, "api.postJson(paths.domainFront.list, {})"],
  [/api\.postJson\("\/infra\/front\/check", \{\}\)/g, "api.postJson(paths.domainFront.check, {})"],
  [/api\.postJson\("\/infra\/front\/config",/g, "api.postJson(paths.domainFront.config,"],
  [/api\.postJson\(`\/mesh\/route\/\$\{agentId\}`,/g, "api.postJson(paths.mesh.route(agentId),"],
  [/api\.del\(`\/stager\/\$\{id\}`\)/g, "api.del(paths.stager.one(id))"],
  [/api\.get\("\/mitre\/phases",/g, "api.get(paths.mitre.phases,"],

  // Tasks collab
  [/api\.get\(`\/tasks\?\$\{params\}`\)/g, "api.get(paths.tasks.list(params.toString()))"],
  [/api\.post\(`\/tasks\/\$\{task\.id\}\/approve`\)/g, "api.post(paths.tasksCollab.approve(task.id))"],
  [/api\.post\(`\/tasks\/\$\{task\.id\}\/reject`\)/g, "api.post(paths.tasksCollab.reject(task.id))"],
  [/api\.post\(`\/collab\/tasks\/\$\{taskId\}\/claim`\)/g, "api.post(paths.tasksCollab.claim(taskId))"],
  [/api\.post\(`\/collab\/tasks\/\$\{taskId\}\/release`\)/g, "api.post(paths.tasksCollab.release(taskId))"],

  // Agent detail key rotate
  [/api\.postJson\(`\/api\/v1\/tasks`,/g, "api.postJson(paths.v1.tasks,"],

  // Settings sections
  [/api\.get\("\/settings\/webhooks"\)/g, "api.get(paths.settings.webhooks)"],
  [/api\.postJson\("\/settings\/webhooks",/g, "api.postJson(paths.settings.webhooks,"],
  [/api\.postJson\("\/settings\/webhooks\/test",/g, "api.postJson(paths.settings.webhooksTest,"],
  [/api\.get\("\/settings\/sync"\)/g, "api.get(paths.settings.sync)"],
  [/api\.get\("\/settings\/sync\/status"\)/g, "api.get(paths.settings.syncStatus)"],
  [/api\.postJson\("\/settings\/sync",/g, "api.postJson(paths.settings.sync,"],
  [/api\.postJson\("\/settings\/sync\/test",/g, "api.postJson(paths.settings.syncTest,"],
  [/api\.post\("\/settings\/sync\/trigger"\)/g, "api.post(paths.settings.syncTrigger)"],
  [/api\.get\("\/settings\/totp\/status"\)/g, "api.get(paths.settings.totpStatus)"],
  [/api\.post\("\/settings\/totp\/generate"\)/g, "api.post(paths.settings.totpGenerate)"],
  [/api\.post\("\/settings\/totp\/enable",/g, "api.post(paths.settings.totpEnable,"],
  [/api\.post\("\/settings\/totp\/disable",/g, "api.post(paths.settings.totpDisable,"],
  [/api\.postFormData\("\/api\/modules", fd\)/g, "api.postFormData(paths.modules.list, fd)"],
  [/api\.del\(`\/api\/modules\/\$\{encodeURIComponent\(name\)\}`\)/g, "api.del(paths.modules.one(name))"],
  [/api\.get\("\/settings\/siem"\)/g, "api.get(paths.settings.siem)"],
  [/api\.postJson\("\/settings\/siem",/g, "api.postJson(paths.settings.siem,"],
  [/api\.postJson\("\/settings\/siem\/test",/g, "api.postJson(paths.settings.siemTest,"],

  // TopBar logout
  [/api\.post\("\/logout"\)/g, "api.post(paths.auth.logout)"],
];

for (const rel of files) {
  const f = path.join(root, rel);
  if (!fs.existsSync(f)) {
    console.log("skip", rel);
    continue;
  }
  let text = fs.readFileSync(f, "utf8");
  const before = text;
  text = ensureImport(text);
  for (const [re, rep] of rules) {
    text = text.replace(re, rep);
  }
  // tasks page special: params may be URLSearchParams
  if (rel.includes("tasks/page.tsx")) {
    text = text.replace(
      /api\.get\(`\/tasks\?\$\{params\}`\)/g,
      "api.get(paths.tasks.list(String(params)))",
    );
  }
  if (text !== before) {
    fs.writeFileSync(f, text);
    console.log("updated", rel);
  } else {
    console.log("no change", rel);
  }
}
