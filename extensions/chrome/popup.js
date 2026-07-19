document.addEventListener("DOMContentLoaded", async () => {
  const statusBar = document.getElementById("statusBar");
  const statusDot = document.getElementById("statusDot");
  const statusText = document.getElementById("statusText");
  const extId = document.getElementById("extId");
  const connStatus = document.getElementById("connStatus");
  const errorDiv = document.getElementById("errorMsg");
  const forceBtn = document.getElementById("forceBtn");

  async function refresh() {
    try {
      const resp = await chrome.runtime.sendMessage({ type: "get_status" });
      if (resp && resp.uuid) {
        extId.textContent = resp.uuid.substring(0, 8) + "...";
        statusBar.className = "status online";
        statusDot.className = "dot online";
        statusText.textContent = "Connected";
        connStatus.textContent = "Active";
      } else {
        statusBar.className = "status offline";
        statusDot.className = "dot offline";
        statusText.textContent = "Not Registered";
        connStatus.textContent = "Waiting for registration...";
      }
    } catch (e) {
      statusBar.className = "status offline";
      statusDot.className = "dot offline";
      statusText.textContent = "Error";
      connStatus.textContent = "Background unavailable";
    }
  }

  forceBtn.addEventListener("click", async () => {
    forceBtn.disabled = true;
    forceBtn.textContent = "Syncing...";
    try {
      const resp = await chrome.runtime.sendMessage({ type: "force_beacon" });
      if (resp.status === "ok") {
        await refresh();
      }
    } catch (e) {
      const errDiv = document.getElementById("error");
      if (errDiv) errDiv.textContent = "Sync failed: " + e.message;
    } finally {
      forceBtn.disabled = false;
      forceBtn.textContent = "Sync Now";
    }
  });

  await refresh();
});