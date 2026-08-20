import { formatTime, formatSize } from "@/lib/utils";

export { formatSize };

export interface FileEntry {
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

type RawDrive =
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

export function isImageFile(name: string): boolean {
  const ext = name.split(".").pop()?.toLowerCase() || "";
  return ["jpg", "jpeg", "png", "gif", "bmp", "ico", "svg", "webp", "tiff"].includes(ext);
}

export function formatTimestamp(ts: string): string {
  if (!ts) return "-";
  return formatTime(ts);
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
