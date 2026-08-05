"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { API_BASE } from "@/lib/constants";
import { api, getCsrfToken } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import {
  isImageFile,
  joinPath,
  parseDrives,
  type DriveInfo,
  type FileEntry,
  type RawDrive,
} from "./types";

export function useAgentFiles(agentId: string) {
  const { t } = useI18n();
  const [currentPath, setCurrentPath] = useState("C:\\");
  const [currentPathInput, setCurrentPathInput] = useState("C:\\");
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploading, setUploading] = useState(false);
  const [previewContent, setPreviewContent] = useState("");
  const [previewIsImage, setPreviewIsImage] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [drives, setDrives] = useState<DriveInfo[]>([]);
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
        const data = await api.post<{
          Files?: FileEntry[];
          files?: FileEntry[];
          Entries?: FileEntry[];
          entries?: FileEntry[];
          data?: FileEntry[];
        }>(paths.agents.filesLs(agentId), { path }, { signal: ac.signal });
        if (ac.signal.aborted) return;
        const items: FileEntry[] = data.files || data.entries || data.data || data.Files || data.Entries || [];
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
    [agentId, showToast],
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
      const path = joinPath(currentPath, filename, osType);
      try {
        const { blob } = await api.download(paths.agents.download(agentId), { path });
        downloadBlob(blob, filename);
        showToast(t("agents.files_downloaded", { filename }), "success");
      } catch (err) {
        showToast(String(err), "error");
      }
    },
    [agentId, currentPath, osType, showToast, t],
  );

  const readFile = useCallback(
    async (filename: string) => {
      const path = joinPath(currentPath, filename, osType);
      try {
        const data = await api.post<{ content?: string; data?: string }>(paths.agents.filesRead(agentId), { path });
        if (isImageFile(filename)) {
          const imgData = data.content || data.data || "";
          setPreviewContent(imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`);
          setPreviewIsImage(true);
        } else {
          setPreviewContent(data.content || data.data || JSON.stringify(data, null, 2));
          setPreviewIsImage(false);
        }
        setShowPreview(true);
        setSelectedFile(filename);
      } catch (err) {
        showToast(String(err), "error");
      }
    },
    [agentId, currentPath, osType, showToast],
  );

  const deleteFile = useCallback(
    async (filename: string) => {
      const path = joinPath(currentPath, filename, osType);
      try {
        await api.post(paths.agents.filesDelete(agentId), { path });
        showToast(t("agents.files_deleted", { filename }), "success");
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
      setUploading(true);
      setUploadProgress(0);
      const formData = new FormData();
      formData.append("file", file);
      formData.append("path", currentPath);
      const abort = new AbortController();
      uploadAbortRef.current = abort;
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${API_BASE}${paths.agents.cmd(agentId, "files/upload")}`);
      xhr.setRequestHeader("X-CSRF-Token", getCsrfToken());
      xhr.upload.addEventListener("progress", (evt) => {
        if (evt.lengthComputable) {
          setUploadProgress(Math.round((evt.loaded / evt.total) * 100));
        }
      });
      xhr.addEventListener("load", () => {
        if (abort.signal.aborted) return;
        setUploading(false);
        setUploadProgress(0);
        if (xhr.status >= 200 && xhr.status < 300) {
          showToast(t("agents.files_uploaded", { name: file.name }), "success");
          onDone?.();
          void loadDirectory(currentPath);
        } else {
          showToast(t("agents.files_upload_failed_status", { status: xhr.status }), "error");
        }
      });
      xhr.addEventListener("error", () => {
        if (abort.signal.aborted) return;
        setUploading(false);
        setUploadProgress(0);
        showToast(t("agents.files_upload_failed_network"), "error");
      });
      xhr.send(formData);
    },
    [agentId, currentPath, loadDirectory, showToast, t],
  );

  const loadDrives = useCallback(async () => {
    try {
      const data = await api.post<{ drives?: RawDrive[]; data?: RawDrive[] }>(paths.agents.drives(agentId));
      const raw: RawDrive[] = data.drives || data.data || [];
      setDrives(parseDrives(raw));
      setShowDrives(true);
    } catch (err) {
      showToast(String(err), "error");
    }
  }, [agentId, showToast]);

  const findFiles = useCallback(async () => {
    if (!findPattern.trim()) return;
    try {
      const data = await api.post<{
        Results?: string[];
        results?: string[];
        Files?: string[];
        files?: string[];
        data?: string[];
      }>(paths.agents.find(agentId), { pattern: findPattern, path: currentPath });
      const results: string[] = data.results || data.files || data.data || data.Results || data.Files || [];
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
            { label: "Home", path: "/home" },
            { label: "Temp", path: "/tmp" },
            { label: "Etc", path: "/etc" },
            { label: "Var", path: "/var" },
            { label: "Opt", path: "/opt" },
            { label: "Root", path: "/" },
          ]
        : [
            { label: "Home", path: "C:\\Users" },
            { label: "Temp", path: "C:\\Windows\\Temp" },
            { label: "Desktop", path: "C:\\Users\\Public\\Desktop" },
            { label: "Documents", path: "C:\\Users\\Public\\Documents" },
            { label: "System32", path: "C:\\Windows\\System32" },
            { label: "ProgramData", path: "C:\\ProgramData" },
          ],
    [osType],
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
    previewContent,
    previewIsImage,
    showPreview,
    setShowPreview,
    drives,
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
