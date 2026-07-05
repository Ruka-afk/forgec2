"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { API_BASE } from "@/lib/constants";
import { ConfirmModal } from "@/components/UI";

interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mod_time: string;
}

interface DriveInfo {
  letter: string;
  label: string;
  total: number;
  free: number;
}

type RawDrive = string | {
  letter?: string;
  path?: string;
  name?: string;
  label?: string;
  volume_label?: string;
  total?: number;
  total_size?: number;
  free?: number;
  free_space?: number;
};

function formatSize(bytes: number): string {
  if (bytes === 0 || bytes === undefined || bytes === null) return "-";
  const units = ["B", "KB", "MB", "GB"];
  let i = 0;
  let size = bytes;
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024;
    i++;
  }
  return `${size.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function getFileIcon(entry: FileEntry): string {
  if (entry.is_dir) return "📁";
  const ext = entry.name.split(".").pop()?.toLowerCase() || "";
  if (["txt", "log", "json", "xml", "ini", "conf", "yaml", "yml", "md", "conf", "cfg", "env"].includes(ext)) return "📄";
  if (["jpg", "jpeg", "png", "gif", "bmp", "ico", "svg", "webp", "tiff"].includes(ext)) return "🖼️";
  if (["exe", "dll", "sys", "bat", "ps1", "vbs", "msi", "com", "scr"].includes(ext)) return "⚙️";
  if (["zip", "rar", "7z", "tar", "gz", "bz2", "xz"].includes(ext)) return "📦";
  if (["doc", "docx", "xls", "xlsx", "ppt", "pptx", "pdf", "rtf", "odt"].includes(ext)) return "📄";
  if (["mp3", "wav", "flac", "ogg", "aac"].includes(ext)) return "🎵";
  if (["mp4", "avi", "mkv", "mov", "wmv", "flv"].includes(ext)) return "🎬";
  if (["iso", "img", "vmdk", "vhd"].includes(ext)) return "💿";
  if (["db", "sqlite", "sql", "mdb"].includes(ext)) return "🗄️";
  return "📄";
}

function isImageFile(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() || "";
  return ["jpg", "jpeg", "png", "gif", "bmp", "ico", "svg", "webp", "tiff"].includes(ext);
}

function formatTimestamp(ts: string): string {
  if (!ts) return "-";
  try {
    const d = new Date(ts);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  } catch {
    return ts;
  }
}

export default function FilesPage({ params }: { params: Promise<{ id: string }> }) {
  const [id, setId] = useState<string>("");
  const [currentPath, setCurrentPath] = useState("C:\\");
  const [currentPathInput, setCurrentPathInput] = useState("C:\\");
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [showUpload, setShowUpload] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploading, setUploading] = useState(false);
  const [previewContent, setPreviewContent] = useState<string>("");
  const [previewIsImage, setPreviewIsImage] = useState(false);
  const [showPreview, setShowPreview] = useState(false);
  const [toast, setToast] = useState<{ text: string; type: string } | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showDrives, setShowDrives] = useState(false);
  const [drives, setDrives] = useState<DriveInfo[]>([]);
  const [showFind, setShowFind] = useState(false);
  const [findPattern, setFindPattern] = useState("");
  const [findResults, setFindResults] = useState<string[]>([]);
  const [osType, setOsType] = useState<"windows" | "linux">("windows");
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  useEffect(() => {
    let cancelled = false;
    params.then(({ id }) => { if (!cancelled) setId(id); });
    return () => { cancelled = true; };
  }, [params]);

  const showToast = useCallback((text: string, type: string = "info") => {
    setToast({ text, type });
    setTimeout(() => setToast(null), 3000);
  }, []);

  const loadDirectory = useCallback(async (path: string) => {
    if (!id) return;
    setLoading(true);
    const body = new URLSearchParams();
    body.append("path", path);
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/files/ls`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      if (res.ok) {
        const data = await res.json();
        const items: FileEntry[] = data.Files || data.files || data.Entries || data.entries || data.data || [];
        items.sort((a, b) => {
          if (a.is_dir && !b.is_dir) return -1;
          if (!a.is_dir && b.is_dir) return 1;
          return a.name.localeCompare(b.name);
        });
        setEntries(items);
        setCurrentPath(path);
        setCurrentPathInput(path);
      } else {
        showToast(`Failed to load directory: ${res.status}`, "error");
        setEntries([]);
      }
    } catch (err) {
      showToast(String(err), "error");
      setEntries([]);
    }
    setLoading(false);
  }, [id, showToast]);

  const detectOs = useCallback(async (): Promise<string> => {
    if (!id) return "windows";
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}&format=json`);
      if (res.ok) {
        const data = await res.json();
        const agentData = data.Agent || data.agent || {};
        const os = (agentData.os || agentData.OS || "windows").toLowerCase();
        if (os.includes("linux") || os.includes("darwin") || os.includes("unix")) {
          setOsType("linux");
          return "linux";
        }
      }
    } catch (e) { console.error("Files: detect OS failed", e); }
    return "windows";
  }, [id]);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      const os = await detectOs();
      if (cancelled) return;
      const initialPath = os === "linux" ? "/" : "C:\\";
      loadDirectory(initialPath);
    })();
    return () => { cancelled = true; };
  }, [id, detectOs, loadDirectory]);

  const navigateTo = (path: string) => {
    loadDirectory(path);
    setSelectedFile(null);
  };

  const goUp = () => {
    const sep = osType === "linux" ? "/" : "\\";
    const cleaned = currentPath.replace(/[\\/]+$/, "");
    const parts = cleaned.split(/[\\/]/).filter(Boolean);
    if (parts.length > 0) {
      parts.pop();
      if (osType === "linux") {
        navigateTo("/" + parts.join("/"));
      } else {
        const parent = parts.join("\\");
        navigateTo(parent ? parent + "\\" : "C:\\");
      }
    }
  };

  const handleBreadcrumbClick = (index: number) => {
    const sep = osType === "linux" ? "/" : "\\";
    const parts = currentPath.split(/[\\/]/).filter(Boolean);
    if (index === 0) {
      navigateTo(osType === "linux" ? "/" : "C:\\");
    } else {      const sliced = parts.slice(0, index);      navigateTo(osType === "linux" ? "/" + sliced.join("/") : sliced.join(sep));    }
  };

  const handleFileClick = (entry: FileEntry) => {
    if (entry.is_dir) {
      const sep = osType === "linux" ? "/" : "\\";
      navigateTo(currentPath.replace(/[\\/]+$/, "") + sep + entry.name);
    } else {
      setSelectedFile(entry.name);
    }
  };

  const handleDownload = async (filename: string) => {
    const sep = osType === "linux" ? "/" : "\\";
    const path = currentPath.replace(/[\\/]+$/, "") + sep + filename;
    const body = new URLSearchParams();    body.append("path", path);    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/download`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      if (res.ok) {        const blob = await res.blob();        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");        a.href = url;
        a.download = filename;
        a.click();
        URL.revokeObjectURL(url);
        showToast(`Downloaded ${filename}`, "success");
      } else {
        showToast(`Download failed: ${res.status}`, "error");
      }    } catch (err) {      showToast(String(err), "error");
    }  };

  const handleRead = async (filename: string) => {
    const sep = osType === "linux" ? "/" : "\\";    const path = currentPath.replace(/[\\/]+$/, "") + sep + filename;
    const body = new URLSearchParams();
    body.append("path", path);
    try {
      if (isImageFile(filename)) {        const res = await fetch(`${API_BASE}?p=/agents/${id}/files/read`, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: body.toString(),
        });
        if (res.ok) {
          const data = await res.json();
          const imgData = data.content || data.data || "";
          setPreviewContent(imgData.startsWith("data:") ? imgData : `data:image/png;base64,${imgData}`);
          setPreviewIsImage(true);
          setShowPreview(true);
          setSelectedFile(filename);
        } else {
          showToast(`Read failed: ${res.status}`, "error");
        }
      } else {
        const res = await fetch(`${API_BASE}?p=/agents/${id}/files/read`, {
          method: "POST",          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: body.toString(),
        });
        if (res.ok) {
          const data = await res.json();          setPreviewContent(data.content || data.data || JSON.stringify(data, null, 2));
          setPreviewIsImage(false);
          setShowPreview(true);          setSelectedFile(filename);
        } else {
          showToast(`Read failed: ${res.status}`, "error");
        }
      }
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const handleDelete = (filename: string) => {
    const sep = osType === "linux" ? "/" : "\\";
    const path = currentPath.replace(/[\\/]+$/, "") + sep + filename;
    setCfm({msg: `Are you sure you want to delete "${filename}"?`, cb: async () => {
      const body = new URLSearchParams();
      body.append("path", path);
      try {
        const res = await fetch(`${API_BASE}?p=/agents/${id}/files/delete`, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: body.toString(),
        });
        if (res.ok) {
          showToast(`Deleted ${filename}`, "success");
          loadDirectory(currentPath);
          if (selectedFile === filename) setSelectedFile(null);
        } else {
          showToast(`Delete failed: ${res.status}`, "error");
        }
      } catch (err) {
        showToast(String(err), "error");
      }
    }});
  };

  const handleUpload = async (e: React.FormEvent<HTMLFormElement>) => {    e.preventDefault();
    if (!fileInputRef.current?.files?.length) return;
    const file = fileInputRef.current.files[0];    setUploading(true);
    setUploadProgress(0);
    const formData = new FormData();
    formData.append("file", file);
    formData.append("path", currentPath);
    try {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${API_BASE}?p=/agents/${id}/files/upload`);
      xhr.upload.addEventListener("progress", (evt) => {
        if (evt.lengthComputable) {          setUploadProgress(Math.round((evt.loaded / evt.total) * 100));        }
      });
      xhr.addEventListener("load", () => {
        setUploading(false);
        setUploadProgress(0);
        if (xhr.status >= 200 && xhr.status < 300) {
          showToast(`Uploaded ${file.name}`, "success");
          setShowUpload(false);
          loadDirectory(currentPath);
        } else {
          showToast(`Upload failed: ${xhr.status}`, "error");
        }
      });      xhr.addEventListener("error", () => {
        setUploading(false);        setUploadProgress(0);
        showToast("Upload failed: network error", "error");
      });
      xhr.send(formData);
    } catch (err) {
      setUploading(false);
      setUploadProgress(0);      showToast(String(err), "error");    }
  };

  const loadDrives = async () => {
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/drives`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: "",
      });
      if (res.ok) {
        const data = await res.json();        const rawDrives: RawDrive[] = data.drives || data.data || [];
        const parsed: DriveInfo[] = rawDrives.map((d: RawDrive) => {          if (typeof d === "string") {
            return { letter: d, label: "", total: 0, free: 0 };
          }
          return {
            letter: d.letter || d.path || d.name || "??",
            label: d.label || d.volume_label || "",
            total: d.total || d.total_size || 0,
            free: d.free || d.free_space || 0,
          };        });
        setDrives(parsed);
        setShowDrives(true);
      } else {
        showToast(`Failed to load drives: ${res.status}`, "error");
      }
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const handleFind = async () => {
    if (!findPattern.trim()) return;
    const sep = osType === "linux" ? "/" : "\\";    const body = new URLSearchParams();    body.append("pattern", findPattern);    body.append("path", currentPath);
    try {
      const res = await fetch(`${API_BASE}?p=/agents/${id}/find`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded" },
        body: body.toString(),
      });
      if (res.ok) {        const data = await res.json();
        const results: string[] = data.Results || data.results || data.Files || data.files || data.data || [];
        setFindResults(results);
        showToast(`Found ${results.length} results`, results.length > 0 ? "success" : "info");
      } else {
        showToast(`Search failed: ${res.status}`, "error");
      }
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const quickPaths = osType === "linux"    ? [        { label: "Home", path: "/home" },
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
      ];

  const sep = osType === "linux" ? "/" : "\\";
  const rootPath = osType === "linux" ? "/" : "C:\\";
  const pathParts = currentPath.split(/[\\/]/).filter(Boolean);

  return (
    <>
      {toast && (
        <div className={`fixed top-4 right-4 z-50 px-4 py-3 rounded-xl text-sm font-medium shadow-lg ${          toast.type === "success" ? "bg-emerald-600 text-white" :          toast.type === "error" ? "bg-red-600 text-white" :
          "bg-blue-600 text-white"
        }`}>          {toast.text}
        </div>
      )}

      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <a href={`/agents/${id}`} className="text-gray-400 hover:text-gray-200">
            <i className="fa-solid fa-arrow-left"></i>
          </a>          <h1 className="text-xl font-bold">          <i className="fa-solid fa-folder-tree text-amber-400 mr-2"></i>
          File Browser
          </h1>
          <span className="text-xs text-gray-500 font-mono bg-slate-800 dark:bg-slate-800 px-2 py-0.5 rounded">{id}</span>
        </div>        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowFind(!showFind)}
            className="bg-purple-600 hover:bg-purple-500 text-white text-xs px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5"
          >            <i className="fa-solid fa-magnifying-glass"></i>
            Find
          </button>
          <button
            onClick={() => setShowUpload(true)}
            className="bg-emerald-600 hover:bg-emerald-500 text-white text-xs px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5"
          >
            <i className="fa-solid fa-cloud-arrow-up"></i>
            Upload
          </button>
          <button
            onClick={loadDrives}
            className="bg-indigo-600 hover:bg-indigo-500 text-white text-xs px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5"
          >
            <i className="fa-solid fa-hard-drive"></i>
            Drives
          </button>
          <button            onClick={() => loadDirectory(currentPath)}
            className="bg-slate-700 hover:bg-slate-600 text-white text-xs px-4 h-9 rounded-xl transition-colors flex items-center gap-1.5"
          >
            <i className="fa-solid fa-rotate"></i>
            Refresh
          </button>
        </div>
      </div>

      <div className="bg-slate-100 dark:bg-slate-800 border border-[var(--border)] rounded-2xl p-5 mb-4 shadow-sm">
        <div className="flex items-center gap-3 mb-3">
          <i className="fa-solid fa-folder-open text-blue-500"></i>
          <input
            type="text"
            value={currentPathInput}            onChange={(e) => setCurrentPathInput(e.target.value)}
            onKeyDown={(e) => {              if (e.key === "Enter") {                navigateTo(currentPathInput);
              }            }}
            className="flex-1 ui-card px-4 h-10 text-sm font-mono focus:outline-none focus:border-blue-500 dark:text-slate-100"          />
          <button
            onClick={() => navigateTo(currentPathInput)}
            className="bg-blue-600 hover:bg-blue-700 text-white px-4 h-10 rounded-xl text-sm transition-colors flex items-center gap-1.5"
          >
            <i className="fa-solid fa-arrow-right"></i> Go
          </button>
          <button            onClick={goUp}
            className="bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 px-4 h-10 rounded-xl text-sm transition-colors flex items-center gap-1.5"          >
            <i className="fa-solid fa-arrow-turn-up"></i> Up
          </button>
        </div>        <div className="flex items-center gap-1.5 text-sm">
          <button
            onClick={() => navigateTo(rootPath)}
            className={`font-mono transition-colors px-2 py-1 rounded-lg ${
              currentPath === rootPath
                ? "text-slate-900 dark:text-slate-100 bg-blue-100 dark:bg-blue-900/30"
                : "text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20"
            }`}
          >            {rootPath}
          </button>
          {pathParts.map((part, i) => (
            <span key={i} className="flex items-center gap-1.5">
              <span className="text-slate-400 dark:text-slate-500 font-mono">{sep === "/" ? "/" : "\\"}</span>
              <button
                onClick={() => handleBreadcrumbClick(i + 1)}
                className={`font-mono transition-colors px-2 py-1 rounded-lg hover:bg-blue-50 dark:hover:bg-blue-900/20 ${                  i === pathParts.length - 1 ? "text-slate-900 dark:text-slate-100 bg-blue-100 dark:bg-blue-900/30" : "text-blue-600 dark:text-blue-400"
                }`}
              >
                {part}
              </button>
            </span>
          ))}
        </div>

        <div className="flex items-center gap-2 mt-3 text-xs">          <span className="text-slate-500 dark:text-slate-400 font-medium">Quick:</span>
          {quickPaths.map((qp) => (
            <button
              key={qp.path}
              onClick={() => navigateTo(qp.path)}
              className="text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 px-2.5 py-1 rounded-lg transition-colors font-medium"            >
              {qp.label}
            </button>
          ))}
        </div>
      </div>

      {showFind && (
        <div className="ui-card p-5 mb-4 shadow-sm">
          <div className="font-semibold text-sm text-slate-900 dark:text-slate-100 mb-3 flex items-center gap-2">
            <i className="fa-solid fa-magnifying-glass text-purple-500"></i>
            Search Files
          </div>
          <div className="flex gap-3">
            <input
              type="text"
              value={findPattern}              onChange={(e) => setFindPattern(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleFind()}
              placeholder="e.g. *.txt, *.doc, secret*"
              className="flex-1 bg-slate-50 dark:bg-slate-700 border border-[var(--border)] rounded-xl px-4 h-10 text-sm font-mono focus:outline-none focus:border-purple-500 dark:text-slate-100"
            />
            <button
              onClick={handleFind}
              className="bg-purple-600 hover:bg-purple-700 text-white px-5 h-10 rounded-xl text-sm transition-colors flex items-center gap-1.5"
            >
              <i className="fa-solid fa-magnifying-glass"></i> Search
            </button>
            {findResults.length > 0 && (
              <button
                onClick={() => { setFindResults([]); setFindPattern(""); }}
                className="bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-[var(--text-secondary)] px-4 h-10 rounded-xl text-sm transition-colors flex items-center gap-1.5"
              >
                <i className="fa-solid fa-xmark"></i> Clear
              </button>
            )}
          </div>
          {findResults.length > 0 && (
            <div className="mt-3 max-h-48 overflow-y-auto border border-[var(--border)] rounded-xl bg-slate-50 dark:bg-slate-700/50">
              {findResults.map((result, i) => (
                <button
                  key={i}
                  onClick={() => {
                    const parentDir = result.substring(0, result.lastIndexOf(sep)) || currentPath;                    const fileName = result.split(sep).pop() || "";
                    navigateTo(parentDir);
                    setSelectedFile(fileName);                  }}
                  className="w-full text-left px-4 py-2 text-xs font-mono text-[var(--text-secondary)] hover:bg-purple-50 dark:hover:bg-purple-900/20 border-b border-[var(--border)] last:border-0 transition-colors flex items-center gap-2"
                >
                  <i className="fa-solid fa-file text-slate-400"></i>
                  {result}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {showDrives && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowDrives(false)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">            <div className="bg-gradient-to-r from-indigo-500 to-indigo-700 px-6 py-5 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-white flex items-center gap-2">
                <i className="fa-solid fa-hard-drive"></i>
                Available Drives
              </h3>
              <button onClick={() => setShowDrives(false)} className="text-white/80 hover:text-white">
                <i className="fa-solid fa-xmark text-lg"></i>
              </button>            </div>
            <div className="p-5 max-h-64 overflow-y-auto">
              {drives.length === 0 ? (
                <p className="text-slate-400 dark:text-slate-500 text-sm text-center py-4">No drives available</p>              ) : (
                <div className="space-y-2">
                  {drives.map((drive, i) => (
                    <button
                      key={i}                      onClick={() => {                        navigateTo(drive.letter);                        setShowDrives(false);
                      }}
                      className="w-full flex items-center gap-3 px-4 py-3 bg-slate-50 dark:bg-slate-700 hover:bg-indigo-50 dark:hover:bg-indigo-900/20 rounded-xl transition-colors"
                    >
                      <i className="fa-solid fa-hard-drive text-indigo-500 text-xl"></i>
                      <div className="text-left">                        <div className="text-sm font-medium text-slate-900 dark:text-slate-100">
                          {drive.letter}{drive.label ? ` (${drive.label})` : ""}
                        </div>                        {drive.total > 0 && (
                          <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                            {formatSize(drive.free)} free of {formatSize(drive.total)}                            <div className="w-full bg-slate-200 dark:bg-slate-600 rounded-full h-1.5 mt-1">
                              <div                                className="bg-indigo-500 h-1.5 rounded-full"                                style={{ width: `${Math.max(0, 100 - (drive.free / drive.total) * 100)}%` }}
                              ></div>
                            </div>
                          </div>
                        )}
                      </div>
                    </button>                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}      {loading && (
        <div className="text-center py-8">
          <div className="inline-flex items-center gap-3">
            <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
            <p className="text-slate-400 dark:text-slate-500 text-sm">Loading files...</p>
          </div>
        </div>
      )}

      {!loading && entries.length === 0 && (        <div className="text-center py-10 ui-card">
          <i className="fa-regular fa-folder-open text-4xl text-slate-300 dark:text-slate-600 mb-3"></i>
          <p className="text-slate-400 dark:text-slate-500">Directory is empty or inaccessible</p>
        </div>
      )}      {!loading && entries.length > 0 && (
        <div className="ui-card overflow-hidden">
          <div className="overflow-x-auto">            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--border)] bg-slate-50 dark:bg-slate-700/50">
                  <th className="text-left py-3 px-4 text-xs font-semibold text-slate-500 dark:text-slate-400 w-12">Type</th>
                  <th className="text-left py-3 px-4 text-xs font-semibold text-slate-500 dark:text-slate-400">Name</th>
                  <th className="text-left py-3 px-4 text-xs font-semibold text-slate-500 dark:text-slate-400 w-24">Size</th>                  <th className="text-left py-3 px-4 text-xs font-semibold text-slate-500 dark:text-slate-400 w-40">Modified</th>
                  <th className="text-right py-3 px-4 text-xs font-semibold text-slate-500 dark:text-slate-400 w-44">Actions</th>
                </tr>
              </thead>              <tbody>
                {entries.map((entry, i) => (
                  <tr
                    key={i}
                    className={`border-b border-slate-100 dark:border-slate-700 last:border-0 hover:bg-blue-50 dark:hover:bg-blue-900/10 cursor-pointer transition-colors ${                      selectedFile === entry.name ? "bg-blue-50 dark:bg-blue-900/20" : ""
                    }`}
                    onClick={() => handleFileClick(entry)}                  >
                    <td className="py-2.5 px-4 text-center text-lg">                      {getFileIcon(entry)}
                    </td>
                    <td className="py-2.5 px-4">
                      <span className={`font-mono text-sm ${entry.is_dir ? "text-blue-600 dark:text-blue-400 font-medium" : "text-slate-700 dark:text-slate-200"}`}>
                        {entry.name}                      </span>
                    </td>
                    <td className="py-2.5 px-4 text-sm text-slate-500 dark:text-slate-400 font-mono">
                      {entry.is_dir ? "" : formatSize(entry.size)}
                    </td>                    <td className="py-2.5 px-4 text-xs text-slate-500 dark:text-slate-400 font-mono">
                      {formatTimestamp(entry.mod_time)}
                    </td>
                    <td className="py-2.5 px-4 text-right" onClick={(e) => e.stopPropagation()}>                      {!entry.is_dir && (
                        <div className="flex items-center justify-end gap-1">
                          <button                            onClick={() => handleRead(entry.name)}
                            className="text-xs px-2.5 py-1.5 text-purple-600 dark:text-purple-400 hover:bg-purple-50 dark:hover:bg-purple-900/20 rounded-lg transition-colors flex items-center gap-1"
                          >
                            <i className="fa-solid fa-eye"></i> Preview
                          </button>
                          <button                            onClick={() => handleDownload(entry.name)}
                            className="text-xs px-2.5 py-1.5 text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/20 rounded-lg transition-colors flex items-center gap-1"
                          >
                            <i className="fa-solid fa-download"></i> DL
                          </button>
                          <button
                            onClick={() => handleDelete(entry.name)}
                            className="text-xs px-2.5 py-1.5 text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors flex items-center gap-1"
                          >
                            <i className="fa-solid fa-trash"></i>                           </button>
                        </div>
                      )}                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>        </div>
      )}

      {showUpload && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => !uploading && setShowUpload(false)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-md mx-4 overflow-hidden">
            <div className="bg-gradient-to-r from-emerald-500 to-emerald-700 px-6 py-5 flex items-center justify-between">
              <h3 className="text-lg font-semibold text-white flex items-center gap-2">
                <i className="fa-solid fa-cloud-arrow-up"></i>
                Upload File
              </h3>              {!uploading && (
                <button onClick={() => setShowUpload(false)} className="text-white/80 hover:text-white">
                  <i className="fa-solid fa-xmark text-lg"></i>
                </button>
              )}
            </div>
            <form onSubmit={handleUpload} className="px-6 py-5 space-y-4">
              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5 block">Destination Path</label>                <input
                  type="text"
                  value={currentPath}
                  readOnly                  className="w-full bg-slate-50 dark:bg-slate-700 border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 text-slate-600 dark:text-slate-300 font-mono"                />
              </div>
              <div>                <label className="text-xs font-medium text-slate-500 dark:text-slate-400 mb-1.5 block">Select File</label>
                <input
                  ref={fileInputRef}
                  type="file"                  className="w-full bg-[var(--card-bg)] border border-[var(--border)] text-sm rounded-xl px-3 py-2.5 dark:text-slate-100"
                />
              </div>
              {uploading && (
                <div>
                  <div className="flex justify-between text-xs text-slate-500 dark:text-slate-400 mb-1">
                    <span>Uploading...</span>
                    <span>{uploadProgress}%</span>
                  </div>
                  <div className="w-full bg-slate-200 dark:bg-slate-600 rounded-full h-2">                    <div                      className="bg-emerald-500 h-2 rounded-full transition-all duration-200"
                      style={{ width: `${uploadProgress}%` }}                    ></div>
                  </div>
                </div>              )}
              <div className="flex gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowUpload(false)}
                  disabled={uploading}                  className="flex-1 px-4 py-2.5 bg-slate-100 hover:bg-slate-200 disabled:opacity-50 text-slate-700 dark:bg-slate-700 dark:text-slate-300 rounded-xl text-sm font-medium transition-colors flex items-center justify-center gap-1.5"
                >
                  <i className="fa-solid fa-xmark"></i> Cancel
                </button>
                <button
                  type="submit"
                  disabled={uploading}
                  className="flex-1 px-4 py-2.5 bg-emerald-600 hover:bg-emerald-700 disabled:opacity-50 text-white rounded-xl text-sm font-medium transition-colors flex items-center justify-center gap-1.5"
                >
                  {uploading ? (                    <><i className="fa-solid fa-spinner fa-spin"></i> Uploading...</>
                  ) : (
                    <><i className="fa-solid fa-cloud-arrow-up"></i> Upload</>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {showPreview && previewContent && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={() => setShowPreview(false)} />
          <div className="relative bg-[var(--card-bg)] rounded-2xl shadow-2xl w-full max-w-3xl mx-4 overflow-hidden max-h-[80vh] flex flex-col">            <div className={`px-6 py-5 flex items-center justify-between ${              previewIsImage ? "bg-gradient-to-r from-pink-500 to-pink-700" : "bg-gradient-to-r from-indigo-500 to-indigo-700"
            }`}>
              <h3 className="text-lg font-semibold text-white flex items-center gap-2">
                {                  previewIsImage ? (
                    <><i className="fa-solid fa-image"></i> Image: {selectedFile}</>
                  ) : (
                    <><i className="fa-solid fa-file-lines"></i> Preview: {selectedFile}</>
                  )
                }
              </h3>
              <div className="flex items-center gap-2">                {!previewIsImage && (
                  <button
                    onClick={() => {                      const blob = new Blob([previewContent], { type: "text/plain" });
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement("a");
                      a.href = url;
                      a.download = selectedFile || "preview.txt";
                      a.click();
                      URL.revokeObjectURL(url);
                    }}                    className="text-white/80 hover:text-white transition-colors p-1"
                    title="Download content"
                  >
                    <i className="fa-solid fa-download"></i>
                  </button>
                )}
                <button onClick={() => setShowPreview(false)} className="text-white/80 hover:text-white">
                  <i className="fa-solid fa-xmark text-lg"></i>
                </button>
              </div>
            </div>{previewIsImage ? (
              <div className="p-4 flex-1 flex items-center justify-center bg-slate-100 dark:bg-slate-900 overflow-auto">                <img src={previewContent} alt={selectedFile || "Preview"} className="max-w-full max-h-[70vh] object-contain rounded-lg shadow-md" />              </div>
            ) : (
              <div className="p-6 overflow-auto flex-1">
                <pre className="text-sm font-mono whitespace-pre-wrap text-slate-700 dark:text-slate-200 bg-slate-50 dark:bg-slate-900 rounded-xl p-4 border border-[var(--border)]">
                  {previewContent}
                </pre>              </div>
            )}
          </div>
        </div>
      )}
      <ConfirmModal open={!!cfm} title="Confirm" message={cfm?.msg || ""} confirmText="Delete" cancelText="Cancel" danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </>
  );
}
