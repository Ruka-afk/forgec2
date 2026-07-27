export interface TaskTypeParam {
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

export async function fetchTaskTypes(): Promise<TaskTypeInfo[]> {
  if (cachedTypes) return cachedTypes;
  try {
    const res = await fetch("/api/v1/task-types");
    const json = await res.json();
    cachedTypes = (json.data || []) as TaskTypeInfo[];
  } catch {
    cachedTypes = FALLBACK_TYPES;
  }
  return cachedTypes;
}

export function getCachedTaskTypes(): TaskTypeInfo[] {
  return cachedTypes ?? FALLBACK_TYPES;
}

export function getTaskTypeLabel(type: string): string {
  const found = (cachedTypes ?? FALLBACK_TYPES).find(t => t.type === type);
  return found ? found.name : type;
}
