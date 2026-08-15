"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useI18n } from "@/lib/i18n";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadText } from "@/lib/download";
import { useInteractStore } from "@/lib/interact-store";
import type { Beacon, BulkResult } from "./types";
import type { useAgentModals } from "./useAgentModals";

export function useAgentActions(opts: {
  selected: Set<string>;
  setSelected: (next: Set<string>) => void;
  loadBeacons: () => void;
  sortedBeacons: Beacon[];
  modals: ReturnType<typeof useAgentModals>;
}) {
  const { t } = useI18n();
  const { selected, setSelected, loadBeacons, sortedBeacons, modals } = opts;

  const [actionMsg, setActionMsg] = useState<string | null>(null);
  const [showResults, setShowResults] = useState(false);
  const [bulkResults, setBulkResults] = useState<BulkResult[]>([]);

  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  useEffect(() => {
    if (!actionMsg) return;
    const timer = setTimeout(() => setActionMsg(null), 5000);
    return () => clearTimeout(timer);
  }, [actionMsg]);

  useEffect(() => {
    if (!showResults) return;
    api
      .get(paths.agents.bulkResults())
      .then((d) => {
        if (!mountedRef.current) return;
        const results = (d.results || d.data || []) as BulkResult[];
        setBulkResults(results);
      })
      .catch(() => {
        if (mountedRef.current) setActionMsg(t("agents.bulk_results_failed"));
      });
  }, [showResults, t]);

  const uninstallAgent = async (id: string) => {
    try {
      await api.postJson(paths.agents.uninstall(id), {});
      setActionMsg(t("agents.detail_uninstall_sent"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.detail_uninstall_failed"));
    }
    modals.setConfirm(null);
  };

  const runBatch = async (payload: Record<string, unknown>) => {
    try {
      const data = await api.postJson<{ success?: boolean; tasks_created?: number; error?: string }>(paths.agents.batch, payload);
      if (data.success) {
        setActionMsg(t("agents.batch_result").replace("{count}", String(data.tasks_created ?? selected.size)));
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.batch_failed"));
      }
    } catch {
      setActionMsg(t("agents.batch_failed"));
    }
  };

  const bulkTask = async (agentIds: string[], type: string, command: string) => {
    try {
      const data = await api.postJson<{ success?: boolean; task_count?: number; failed?: string[]; error?: string }>(paths.agents.bulkTask, {
        agent_ids: agentIds,
        type,
        command,
      });
      if (data.success) {
        let bulkMsg = t("agents.bulk_type_result").replace("{type}", type).replace("{count}", String(data.task_count ?? agentIds.length));
        if (data.failed?.length) bulkMsg += ` (${data.failed.length} ${t("agents.failed_suffix")})`;
        setActionMsg(bulkMsg);
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.bulk_task_failed"));
      }
    } catch {
      setActionMsg(t("agents.bulk_task_failed"));
    }
  };

  const batchScreenshot = () => {
    if (!selected.size) return;
    modals.setScreenshotConfirm(true);
  };

  const doBatchScreenshot = () => {
    modals.setScreenshotConfirm(false);
    runBatch({ agent_ids: Array.from(selected), task_type: "screenshot" });
  };

  const batchSleep = () => {
    if (!selected.size) return;
    modals.openBatchSleep("30", "20");
  };

  const doBatchSleep = () => {
    if (!selected.size) return;
    runBatch({ agent_ids: Array.from(selected), task_type: "sleep", args: `${modals.batchSleepInterval},${modals.batchSleepJitter}` });
    modals.closeBatchSleep();
  };

  const batchDelete = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    try {
      const data = await api.postJson<{ success?: boolean; deleted?: number; error?: string }>(paths.agents.batchDelete, { agent_ids: ids });
      if (data.success) {
        setActionMsg(t("agents.delete_result").replace("{count}", String(data.deleted ?? 0)));
        setSelected(new Set());
        loadBeacons();
      } else {
        setActionMsg(data.error || t("agents.delete_failed"));
      }
    } catch {
      setActionMsg(t("agents.delete_failed"));
    }
    modals.setConfirm(null);
  };

  const killAgent = async (id: string) => {
    try {
      await api.postJson(paths.agents.kill(id), {});
      setActionMsg(t("agents.kill_sent"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.kill_failed"));
    }
    modals.setConfirm(null);
  };

  const deleteAgent = async (id: string) => {
    try {
      await api.del(paths.agents.one(id));
      setActionMsg(t("agents.deleted"));
      loadBeacons();
    } catch {
      setActionMsg(t("agents.delete_failed"));
    }
    modals.setConfirm(null);
  };

  const bulkKill = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, "kill", "");
    modals.setConfirm(null);
  };

  const bulkUninstall = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, "uninstall", "");
    modals.setConfirm(null);
  };

  const handleCmdSubmit = async () => {
    const ids = Array.from(selected);
    if (!ids.length) return;
    await bulkTask(ids, modals.cmdType, modals.cmdText);
    modals.closeCmdModal();
  };

  const sendScreenshot = useCallback(async (id: string) => {
    try {
      const data = await api.post<{ task_id?: number }>(paths.agents.screenshotTask(id));
      const taskId = Number(data.task_id);
      const queued = Number.isFinite(taskId) && taskId > 0;
      if (queued) useInteractStore.getState().revealTask(id, taskId);
      setActionMsg(t("agents.detail_action_queued", { label: t("agents.screenshot") }));
    } catch {
      setActionMsg(t("agents.screenshot_failed"));
    }
  }, [t]);

  const submitQuickSleep = async () => {
    if (!modals.quickSleepAgent) return;
    try {
      await api.postJson(paths.agents.setSleep(modals.quickSleepAgent.id), {
        interval: Number(modals.sleepInterval),
        jitter: Number(modals.sleepJitter),
      });
      setActionMsg(t("agents.sleep_updated").replace("{name}", modals.quickSleepAgent.hostname));
      modals.closeQuickSleep();
      loadBeacons();
    } catch {
      setActionMsg(t("agents.sleep_failed"));
    }
  };

  const submitNotes = async () => {
    if (!modals.editingNotesId) return;
    try {
      await api.postJson(paths.agents.note(modals.editingNotesId), { notes: modals.editNotesText });
      setActionMsg(t("agents.notes_updated"));
      modals.closeNotesEdit();
      loadBeacons();
    } catch {
      setActionMsg(t("agents.notes_failed"));
    }
  };

  const exportCSV = () => {
    const headers = ["Hostname", "User", "OS", "IP", "Status", "Last Seen", "Version", "Active Window", "Notes"];
    const rows = sortedBeacons.map((b) => [
      b.hostname || "",
      b.username || "",
      b.os || "",
      b.ip || "",
      b.status || "",
      b.last_seen || "",
      b.version || "",
      b.active_window || "",
      b.notes || "",
    ]);
    const csv = [
      headers.join(","),
      ...rows.map((r) => r.map((c) => `"${String(c).replace(/"/g, '""')}"`).join(",")),
    ].join("\n");
    downloadText(csv, `agents-${new Date().toISOString().slice(0, 10)}.csv`, "text/csv");
  };

  return {
    actionMsg,
    setActionMsg,
    showResults,
    setShowResults,
    bulkResults,
    uninstallAgent,
    runBatch,
    bulkTask,
    batchScreenshot,
    doBatchScreenshot,
    batchSleep,
    doBatchSleep,
    batchDelete,
    killAgent,
    deleteAgent,
    bulkKill,
    bulkUninstall,
    handleCmdSubmit,
    sendScreenshot,
    submitQuickSleep,
    submitNotes,
    exportCSV,
  };
}
