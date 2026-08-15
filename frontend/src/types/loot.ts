import type { Screenshot } from "./screenshot";

export interface KeylogTask {
  id: string;
  agent_id: string;
  hostname?: string;
  agent?: { hostname: string };
  result: string;
  error: string;
  status: string;
  created_at: string;
}

export interface DownloadTask {
  id: string;
  agent_id: string;
  hostname?: string;
  agent?: { hostname: string };
  type?: string;
  command: string;
  result: string;
  status: string;
  created_at: string;
}

export interface LootData {
  screenshots: Screenshot[];
  keylog_tasks: KeylogTask[];
  download_tasks: DownloadTask[];
}
