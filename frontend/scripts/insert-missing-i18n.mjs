// insert-missing-i18n.mjs
// Inserts missing i18n keys into en.ts and zh.ts in alphabetical order.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const I18N_DIR = path.join(__dirname, "..", "src", "lib", "i18n");
const EN_FILE = path.join(I18N_DIR, "en.ts");
const ZH_FILE = path.join(I18N_DIR, "zh.ts");

const MISSING_EN = {
  "agents.agents_count": "Agents ({count})",
  "agents.files_deleted": "Deleted {filename}",
  "agents.files_downloaded": "Downloaded {filename}",
  "agents.files_found_results": "Found {n} result(s)",
  "agents.files_free_of": "{free} free of {total}",
  "agents.files_upload_failed_status": "Upload failed (HTTP {status})",
  "agents.files_uploaded": "Uploaded {name}",
  "agents.recording_recording_fmt": "Recording #{id}",
  "agents.recording_task_fmt": "Task #{id}",
  "agents.token_subtitle": "Token for {hostname}",
  "agents.traffic_baseline_val": "Baseline: {value} bytes",
  "agents.traffic_beacon_timeline": "Beacon Timeline ({count})",
  "agents.traffic_mean": "Mean: {value}s",
  "agents.traffic_stddev": "StdDev: {value}s",
  "agents.traffic_suggest_interval": "Suggested interval: {value}s",
  "agents.traffic_suggest_jitter": "Suggested jitter: {value}%",
  "agents.traffic_suggest_pad": "Suggested padding: {value} bytes",
  "auto.rule_conditions_actions": "({conditions} conditions, {actions} actions)",
  "auto.rules_count": "{count} rule(s)",
  "auto.webhooks_count": "{count} webhook(s)",
  "autotag.applied": "Applied auto-tags to {count} agent(s)",
  "cb.listener_reset": "Reset circuit breaker for listener {id}",
  "cb.listener_toggled": "Listener {id} {state}",
  "cb.probes_every": "Probes every {seconds}s",
  "chain.parent_set": "Parent set to {id}",
  "chain.select_parent_desc": "Parent: {name}",
  "container.task_dispatched": "Dispatched task #{id}",
  "cred.batch_desc": "{count} selected",
  "domain_fronting.remove_domain": "Remove domain {domain}?",
  "login.footer_version": "ForgeC2 v{version}",
  "loot.delete_selected": "Delete {count} selected",
  "notifications.mark_n_read": "Mark {count} as read",
  "packer.exe_loaded": "EXE loaded ({size} KB)",
  "plugins.subtitle": "{count} plugin(s) installed",
  "screenshots.confirm_delete": "Delete {count} screenshot(s)?",
  "scripting.run_history": "History ({count})",
  "sidebar.current_operator": "Operator: {username}",
  "stager.tokens_count": "{count} token(s)",
  "users.showing": "Showing {filtered} of {total}",
};

const MISSING_ZH = {
  "agents.agents_count": "Agent（{count}）",
  "agents.files_deleted": "已删除 {filename}",
  "agents.files_downloaded": "已下载 {filename}",
  "agents.files_found_results": "找到 {n} 个结果",
  "agents.files_free_of": "可用 {free} / 总计 {total}",
  "agents.files_upload_failed_status": "上传失败（HTTP {status}）",
  "agents.files_uploaded": "已上传 {name}",
  "agents.recording_recording_fmt": "录制 #{id}",
  "agents.recording_task_fmt": "任务 #{id}",
  "agents.token_subtitle": "{hostname} 的 Token",
  "agents.traffic_baseline_val": "基线：{value} 字节",
  "agents.traffic_beacon_timeline": "信标时间线（{count}）",
  "agents.traffic_mean": "均值：{value}秒",
  "agents.traffic_stddev": "标准差：{value}秒",
  "agents.traffic_suggest_interval": "建议间隔：{value}秒",
  "agents.traffic_suggest_jitter": "建议抖动：{value}%",
  "agents.traffic_suggest_pad": "建议填充：{value} 字节",
  "auto.rule_conditions_actions": "（{conditions} 条件，{actions} 动作）",
  "auto.rules_count": "{count} 条规则",
  "auto.webhooks_count": "{count} 个 Webhook",
  "autotag.applied": "已为 {count} 个 Agent 应用自动标签",
  "cb.listener_reset": "已重置监听器 {id} 的熔断器",
  "cb.listener_toggled": "监听器 {id} 已{state}",
  "cb.probes_every": "每 {seconds} 秒探测一次",
  "chain.parent_set": "父节点已设为 {id}",
  "chain.select_parent_desc": "父节点：{name}",
  "container.task_dispatched": "已分发任务 #{id}",
  "cred.batch_desc": "已选 {count} 项",
  "domain_fronting.remove_domain": "删除域名 {domain}？",
  "login.footer_version": "ForgeC2 v{version}",
  "loot.delete_selected": "删除已选的 {count} 项",
  "notifications.mark_n_read": "标记 {count} 条为已读",
  "packer.exe_loaded": "已加载 EXE（{size} KB）",
  "plugins.subtitle": "已安装 {count} 个插件",
  "screenshots.confirm_delete": "删除 {count} 张截图？",
  "scripting.run_history": "历史记录（{count}）",
  "sidebar.current_operator": "操作员：{username}",
  "stager.tokens_count": "{count} 个 Token",
  "users.showing": "显示 {filtered} / 共 {total}",
};

function insertKeys(filePath, missingMap) {
  let content = fs.readFileSync(filePath, "utf8");
  const keyRe = /"([A-Za-z_][A-Za-z0-9_]*\.[A-Za-z0-9_]+)":/g;
  const existingKeys = new Set();
  let m;
  while ((m = keyRe.exec(content)) !== null) existingKeys.add(m[1]);

  const toInsert = Object.keys(missingMap)
    .filter((k) => !existingKeys.has(k))
    .sort();

  if (toInsert.length === 0) {
    console.log(`  No keys to insert in ${path.basename(filePath)}`);
    return;
  }

  // Build the insertion block
  const lines = content.split("\n");
  const insertBlock = toInsert.map((k) => `    "${k}": "${missingMap[k]}",`);

  // Find insertion position: before the first defined key that sorts after the insertion key
  const allDefined = [...existingKeys].sort();
  let insertionIdx = -1;
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i].match(/^\s*"([A-Za-z_][A-Za-z0-9_]*\.[A-Za-z0-9_]+)":/);
    if (match) {
      const defKey = match[1];
      // The first insertion key should go before the first defined key > it
      if (toInsert[0] < defKey) {
        insertionIdx = i;
        break;
      }
    }
  }

  if (insertionIdx === -1) {
    // Insert before the closing "};"
    const closeIdx = lines.findIndex((l) => l.trim() === "};");
    insertionIdx = closeIdx;
  }

  const resultLines = [
    ...lines.slice(0, insertionIdx),
    ...insertBlock,
    ...lines.slice(insertionIdx),
  ];

  fs.writeFileSync(filePath, resultLines.join("\n"), "utf8");
  console.log(`  Inserted ${toInsert.length} keys into ${path.basename(filePath)}`);
}

function main() {
  console.log("Inserting missing keys into en.ts...");
  insertKeys(EN_FILE, MISSING_EN);
  console.log("Inserting missing keys into zh.ts...");
  insertKeys(ZH_FILE, MISSING_ZH);
  console.log("\nDone. Run `node scripts/check-i18n.mjs` to verify.");
}

main();
