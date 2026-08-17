interface TaskTypeParam {
  name: string;
  type: string;
  required: boolean;
  description?: string;
}

export interface TaskTypeInfo {
  type: string;
  name: string;
  description?: string;
  category?: string;
  requires_shell?: boolean;
  requires_elevation?: boolean;
  parameters?: TaskTypeParam[];
}

const FALLBACK_TYPES: TaskTypeInfo[] = [
  { type: "shell", name: "Shell" },
  { type: "powershell", name: "PowerShell" },
  { type: "command", name: "Command" },
  { type: "script", name: "Script" },
  { type: "bof", name: "BOF" },
  { type: "custom", name: "Custom" },
];

let cachedTypes: TaskTypeInfo[] | null = null;

import { buildUrl } from "./api";
import { paths } from "./api-paths";

export async function fetchTaskTypes(): Promise<TaskTypeInfo[]> {
  if (cachedTypes) return cachedTypes;
  try {
    const res = await fetch(buildUrl(paths.v1.taskTypes));
    const json = await res.json();
    cachedTypes = (json.data || []) as TaskTypeInfo[];
  } catch {
    cachedTypes = FALLBACK_TYPES;
  }
  return cachedTypes;
}
