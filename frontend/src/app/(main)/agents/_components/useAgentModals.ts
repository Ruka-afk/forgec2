"use client";

import { useReducer, useCallback } from "react";
import type { Beacon } from "./types";

export type ConfirmType = "kill" | "delete" | "uninstall" | "batch-delete" | "bulk-kill" | "bulk-uninstall";

interface ModalState {
  confirm: { type: ConfirmType; id?: string; hostname?: string } | null;
  cmdModalOpen: boolean;
  cmdType: string;
  cmdText: string;
  quickSleepAgent: { id: string; hostname: string; interval: number; jitter: number } | null;
  sleepInterval: string;
  sleepJitter: string;
  editingNotesId: string | null;
  editNotesText: string;
  screenshotConfirmOpen: boolean;
  batchSleepOpen: boolean;
  batchSleepInterval: string;
  batchSleepJitter: string;
}

type ModalAction =
  | { type: "SET_CONFIRM"; payload: ModalState["confirm"] }
  | { type: "OPEN_CMD_MODAL" }
  | { type: "CLOSE_CMD_MODAL" }
  | { type: "SET_CMD_TYPE"; payload: string }
  | { type: "SET_CMD_TEXT"; payload: string }
  | { type: "OPEN_QUICK_SLEEP"; payload: { id: string; hostname: string; interval: number; jitter: number } }
  | { type: "CLOSE_QUICK_SLEEP" }
  | { type: "SET_SLEEP_INTERVAL"; payload: string }
  | { type: "SET_SLEEP_JITTER"; payload: string }
  | { type: "OPEN_NOTES_EDIT"; payload: { id: string; text: string } }
  | { type: "CLOSE_NOTES_EDIT" }
  | { type: "SET_NOTES_TEXT"; payload: string }
  | { type: "SET_SCREENSHOT_CONFIRM"; payload: boolean }
  | { type: "OPEN_BATCH_SLEEP"; payload: { interval: string; jitter: string } }
  | { type: "CLOSE_BATCH_SLEEP" }
  | { type: "SET_BATCH_SLEEP_INTERVAL"; payload: string }
  | { type: "SET_BATCH_SLEEP_JITTER"; payload: string };

const initialState: ModalState = {
  confirm: null,
  cmdModalOpen: false,
  cmdType: "shell",
  cmdText: "",
  quickSleepAgent: null,
  sleepInterval: "30",
  sleepJitter: "20",
  editingNotesId: null,
  editNotesText: "",
  screenshotConfirmOpen: false,
  batchSleepOpen: false,
  batchSleepInterval: "30",
  batchSleepJitter: "20",
};

function modalReducer(state: ModalState, action: ModalAction): ModalState {
  switch (action.type) {
    case "SET_CONFIRM":
      return { ...state, confirm: action.payload };
    case "OPEN_CMD_MODAL":
      return { ...state, cmdModalOpen: true, cmdType: "shell", cmdText: "" };
    case "CLOSE_CMD_MODAL":
      return { ...state, cmdModalOpen: false };
    case "SET_CMD_TYPE":
      return { ...state, cmdType: action.payload, cmdText: "" };
    case "SET_CMD_TEXT":
      return { ...state, cmdText: action.payload };
    case "OPEN_QUICK_SLEEP":
      return {
        ...state,
        quickSleepAgent: action.payload,
        sleepInterval: String(action.payload.interval),
        sleepJitter: String(action.payload.jitter),
      };
    case "CLOSE_QUICK_SLEEP":
      return { ...state, quickSleepAgent: null };
    case "SET_SLEEP_INTERVAL":
      return { ...state, sleepInterval: action.payload };
    case "SET_SLEEP_JITTER":
      return { ...state, sleepJitter: action.payload };
    case "OPEN_NOTES_EDIT":
      return { ...state, editingNotesId: action.payload.id, editNotesText: action.payload.text };
    case "CLOSE_NOTES_EDIT":
      return { ...state, editingNotesId: null };
    case "SET_NOTES_TEXT":
      return { ...state, editNotesText: action.payload };
    case "SET_SCREENSHOT_CONFIRM":
      return { ...state, screenshotConfirmOpen: action.payload };
    case "OPEN_BATCH_SLEEP":
      return {
        ...state,
        batchSleepOpen: true,
        batchSleepInterval: action.payload.interval,
        batchSleepJitter: action.payload.jitter,
      };
    case "CLOSE_BATCH_SLEEP":
      return { ...state, batchSleepOpen: false };
    case "SET_BATCH_SLEEP_INTERVAL":
      return { ...state, batchSleepInterval: action.payload };
    case "SET_BATCH_SLEEP_JITTER":
      return { ...state, batchSleepJitter: action.payload };
    default:
      return state;
  }
}

export function useAgentModals() {
  const [state, dispatch] = useReducer(modalReducer, initialState);

  const setConfirm = useCallback((payload: ModalState["confirm"]) => dispatch({ type: "SET_CONFIRM", payload }), []);
  const openCmdModal = useCallback(() => dispatch({ type: "OPEN_CMD_MODAL" }), []);
  const closeCmdModal = useCallback(() => dispatch({ type: "CLOSE_CMD_MODAL" }), []);
  const setCmdType = useCallback((payload: string) => dispatch({ type: "SET_CMD_TYPE", payload }), []);
  const setCmdText = useCallback((payload: string) => dispatch({ type: "SET_CMD_TEXT", payload }), []);
  const openQuickSleep = useCallback((agent: Beacon) => {
    dispatch({
      type: "OPEN_QUICK_SLEEP",
      payload: {
        id: agent.id || "",
        hostname: agent.hostname || "",
        interval: agent.current_interval || 30,
        jitter: agent.current_jitter || 20,
      },
    });
  }, []);
  const closeQuickSleep = useCallback(() => dispatch({ type: "CLOSE_QUICK_SLEEP" }), []);
  const setSleepInterval = useCallback((payload: string) => dispatch({ type: "SET_SLEEP_INTERVAL", payload }), []);
  const setSleepJitter = useCallback((payload: string) => dispatch({ type: "SET_SLEEP_JITTER", payload }), []);
  const openNotesEdit = useCallback((agent: Beacon) => {
    dispatch({ type: "OPEN_NOTES_EDIT", payload: { id: agent.id || "", text: agent.notes || "" } });
  }, []);
  const closeNotesEdit = useCallback(() => dispatch({ type: "CLOSE_NOTES_EDIT" }), []);
  const setNotesText = useCallback((payload: string) => dispatch({ type: "SET_NOTES_TEXT", payload }), []);
  const setScreenshotConfirm = useCallback((payload: boolean) => dispatch({ type: "SET_SCREENSHOT_CONFIRM", payload }), []);
  const openBatchSleep = useCallback(
    (interval: string, jitter: string) => dispatch({ type: "OPEN_BATCH_SLEEP", payload: { interval, jitter } }),
    [],
  );
  const closeBatchSleep = useCallback(() => dispatch({ type: "CLOSE_BATCH_SLEEP" }), []);
  const setBatchSleepInterval = useCallback((payload: string) => dispatch({ type: "SET_BATCH_SLEEP_INTERVAL", payload }), []);
  const setBatchSleepJitter = useCallback((payload: string) => dispatch({ type: "SET_BATCH_SLEEP_JITTER", payload }), []);

  return {
    ...state,
    setConfirm,
    openCmdModal,
    closeCmdModal,
    setCmdType,
    setCmdText,
    openQuickSleep,
    closeQuickSleep,
    setSleepInterval,
    setSleepJitter,
    openNotesEdit,
    closeNotesEdit,
    setNotesText,
    setScreenshotConfirm,
    openBatchSleep,
    closeBatchSleep,
    setBatchSleepInterval,
    setBatchSleepJitter,
  };
}
