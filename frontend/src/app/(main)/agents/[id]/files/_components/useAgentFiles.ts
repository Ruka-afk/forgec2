"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { pullRemoteFile, pushLocalFile } from "./file-transfer";
import {
  isImageFile,
  joinPath,
  type FileEntry,
} from "./types";
import { extractImmediateListing, filesLsTaskId, isFilesLsAck, parseLsListing } from "./ls-listing";
import {
  fileReadPreview,
  fileTaskId,
  isFileTaskAck,
  looksLikeFileTaskAckJson,
  parseFindResult,
  type TransferProgress,
} from "./file-task";

export function useAgentFiles(agentId: string) {
  const { t } = useI18n();
  const [currentPath, setCurrentPath] = useState("C:\\");
  const [currentPathInput, setCurrentPathInput] = useState("C:\\");
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploading, setUploading] = useState(false);
  const [pullName, setPullName] = useState<string | null>(null);
  const [pullProgress, setPullProgress] = useState<TransferProgress | null>(null);
  const [previewContent, setPreviewContent] = useState("");
  const [previewIsImage, setPreviewIsImage] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [driveOutput, setDriveOutput] = useState("");
  const [showDrives, setShowDrives] = useState(false);
  const [findPattern, setFindPattern] = useState("");
  const [findResults, setFindResults] = useState<string[]>([]);
  const [osType, setOsType] = useState<"windows" | "linux">("windows");
  const uploadAbortRef = useRef<AbortController | null>(null);
  const lsAbortRef = useRef<AbortController | null>(null);

  const showToast = useCallback((text: string, type: string = "info") => {
    if (type === "success") toast.success(text);
    else if (type === "error") toast.error(text);
    else toast(text);
  }, []);

  const loadDirectory = useCallback(
    async (path: string) => {
      if (!agentId) return;
      lsAbortRef.current?.abort();
      const ac = new AbortController();
      lsAbortRef.current = ac;
      setLoading(true);
      try {
        const data = await api.post(paths.agents.filesLs(agentId), { path }, { signal: ac.signal });
        if (ac.signal.aborted) return;
        let items = extractImmediateListing(data) ?? [];
        if (isFilesLsAck(data)) {
          const taskId = filesLsTaskId(data);
          if (!taskId) throw new Error(t("agents.dock_files_waiting"));
          const st = await pollTask(agentId, taskId, { signal: ac.signal, timeoutMs: 90_000 });
          if (ac.signal.aborted) return;
          if (st.status === "failed") throw new Error(st.error || t("agents.files_ls_failed"));
          items = parseLsListing(st.result || "");
        }
        items.sort((a, b) => {
          if (a.is_dir && !b.is_dir) return -1;
          if (!a.is_dir && b.is_dir) return 1;
          return a.name.localeCompare(b.name);
        });
        setEntries(items);
        setCurrentPath(path);
        setCurrentPathInput(path);
      } catch (err) {
        if (ac.signal.aborted) return;
        showToast(String(err), "error");
        setEntries([]);
      } finally {
        if (!ac.signal.aborted) setLoading(false);
      }
    },
    [agentId, showToast, t],
  );

  const detectOs = useCallback(async (): Promise<"windows" | "linux"> => {
    if (!agentId) return "windows";
    try {
      const data = await api.get<{
        Agent?: { os?: string; OS?: string };
        agent?: { os?: string; OS?: string };
      }>(`${paths.agents.one(agentId)}?format=json`);
      const agentData = data.agent || data.Agent || {};
      const os = String(agentData.os || agentData.OS || "windows").toLowerCase();
      if (os.includes("linux") || os.includes("darwin") || os.includes("unix")) {
        setOsType("linux");
        return "linux";
      }
    } catch {
      toast.error(t("agents.files_detect_os_failed"));
    }
    return "windows";
  }, [agentId, t]);

  useEffect(() => {
    if (!agentId) return;
    let cancelled = false;
    (async () => {
      const os = await detectOs();
      if (cancelled) return;
      const initialPath = os === "linux" ? "/" : "C:\\";
      void loadDirectory(initialPath);
    })();
    return () => {
      cancelled = true;
      if (uploadAbortRef.current) {
        uploadAbortRef.current.abort();
        uploadAbortRef.current = null;
      }
      if (lsAbortRef.current) {
        lsAbortRef.current.abort();
        lsAbortRef.current = null;
      }
    };
  }, [agentId, detectOs, loadDirectory]);

  const navigateTo = useCallback(
    (path: string) => {
      void loadDirectory(path);
      setSelectedFile(null);
    },
    [loadDirectory],
  );

  const downloadFile = useCallback(
    async (filename: string) => {
      const remote = joinPath(currentPath, filename, osType);
      const known = entries.find((e) => e.name === filename)?.size ?? 0;
      setPullName(filename);
      setPullProgress(null);
      try {
        showToast(t("agents.files_pull_queued", { filename }), "info");
        const kind = await pullRemoteFile({
          agentId,
          remotePath: remote,
          filename,
          fileSize: known,
          t,
          onProgress: setPullProgress,
        });
        if (kind === "partial") {
          showToast(t("agents.files_pull_partial", { filename, size: String(known || "") }), "info");
        } else {
          showToast(t("agents.files_pulled", { filename }), "success");
        }
      } catch (err) {
        showToast(String(err), "error");
      } finally {
        setPullName(null);
        setPullProgress(null);
      }
    },
    [agentId, currentPath, entries, osType, showToast, t],
  );

  const readFile = useCallback(
    async (filename: string) => {
      const path = joinPath(currentPath, filename, osType);
      try {
        const data = await api.post(paths.agents.filesRead(agentId), { path });
        let raw = "";
        if (isFileTaskAck(data)) {
          const taskId = fileTaskId(data);
          if (!taskId) throw new Error(t("agents.files_read_waiting"));
          showToast(t("agents.files_read_waiting"), "info");
          const st = await pollTask(agentId, taskId, { timeoutMs: 90_000 });
          if (st.status === "failed") throw new Error(st.error || t("agents.files_read_failed"));
          raw = st.result || "";
        } else {
          const rec = data as { content?: string; data?: string };
          raw = rec.content || rec.data || "";
        }
        const preview = fileReadPreview(raw, isImageFile(filename));
        if (!preview.content && looksLikeFileTaskAckJson(raw)) throw new Error(t("agents.files_read_waiting"));
        setPreviewContent(preview.content);
        setPreviewIsImage(preview.isImage);
        setShowPreview(true);
        setSelectedFile(filename);
      } catch (err) {
        showToast(String(err), "error");
      }
    },
    [agentId, currentPath, osType, showToast, t],
  );

  const deleteFile = useCallback(
    async (filename: string) => {
      const path = joinPath(currentPath, filename, osType);
      try {
        const data = await api.post(paths.agents.filesDelete(agentId), { path });
        showToast(t("agents.files_delete_queued", { filename }), "info");
        if (isFileTaskAck(data)) {
          const taskId = fileTaskId(data);
          if (taskId) {
            const st = await pollTask(agentId, taskId, { timeoutMs: 90_000 });
            if (st.status === "failed") throw new Error(st.error || t("agents.files_delete_failed"));
          }
        }
        await loadDirectory(currentPath);
        if (selectedFile === filename) setSelectedFile(null);
      } catch (err) {
        showToast(String(err), "error");
      }
    },
    [agentId, currentPath, osType, selectedFile, loadDirectory, showToast, t],
  );

  const uploadFile = useCallback(
    (file: File, onDone?: () => void) => {
      const dest = joinPath(currentPath, file.name, osType);
      setUploading(true);
      setUploadProgress(0);
      const abort = new AbortController();
      uploadAbortRef.current = abort;
      showToast(t("agents.files_push_queued", { name: file.name }), "info");
      void pushLocalFile({
        agentId,
        destPath: dest,
        file,
        t,
        onProgress: setUploadProgress,
        signal: abort.signal,
      })
        .then(() => {
          if (abort.signal.aborted) return;
          showToast(t("agents.files_push_done", { name: file.name }), "success");
          onDone?.();
          void loadDirectory(currentPath);
        })
        .catch((err) => {
          if (abort.signal.aborted) return;
          showToast(err instanceof Error ? err.message : t("agents.files_push_failed"), "error");
        })
        .finally(() => {
          if (!abort.signal.aborted) {
            setUploading(false);
            setUploadProgress(0);
          }
        });
    },
    [agentId, currentPath, loadDirectory, osType, showToast, t],
  );

  const loadDrives = useCallback(async () => {
    try {
      showToast(t("agents.files_drives_waiting"), "info");
      const data = await api.post(paths.agents.drives(agentId));
      const taskId = fileTaskId(data);
      if (!taskId) throw new Error(t("agents.files_drives_waiting"));
      const st = await pollTask(agentId, taskId, { timeoutMs: 90_000 });
      if (st.status === "failed") throw new Error(st.error || t("agents.files_no_drives"));
      setDriveOutput(st.result || "");
      setShowDrives(true);
    } catch (err) {
      showToast(String(err), "error");
    }
  }, [agentId, showToast, t]);

  const findFiles = useCallback(async () => {
    if (!findPattern.trim()) return;
    try {
      showToast(t("agents.files_find_queued"), "info");
      const data = await api.post(paths.agents.find(agentId), { pattern: findPattern, path: currentPath });
      const taskId = fileTaskId(data);
      if (!taskId) throw new Error(t("agents.files_find_failed"));
      const st = await pollTask(agentId, taskId, { timeoutMs: 180_000 });
      if (st.status === "failed") throw new Error(st.error || t("agents.files_find_failed"));
      const results = parseFindResult(st.result || "");
      setFindResults(results);
      showToast(t("agents.files_found_results", { n: results.length }), results.length > 0 ? "success" : "info");
    } catch (err) {
      showToast(String(err), "error");
    }
  }, [agentId, findPattern, currentPath, showToast, t]);

  const quickPaths = useMemo(
    () =>
      osType === "linux"
        ? [
            { label: t("agents.files_quick_home"), path: "/home" },
            { label: t("agents.files_quick_temp"), path: "/tmp" },
            { label: t("agents.files_quick_etc"), path: "/etc" },
            { label: t("agents.files_quick_var"), path: "/var" },
            { label: t("agents.files_quick_opt"), path: "/opt" },
            { label: t("agents.files_quick_root"), path: "/" },
          ]
        : [
            { label: t("agents.files_quick_home"), path: "C:\\Users" },
            { label: t("agents.files_quick_temp"), path: "C:\\Windows\\Temp" },
            { label: t("agents.files_quick_desktop"), path: "C:\\Users\\Public\\Desktop" },
            { label: t("agents.files_quick_documents"), path: "C:\\Users\\Public\\Documents" },
            { label: t("agents.files_quick_system32"), path: "C:\\Windows\\System32" },
            { label: t("agents.files_quick_programdata"), path: "C:\\ProgramData" },
          ],
    [osType, t],
  );

  return {
    currentPath,
    currentPathInput,
    setCurrentPathInput,
    entries,
    loading,
    selectedFile,
    setSelectedFile,
    uploadProgress,
    uploading,
    pullName,
    pullProgress,
    previewContent,
    previewIsImage,
    showPreview,
    setShowPreview,
    driveOutput,
    showDrives,
    setShowDrives,
    findPattern,
    setFindPattern,
    findResults,
    setFindResults,
    osType,
    quickPaths,
    loadDirectory,
    navigateTo,
    downloadFile,
    readFile,
    deleteFile,
    uploadFile,
    loadDrives,
    findFiles,
  };
}
