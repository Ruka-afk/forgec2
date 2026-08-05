/**
 * One-shot helper: rewrite known bare lab API strings to paths.* helpers.
 * Run: node scripts/wire-lab-paths.mjs
 */
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const root = path.join(__dirname, "..");

const files = [
  "src/app/(main)/phishing/PhishingPageContent.tsx",
  "src/app/(main)/scheduler/page.tsx",
  "src/app/(main)/lateral/LateralPageContent.tsx",
  "src/app/(main)/privesc/page.tsx",
  "src/app/(main)/scripting/page.tsx",
  "src/app/(main)/roles/page.tsx",
  "src/app/(main)/tags/page.tsx",
  "src/app/(main)/cloud/page.tsx",
  "src/app/(main)/command_templates/page.tsx",
  "src/app/(main)/integrations/page.tsx",
  "src/app/(main)/scanner/page.tsx",
  "src/app/(main)/traffic/page.tsx",
  "src/app/(main)/toolkit/page.tsx",
  "src/app/(main)/infrastructure/InfrastructurePageContent.tsx",
  "src/app/(main)/infrastructure/_components/useInfrastructureConfigForm.ts",
  "src/app/(main)/scripting/page.tsx",
  "src/app/(main)/generate/hooks/usePayloadGenerator.ts",
  "src/app/(main)/settings/_components/BackupSection.tsx",
  "src/app/(main)/settings/_components/ExtC2Section.tsx",
  "src/app/(main)/plugins/_components/usePluginsData.ts",
  "src/app/(main)/automation/AutomationPageContent.tsx",
];

function ensureImport(text) {
  if (text.includes('from "@/lib/api-paths"') || text.includes("from '@/lib/api-paths'")) return text;
  if (text.includes('from "@/lib/api"')) {
    return text.replace('from "@/lib/api";', 'from "@/lib/api";\nimport { paths } from "@/lib/api-paths";');
  }
  return text;
}

const regexReplacements = [
  [/api\.get\("\/phishing\/templates"\)/g, "api.get(paths.phishing.templates)"],
  [/api\.get\("\/phishing\/campaigns"\)/g, "api.get(paths.phishing.campaigns)"],
  [/api\.get\("\/phishing\/captures"\)/g, "api.get(paths.phishing.captures)"],
  [/api\.postJson\("\/phishing\/templates",/g, "api.postJson(paths.phishing.templates,"],
  [/api\.postJson\("\/phishing\/campaigns",/g, "api.postJson(paths.phishing.campaigns,"],
  [/api\.putJson\(`\/phishing\/templates\/\$\{editTplId\}`,/g, "api.putJson(paths.phishing.template(editTplId),"],
  [/api\.del\(`\/phishing\/templates\/\$\{id\}`\)/g, "api.del(paths.phishing.template(id))"],
  [/api\.post\(`\/phishing\/campaigns\/\$\{id\}\/stop`\)/g, "api.post(paths.phishing.campaignStop(id))"],
  [/api\.del\(`\/phishing\/campaigns\/\$\{id\}`\)/g, "api.del(paths.phishing.campaign(id))"],

  [/api\.postJson\("\/scheduler\/tasks",/g, "api.postJson(paths.scheduler.tasks,"],
  [/api\.putJson\(`\/scheduler\/tasks\/\$\{editingId\}`,/g, "api.putJson(paths.scheduler.task(editingId),"],
  [/api\.postJson\(`\/scheduler\/tasks\/\$\{id\}\/toggle`, \{\}\)/g, "api.postJson(paths.scheduler.toggle(id), {})"],
  [/api\.del\(`\/scheduler\/tasks\/\$\{id\}`\)/g, "api.del(paths.scheduler.task(id))"],

  [/api\.get\(`\/api\/lateral\/history\/all`\)/g, "api.get(paths.lateral.historyAll)"],
  [/api\.get\("\/api\/lateral\/history\/all"\)/g, "api.get(paths.lateral.historyAll)"],
  [/api\.get\(`\/tasks\?type=lateral&pageSize=50`\)/g, 'api.get(paths.tasks.list("type=lateral&pageSize=50"))'],
  [/api\.postJson\(`\/api\/lateral\/execute`,/g, "api.postJson(paths.lateral.execute,"],
  [/api\.postJson\("\/api\/lateral\/execute",/g, "api.postJson(paths.lateral.execute,"],

  [/api\.get\(`\/privesc`\)/g, "api.get(paths.privesc.page)"],
  [/api\.get\("\/privesc"\)/g, "api.get(paths.privesc.page)"],
  [/api\.postJson\(`\/api\/privesc\/run`,/g, "api.postJson(paths.privesc.run,"],
  [/api\.get\(`\/api\/privesc\/history\/\$\{historyId\}`\)/g, "api.get(paths.privesc.history(historyId))"],
  [/api\.postJson\(`\/api\/privesc\/execute`,/g, "api.postJson(paths.privesc.execute,"],

  [/api\.get\("\/api\/scripts"\)/g, "api.get(paths.scripts.list)"],
  [/api\.get\("\/api\/scripts\/history"\)/g, "api.get(paths.scripts.history)"],
  [/api\.postJson\("\/api\/scripts",/g, "api.postJson(paths.scripts.list,"],
  [/api\.del\(`\/api\/scripts\/\$\{scriptId\}`\)/g, "api.del(paths.scripts.one(scriptId))"],
  [/api\.postJson\("\/api\/scripts\/execute",/g, "api.postJson(paths.scripts.execute,"],

  [/api\.postJson\("\/api\/templates",/g, "api.postJson(paths.templates.list,"],
  [/api\.del\(`\/api\/templates\/\$\{id\}`\)/g, "api.del(paths.templates.one(id))"],

  [/api\.postJson\("\/api\/roles",/g, "api.postJson(paths.roles.list,"],
  [/api\.postJson\(`\/api\/roles\/\$\{role\.id\}`,/g, "api.postJson(paths.roles.one(role.id),"],
  [/api\.del\(`\/api\/roles\/\$\{id\}`\)/g, "api.del(paths.roles.one(id))"],

  [/return api\.get\("\/api\/tags"\)/g, "return api.get(paths.tags.list)"],
  [/api\.get\("\/api\/tags"\)/g, "api.get(paths.tags.list)"],

  [/api\.postJson\("\/cloud\/steal",/g, "api.postJson(paths.cloud.steal,"],

  [/api\.get\("\/scanner"\)/g, "api.get(paths.scanner.page)"],
  [/api\.post\("\/api\/scan",/g, "api.post(paths.scanner.scan,"],
  [/api\.get\("\/traffic"\)/g, "api.get(paths.traffic.page)"],
  [/api\.get\("\/toolkit\/results"\)/g, "api.get(paths.toolkit.results)"],
  [/api\.post\(`\/toolkit\/agents\/\$\{selectedAgent\}\/action`,/g, "api.post(paths.toolkit.action(selectedAgent),"],

  [/api\.postJson\("\/integrations",/g, "api.postJson(paths.integrations.list,"],
  [/api\.postJson\(`\/integrations\/\$\{id\}\/toggle`, \{\}\)/g, "api.postJson(paths.integrations.toggle(id), {})"],
  [/api\.del\(`\/integrations\/\$\{id\}`\)/g, "api.del(paths.integrations.one(id))"],

  [/api\.del\(`\/redirectors\/\$\{rd\.id\}`\)/g, "api.del(paths.redirectors.one(rd.id))"],
  [/api\.putJson\(`\/redirectors\/\$\{editingRd\}`, payload\)/g, "api.putJson(paths.redirectors.one(editingRd), payload)"],
  [/api\.postJson\("\/redirectors", payload\)/g, "api.postJson(paths.redirectors.list, payload)"],
  [/api\.postJson\("\/redirectors\/test-ssh",/g, "api.postJson(paths.redirectors.testSsh,"],
  [/api\.postJson\("\/infrastructure\/acme\/provision",/g, "api.postJson(paths.infrastructure.acmeProvision,"],
  [/api\.get\(`\/infrastructure\/profile\/export\?format=\$\{exportFormat\}`\)/g, "api.get(paths.infrastructure.profileExport(exportFormat))"],

  [/api\.del\(`\/api\/generate\/profile\/\$\{name\}`\)/g, "api.del(paths.generate.profileDelete(name))"],
  [/api\.post\("\/settings\/db\/backup"\)/g, "api.post(paths.settings.dbBackup)"],
  [/api\.get\("\/extc2\/configs"\)/g, "api.get(paths.extc2.configs)"],
  [/api\.del\(`\/extc2\/configs\/\$\{id\}`\)/g, "api.del(paths.extc2.config(id))"],
  [/api\.get\("\/api\/plugins"/g, 'api.get(paths.plugins.list"'],
];

// fix plugins carefully below
const pluginFileFixes = (t) =>
  t
    .replace(/api\.get\("\/api\/plugins"/g, 'api.get(paths.plugins.list"')
    .replace(/api\.get\(paths\.plugins\.list", \{ signal \}\)/g, "api.get(paths.plugins.list, { signal })")
    .replace(/api\.get\("\/api\/plugins", \{ signal \}\)/g, "api.get(paths.plugins.list, { signal })");

const automationFixes = (t) =>
  t
    .replace(/api\.postJson\("\/api\/automation\/rules",/g, "api.postJson(paths.automation.rules,")
    .replace(/api\.postJson\("\/api\/webhooks",/g, "api.postJson(paths.automation.webhooks,")
    .replace(/api\.del\(`\/api\/automation\/rules\/\$\{id\}`\)/g, "api.del(paths.automation.rule(id))")
    .replace(/api\.del\(`\/api\/webhooks\/\$\{id\}`\)/g, "api.del(paths.automation.webhook(id))")
    .replace(/api\.postJson\("\/api\/monitor\/alert-rules",/g, "api.postJson(paths.automation.alertRules,")
    .replace(/api\.del\(`\/api\/monitor\/alert-rules\/\$\{id\}`\)/g, "api.del(paths.automation.alertRule(id))")
    .replace(
      /api\.putJson\(`\/api\/monitor\/alert-rules\/\$\{rule\.id\}`,/g,
      "api.putJson(paths.automation.alertRule(rule.id),",
    )
    .replace(/api\.post\(`\/api\/monitor\/alerts\/\$\{id\}\/acknowledge`\)/g, "api.post(paths.automation.alertAck(id))")
    .replace(/api\.post\(`\/api\/monitor\/alerts\/\$\{id\}\/resolve`\)/g, "api.post(paths.automation.alertResolve(id))")
    .replace(/api\.post\(`\/api\/automation\/rules\/\$\{id\}\/toggle`\)/g, "api.post(paths.automation.ruleToggle(id))");

for (const rel of files) {
  const f = path.join(root, rel);
  if (!fs.existsSync(f)) {
    console.log("skip missing", rel);
    continue;
  }
  let text = fs.readFileSync(f, "utf8");
  const before = text;
  text = ensureImport(text);
  for (const [re, rep] of regexReplacements) {
    text = text.replace(re, rep);
  }
  if (rel.includes("usePluginsData")) text = pluginFileFixes(text);
  if (rel.includes("AutomationPageContent")) text = automationFixes(text);
  if (text !== before) {
    fs.writeFileSync(f, text);
    console.log("updated", rel);
  } else {
    console.log("no change", rel);
  }
}
