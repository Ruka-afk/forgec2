export interface FileEntry {
  name: string;
  is_dir: boolean;
  size: number;
  mod_time: string;
}

export interface DriveInfo {
  letter: string;
  label: string;
  total: number;
  free: number;
}

export type RawDrive =
  | string
  | {
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

export function formatSize(bytes: number): string {
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

export function getFileIcon(entry: FileEntry): string {
  if (entry.is_dir) return "📁";
  const ext = entry.name.split(".").pop()?.toLowerCase() || "";
  if (["txt", "log", "json", "xml", "ini", "conf", "yaml", "yml", "md", "cfg", "env"].includes(ext)) return "📄";
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

export function isImageFile(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() || "";
  return ["jpg", "jpeg", "png", "gif", "bmp", "ico", "svg", "webp", "tiff"].includes(ext);
}

export function formatTimestamp(ts: string): string {
  if (!ts) return "-";
  try {
    const d = new Date(ts);
    return d.toLocaleDateString() + " " + d.toLocaleTimeString();
  } catch {
    return ts;
  }
}

export function joinPath(currentPath: string, name: string, osType: "windows" | "linux"): string {
  const sep = osType === "linux" ? "/" : "\\";
  return currentPath.replace(/[\\/]+$/, "") + sep + name;
}

export function parentPath(currentPath: string, osType: "windows" | "linux"): string {
  const cleaned = currentPath.replace(/[\\/]+$/, "");
  const parts = cleaned.split(/[\\/]/).filter(Boolean);
  if (parts.length === 0) return osType === "linux" ? "/" : "C:\\";
  parts.pop();
  if (osType === "linux") return "/" + parts.join("/");
  const parent = parts.join("\\");
  return parent ? parent + "\\" : "C:\\";
}

export function parseDrives(raw: RawDrive[]): DriveInfo[] {
  return raw.map((d) => {
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
}
