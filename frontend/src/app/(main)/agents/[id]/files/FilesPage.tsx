"use client";

import { useState, useEffect, useCallback, useRef, useMemo } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { API_BASE } from "@/lib/constants";
import { api, getCsrfToken } from "@/lib/api";
import { downloadBlob, downloadText } from "@/lib/download";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ConfirmModal, Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
import { toast } from "sonner"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { ArrowLeft, ArrowRight, CloudUpload, Download, Eye, File, FileText, FolderOpen, FolderTree, HardDrive, ImageIcon, RotateCw, Search, Trash2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useI18n } from "@/lib/i18n";

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

export default function FilesPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
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
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadAbortRef = useRef<AbortController | null>(null);
  const [showDrives, setShowDrives] = useState(false);
  const [drives, setDrives] = useState<DriveInfo[]>([]);
  const [showFind, setShowFind] = useState(false);
  const [findPattern, setFindPattern] = useState("");
  const [findResults, setFindResults] = useState<string[]>([]);
  const [osType, setOsType] = useState<"windows" | "linux">("windows");
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);

  const showToast = useCallback((text: string, type: string = "info") => {
    if (type === "success") { toast.success(text); }
    else if (type === "error") { toast.error(text); }
    else { toast(text); }
  }, []);

  const loadDirectory = useCallback(async (path: string) => {
    if (!id) return;
    setLoading(true);
    try {
      const data = await api.post<{ Files?: FileEntry[]; files?: FileEntry[]; Entries?: FileEntry[]; entries?: FileEntry[]; data?: FileEntry[] }>(`/agents/${id}/files/ls`, { path });
      const items: FileEntry[] = data.files || data.entries || data.data || [];
      items.sort((a, b) => {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        return a.name.localeCompare(b.name);
      });
      setEntries(items);
      setCurrentPath(path);
      setCurrentPathInput(path);
    } catch (err) {
      showToast(String(err), "error");
      setEntries([]);
    }
    setLoading(false);
  }, [id, showToast]);

  const detectOs = useCallback(async (): Promise<string> => {
    if (!id) return "windows";
    try {
      const data = await api.get<{ Agent?: { os?: string; OS?: string }; agent?: { os?: string; OS?: string } }>(`/agents/${id}?format=json`);
      const agentData = data.agent || {};
      const os = (agentData.os || "windows").toLowerCase();
      if (os.includes("linux") || os.includes("darwin") || os.includes("unix")) {
        setOsType("linux");
        return "linux";
      }
    } catch { toast.error(t("agents.files_detect_os_failed")); }
    return "windows";
  }, [id, t]);

  useEffect(() => {
    if (!id) return;
    let cancelled = false;
    (async () => {
      const os = await detectOs();
      if (cancelled) return;
      const initialPath = os === "linux" ? "/" : "C:\\";
      loadDirectory(initialPath);
    })();
    return () => {
      cancelled = true;
      if (uploadAbortRef.current) {
        uploadAbortRef.current.abort();
        uploadAbortRef.current = null;
      }
    };
  }, [id, detectOs, loadDirectory]);

  const navigateTo = (path: string) => {
    loadDirectory(path);
    setSelectedFile(null);
  };

  const goUp = () => {
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
    try {
      const { blob } = await api.download(`/agents/${id}/download`, { path });
      downloadBlob(blob, filename);
      showToast(t("agents.files_downloaded", { filename }), "success");
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const handleRead = async (filename: string) => {
    const sep = osType === "linux" ? "/" : "\\";
    const path = currentPath.replace(/[\\/]+$/, "") + sep + filename;
    try {
      const data = await api.post<{ content?: string; data?: string }>(`/agents/${id}/files/read`, { path });
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
  };

  const handleDelete = (filename: string) => {
    const sep = osType === "linux" ? "/" : "\\";
    const path = currentPath.replace(/[\\/]+$/, "") + sep + filename;
    setCfm({msg: t("agents.files_confirm_delete"), cb: async () => {
      try {
        await api.post(`/agents/${id}/files/delete`, { path });
        showToast(t("agents.files_deleted", { filename }), "success");
        loadDirectory(currentPath);
        if (selectedFile === filename) setSelectedFile(null);
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
    const abort = new AbortController();
    uploadAbortRef.current = abort;
    try {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `${API_BASE}/agents/${id}/files/upload`);
      xhr.setRequestHeader("X-CSRF-Token", getCsrfToken());
      xhr.upload.addEventListener("progress", (evt) => {
        if (evt.lengthComputable) {          setUploadProgress(Math.round((evt.loaded / evt.total) * 100));        }
      });
      xhr.addEventListener("load", () => {
        if (abort.signal.aborted) return;
        setUploading(false);
        setUploadProgress(0);
        if (xhr.status >= 200 && xhr.status < 300) {
          showToast(t("agents.files_uploaded", { name: file.name }), "success");
          setShowUpload(false);
          loadDirectory(currentPath);
        } else {
          showToast(t("agents.files_upload_failed_status", { status: xhr.status }), "error");
        }
      });      xhr.addEventListener("error", () => {
        if (abort.signal.aborted) return;
        setUploading(false);        setUploadProgress(0);
        showToast(t("agents.files_upload_failed_network"), "error");
      });
      xhr.send(formData);
    } catch (err) {
      setUploading(false);
      setUploadProgress(0);      showToast(String(err), "error");    }
  };

  const loadDrives = async () => {
    try {
      const data = await api.post<{ drives?: RawDrive[]; data?: RawDrive[] }>(`/agents/${id}/drives`);
      const rawDrives: RawDrive[] = data.drives || data.data || [];
      const parsed: DriveInfo[] = rawDrives.map((d: RawDrive) => {
        if (typeof d === "string") {
          return { letter: d, label: "", total: 0, free: 0 };
        }
        return {
          letter: d.letter || d.path || d.name || "??",
          label: d.label || d.volume_label || "",
          total: d.total || d.total_size || 0,
          free: d.free || d.free_space || 0,
        };
      });
      setDrives(parsed);
      setShowDrives(true);
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const handleFind = async () => {
    if (!findPattern.trim()) return;
    try {
      const data = await api.post<{ Results?: string[]; results?: string[]; Files?: string[]; files?: string[]; data?: string[] }>(`/agents/${id}/find`, { pattern: findPattern, path: currentPath });
      const results: string[] = data.results || data.files || data.data || [];
      setFindResults(results);
      showToast(t("agents.files_found_results", { n: results.length }), results.length > 0 ? "success" : "info");
    } catch (err) {
      showToast(String(err), "error");
    }
  };

  const quickPaths = useMemo(() => osType === "linux"    ? [        { label: "Home", path: "/home" },
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
      ], [osType]);

  const sep = osType === "linux" ? "/" : "\\";
  const rootPath = osType === "linux" ? "/" : "C:\\";
  const pathParts = currentPath.split(/[\\/]/).filter(Boolean);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">          <FolderTree className="w-4 h-4" />
          {t("agents.files_title")}
          </h1>
          <Badge variant="secondary" className="text-xs font-mono">{id}</Badge>
        </div>        <div className="flex items-center gap-2">
          <Button variant="default" size="sm" onClick={() => setShowFind(!showFind)} className="bg-primary hover:bg-primary/90 text-primary-foreground">
            <Search className="w-4 h-4" />
            {t("agents.files_find")}
          </Button>
          <Button variant="default" size="sm" onClick={() => setShowUpload(true)} className="bg-emerald-600 hover:bg-emerald-500">
            <CloudUpload className="w-4 h-4" />
            {t("agents.files_upload")}
          </Button>
          <Button variant="default" size="sm" onClick={loadDrives}>
            <HardDrive className="w-4 h-4" />
            {t("agents.files_drives")}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => loadDirectory(currentPath)}>
            <RotateCw className="w-4 h-4" />
            {t("agents.files_refresh")}
          </Button>
        </div>
      </div>

      <div className="bg-muted border border-border rounded-xl p-5 mb-4 shadow-sm">
        <div className="flex items-center gap-3 mb-3">
          <FolderOpen className="w-4 h-4" />
          <Input
            type="text"
            value={currentPathInput}            onChange={(e) => setCurrentPathInput(e.target.value)}
            onKeyDown={(e) => {              if (e.key === "Enter") {                navigateTo(currentPathInput);
              }            }}
                          className="flex-1 font-mono text-foreground"          />
          <Button variant="default" onClick={() => navigateTo(currentPathInput)}>
            <ArrowRight className="w-4 h-4" /> {t("agents.files_go")}
          </Button>
          <Button variant="secondary" onClick={goUp} aria-label="Go up">
            <ArrowLeft className="w-4 h-4" /> {t("agents.files_up")}
          </Button>
        </div>        <div className="flex items-center gap-1.5 text-sm">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigateTo(rootPath)}
            className={`font-mono transition-colors ${
              currentPath === rootPath
                ? "text-foreground bg-primary/10"
                : "text-primary hover:bg-primary/5"
            }`}
          >            {rootPath}
          </Button>
          {pathParts.map((part, i) => (
            <span key={part} className="flex items-center gap-1.5">
              <span className="text-muted-foreground font-mono">{sep === "/" ? "/" : "\\"}</span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleBreadcrumbClick(i + 1)}
                className={`font-mono transition-colors hover:bg-primary/5 ${                  i === pathParts.length - 1 ? "text-foreground bg-primary/10" : "text-primary"
                }`}
              >
                {part}
              </Button>
            </span>
          ))}
        </div>

        <div className="flex items-center gap-2 mt-3 text-xs">          <span className="text-muted-foreground font-medium">{t("agents.files_quick")}</span>
          {quickPaths.map((qp) => (
            <Button
              variant="outline"
              size="sm"
              key={qp.path}
              onClick={() => navigateTo(qp.path)}
              className="text-primary hover:bg-primary/5 transition-colors font-medium"
            >
              {qp.label}
            </Button>
          ))}
        </div>
      </div>

      {showFind && (
        <Card className="p-5 mb-4">
          <div className="font-semibold text-sm text-foreground mb-3 flex items-center gap-2">
            <Search className="w-4 h-4" />
            {t("agents.files_search_files")}
          </div>
          <div className="flex gap-3">
            <Input
              type="text"
              value={findPattern}              onChange={(e) => setFindPattern(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleFind()}
              placeholder={t("agents.files_find_placeholder")}
            className="flex-1 font-mono text-foreground"
            />
            <Button variant="default" onClick={handleFind} className="bg-primary hover:bg-primary/90 text-primary-foreground">
              <Search className="w-4 h-4" /> {t("common.search")}
            </Button>
            {findResults.length > 0 && (
              <Button variant="secondary" onClick={() => { setFindResults([]); setFindPattern(""); }}>
                <X className="w-4 h-4" /> {t("agents.files_clear")}
              </Button>
            )}
          </div>
          {findResults.length > 0 && (
            <div className="mt-3 max-h-48 overflow-y-auto border border-border rounded-xl bg-muted/50">
              {findResults.map((result) => (
                <Button
                  variant="ghost"
                  size="sm"
                  key={result}
                  onClick={() => {
                    const parentDir = result.substring(0, result.lastIndexOf(sep)) || currentPath;                    const fileName = result.split(sep).pop() || "";
                    navigateTo(parentDir);
                    setSelectedFile(fileName);                  }}
                  className="w-full justify-start text-left px-4 py-2 text-xs font-mono text-muted-foreground hover:bg-primary/10 border-b border-border last:border-0 transition-colors gap-2"
                >
                  <File className="w-4 h-4" />
                  {result}
                </Button>
              ))}
            </div>
          )}
        </Card>
      )}

      <Dialog open={showDrives} onOpenChange={setShowDrives}>
        <DialogContent className="max-w-md">
          <DialogHeader className="bg-gradient-to-r from-indigo-500 to-indigo-700 -mx-6 -mt-6 px-6 py-5 rounded-t-2xl">
            <DialogTitle className="text-lg font-semibold text-white flex items-center gap-2">
              <HardDrive className="w-4 h-4" />
              {t("agents.files_available_drives")}
            </DialogTitle>
          </DialogHeader>
          <div className="max-h-64 overflow-y-auto">
            {drives.length === 0 ? (
              <p className="text-muted-foreground text-sm text-center py-4">{t("agents.files_no_drives")}</p>            ) : (
              <div className="space-y-2">
                {drives.map((drive) => (
                  <Button
                    variant="ghost"
                    size="sm"
                    key={drive.letter}                    onClick={() => {                      navigateTo(drive.letter);                      setShowDrives(false);
                    }}
                    className="w-full justify-start items-center gap-3 px-4 py-3 bg-muted hover:bg-primary/10 rounded-xl transition-colors"
                  >
                    <HardDrive className="w-4 h-4" />
                    <div className="text-left">                      <div className="text-sm font-medium text-foreground">
                        {drive.letter}{drive.label ? ` (${drive.label})` : ""}
                      </div>                      {drive.total > 0 && (
                        <div className="text-xs text-muted-foreground mt-0.5">
                          {t("agents.files_free_of", { free: formatSize(drive.free), total: formatSize(drive.total) })}                          <Progress value={Math.max(0, 100 - (drive.free / drive.total) * 100)} className="h-1.5 mt-1" />
                        </div>
                      )}
                    </div>
                  </Button>                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>      {loading && (
        <div className="text-center py-8">
          <div className="inline-flex items-center gap-3">
            <Spinner size="sm" color="blue" />
            <p className="text-muted-foreground text-sm">{t("agents.files_loading")}</p>
          </div>
        </div>
      )}

      {!loading && entries.length === 0 && (        <Card className="text-center py-10">
          <FolderOpen className="w-4 h-4" />
          <p className="text-muted-foreground">{t("agents.files_empty")}</p>
        </Card>
      )}      {!loading && entries.length > 0 && (
        <Card className="overflow-hidden">
          <div className="overflow-x-auto">
            <Table className="w-full text-sm">
              <TableHeader>
                <TableRow className="border-b border-border bg-muted/50">
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-12">{t("common.type")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground">{t("agents.files_col_name")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-24">{t("agents.files_col_size")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-40">{t("agents.files_col_modified")}</TableHead>
                  <TableHead className="text-right py-3 px-4 text-xs font-semibold text-muted-foreground w-44">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((entry) => (
                  <TableRow
                    key={entry.name}
                    className={`border-b border-border last:border-0 hover:bg-primary/5 cursor-pointer transition-colors ${                      selectedFile === entry.name ? "bg-primary/10" : ""
                    }`}
                    onClick={() => handleFileClick(entry)}>
                    <TableCell className="py-2.5 px-4 text-center text-lg">
                      {getFileIcon(entry)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4">
                      <span className={`font-mono text-sm ${entry.is_dir ? "text-primary font-medium" : "text-foreground"}`}>
                        {entry.name}
                      </span>
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-sm text-muted-foreground font-mono">
                      {entry.is_dir ? "" : formatSize(entry.size)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-xs text-muted-foreground font-mono">
                      {formatTimestamp(entry.mod_time)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-right" onClick={(e) => e.stopPropagation()}>
                      {!entry.is_dir && (
                        <div className="flex items-center justify-end gap-1">
                          <Button variant="ghost" size="sm" onClick={() => handleRead(entry.name)} className="text-primary">
                            <Eye className="w-4 h-4" /> {t("agents.files_preview_btn")}
                          </Button>
                          <Button variant="ghost" size="sm" onClick={() => handleDownload(entry.name)} className="text-blue-600 dark:text-blue-400">
                            <Download className="w-4 h-4" /> {t("agents.files_dl")}
                          </Button>
                          <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(entry.name)} className="text-destructive" aria-label="Delete file">
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>        </Card>
      )}

      <Dialog open={showUpload} onOpenChange={(open) => { if (!uploading) setShowUpload(open); }}>
        <DialogContent className="max-w-md">
          <DialogHeader className="bg-gradient-to-r from-emerald-500 to-emerald-700 -mx-6 -mt-6 px-6 py-5 rounded-t-2xl">
            <DialogTitle className="text-lg font-semibold text-white flex items-center gap-2">
              <CloudUpload className="w-4 h-4" />
              {t("agents.files_upload_file")}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpload} className="space-y-4">
            <div>                <Label className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("agents.files_destination_path")}</Label>                <Input
                type="text"
                value={currentPath}
                readOnly                className="font-mono text-foreground"              />
            </div>
            <div>                <Label className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("agents.files_select_file")}</Label>
              <input
                ref={fileInputRef}
                type="file"                className="w-full bg-card border border-border text-sm rounded-xl px-3 py-2.5 text-foreground"
                aria-label="Select file to upload"
              />
            </div>
            {uploading && (
              <div>
                <div className="flex justify-between text-xs text-muted-foreground mb-1">
                  <span>{t("agents.files_uploading")}</span>
                  <span>{uploadProgress}%</span>
                </div>
                <Progress value={uploadProgress} />
              </div>            )}
            <DialogFooter className="flex gap-3 pt-2 sm:justify-stretch">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setShowUpload(false)}
                disabled={uploading}
                className="flex-1"
              >
                <X className="w-4 h-4" /> {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                variant="default"
                disabled={uploading}
                className="flex-1 bg-emerald-600 hover:bg-emerald-700"
              >
                        {uploading ? (                    <><Spinner size="sm" /> {t("agents.files_uploading")}</>
                ) : (
                  <><CloudUpload className="w-4 h-4" /> {t("agents.files_upload")}</>
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={showPreview && !!previewContent} onOpenChange={setShowPreview}>
        <DialogContent className="max-w-3xl max-h-[80vh] flex flex-col">
          <DialogHeader className={`px-6 py-5 ${              previewIsImage ? "bg-gradient-to-r from-pink-500 to-pink-700" : "bg-gradient-to-r from-indigo-500 to-indigo-700"
          } -mx-6 -mt-6 rounded-t-2xl`}>
            <DialogTitle className="text-lg font-semibold text-white flex items-center justify-between">
              <span className="flex items-center gap-2">
                {                  previewIsImage ? (
                    <><ImageIcon className="w-4 h-4" aria-hidden="true" /> {t("agents.files_image")} {selectedFile}</>
                  ) : (
                    <><FileText className="w-4 h-4" /> {t("agents.files_preview")} {selectedFile}</>
                  )
                }
              </span>
              <span className="flex items-center gap-2">                {!previewIsImage && (
                <Tooltip>
                  <TooltipTrigger render={<Button variant="ghost" size="icon-sm"
                    onClick={() => {
                        downloadText(previewContent, selectedFile || "preview.txt");
                    }}
                    aria-label={t("agents.files_download")}
                  >
                    <Download className="w-4 h-4" />
                  </Button>} />
                  <TooltipContent>{t("agents.files_download_content")}</TooltipContent>
                </Tooltip>
                )}
              </span>
            </DialogTitle>
          </DialogHeader>
          {previewIsImage ? (
            <div className="p-4 flex-1 flex items-center justify-center bg-muted/50 overflow-auto">                <img src={previewContent} alt={selectedFile || t("agents.files_preview_btn")} className="max-w-full max-h-[70vh] object-contain rounded-lg shadow-lg dark:shadow-black/30" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />              </div>
          ) : (
            <div className="p-4 sm:p-5 overflow-auto flex-1">
              <pre className="text-sm font-mono whitespace-pre-wrap text-foreground bg-muted/50 rounded-xl p-4 border border-border">
                {previewContent}
              </pre>            </div>
          )}
        </DialogContent>
      </Dialog>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.delete")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
