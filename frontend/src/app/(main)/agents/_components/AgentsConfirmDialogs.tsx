"use client";

import { memo } from "react";
import { ConfirmModal } from "@/components/ui/confirm-modal";
import { BulkCommandModal } from "./BulkCommandModal";
import { QuickSleepModal } from "./QuickSleepModal";
import { NotesEditModal } from "./NotesEditModal";
import { BatchSleepModal } from "./BatchSleepModal";
import type { AgentConfirmType } from "./useAgentModals";

type TKey = (key: string, params?: Record<string, string | number>) => string;
type ConfirmState = { type: AgentConfirmType; id?: string; hostname?: string } | null;

interface AgentsConfirmDialogsProps {
  t: TKey;
  confirm: ConfirmState;
  setConfirm: (c: ConfirmState) => void;
  selectedSize: number;
  killAgent: (id: string) => void;
  deleteAgent: (id: string) => void;
  uninstallAgent: (id: string) => void;
  batchDelete: () => void;
  bulkKill: () => void;
  bulkUninstall: () => void;
  cmdModalOpen: boolean;
  cmdType: string;
  setCmdType: (v: string) => void;
  cmdText: string;
  setCmdText: (v: string) => void;
  handleCmdSubmit: () => void;
  closeCmdModal: () => void;
  quickSleepAgent: { id: string; hostname: string; interval: number; jitter: number } | null;
  sleepInterval: string;
  setSleepInterval: (v: string) => void;
  sleepJitter: string;
  setSleepJitter: (v: string) => void;
  submitQuickSleep: () => void;
  closeQuickSleep: () => void;
  editingNotesId: string | null;
  editNotesText: string;
  setNotesText: (v: string) => void;
  submitNotes: () => void;
  closeNotesEdit: () => void;
  screenshotConfirmOpen: boolean;
  setScreenshotConfirm: (open: boolean) => void;
  doBatchScreenshot: () => void;
  batchSleepOpen: boolean;
  batchSleepInterval: string;
  setBatchSleepInterval: (v: string) => void;
  batchSleepJitter: string;
  setBatchSleepJitter: (v: string) => void;
  doBatchSleep: () => void;
  closeBatchSleep: () => void;
}

/** All confirm / command / sleep / notes dialogs for the agents list. */
export default memo(function AgentsConfirmDialogs(props: AgentsConfirmDialogsProps) {
  const {
    t, confirm, setConfirm, selectedSize,
    killAgent, deleteAgent, uninstallAgent, batchDelete, bulkKill, bulkUninstall,
    cmdModalOpen, cmdType, setCmdType, cmdText, setCmdText, handleCmdSubmit, closeCmdModal,
    quickSleepAgent, sleepInterval, setSleepInterval, sleepJitter, setSleepJitter, submitQuickSleep, closeQuickSleep,
    editingNotesId, editNotesText, setNotesText, submitNotes, closeNotesEdit,
    screenshotConfirmOpen, setScreenshotConfirm, doBatchScreenshot,
    batchSleepOpen, batchSleepInterval, setBatchSleepInterval, batchSleepJitter, setBatchSleepJitter, doBatchSleep, closeBatchSleep,
  } = props;

  return (
    <>
      <ConfirmModal
        open={confirm?.type === "kill"}
        title={t("agents.confirm_kill_title")}
        message={t("agents.confirm_kill_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_kill_btn")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void killAgent(confirm.id); }}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "delete"}
        title={t("agents.confirm_delete_title")}
        message={t("agents.confirm_delete_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.confirm_delete_btn")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void deleteAgent(confirm.id); }}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "uninstall"}
        title={t("agents.uninstall_agent")}
        message={t("agents.uninstall_msg").replace("{name}", confirm?.hostname || confirm?.id || "")}
        confirmText={t("agents.uninstall")}
        danger
        requireText={confirm?.hostname || ""}
        onConfirm={() => { if (confirm?.id) void uninstallAgent(confirm.id); }}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "batch-delete"}
        title={t("agents.confirm_batch_delete_title")}
        message={t("agents.confirm_batch_delete_msg").replace("{count}", String(selectedSize))}
        confirmText={t("agents.confirm_batch_delete_btn")}
        danger
        onConfirm={batchDelete}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "bulk-kill"}
        title={t("agents.confirm_bulk_kill_title")}
        message={t("agents.confirm_bulk_kill_msg").replace("{count}", String(selectedSize))}
        confirmText={t("agents.confirm_bulk_kill_btn")}
        danger
        onConfirm={bulkKill}
        onCancel={() => setConfirm(null)}
      />
      <ConfirmModal
        open={confirm?.type === "bulk-uninstall"}
        title={t("agents.confirm_bulk_uninstall_title")}
        message={t("agents.confirm_bulk_uninstall_msg").replace("{count}", String(selectedSize))}
        confirmText={t("agents.confirm_bulk_uninstall_btn")}
        danger
        onConfirm={bulkUninstall}
        onCancel={() => setConfirm(null)}
      />

      <BulkCommandModal
        open={cmdModalOpen}
        selectedCount={selectedSize}
        cmdType={cmdType}
        setCmdType={setCmdType}
        cmdText={cmdText}
        setCmdText={setCmdText}
        onSubmit={handleCmdSubmit}
        onClose={closeCmdModal}
      />

      {quickSleepAgent && (
        <QuickSleepModal
          agent={quickSleepAgent}
          sleepInterval={sleepInterval}
          setSleepInterval={setSleepInterval}
          sleepJitter={sleepJitter}
          setSleepJitter={setSleepJitter}
          onSubmit={submitQuickSleep}
          onClose={closeQuickSleep}
        />
      )}

      {editingNotesId && (
        <NotesEditModal
          notesText={editNotesText}
          setNotesText={setNotesText}
          onSubmit={submitNotes}
          onClose={closeNotesEdit}
        />
      )}

      <ConfirmModal
        open={screenshotConfirmOpen}
        title={t("agents.confirm_screenshot_title")}
        message={t("agents.confirm_screenshot_msg").replace("{count}", String(selectedSize))}
        confirmText={t("agents.confirm_screenshot_btn")}
        danger={false}
        onConfirm={doBatchScreenshot}
        onCancel={() => setScreenshotConfirm(false)}
      />

      {batchSleepOpen && (
        <BatchSleepModal
          agentCount={selectedSize}
          interval={batchSleepInterval}
          onIntervalChange={setBatchSleepInterval}
          jitter={batchSleepJitter}
          setJitter={setBatchSleepJitter}
          onSubmit={doBatchSleep}
          onClose={closeBatchSleep}
        />
      )}
    </>
  );
});
