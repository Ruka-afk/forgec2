import { api } from "./api";
import { paths } from "./api-paths";
import { firstArray } from "./envelope";
import { fetchCached } from "./hooks/useCachedData";

export interface TaskTypeInfo {
  type: string;
  name: string;
  description?: string;
  category?: string;
  requires_shell?: boolean;
  requires_elevation?: boolean;
  parameters?: Array<{ name: string; type: string; required: boolean; description?: string }>;
}

/**
 * Shared task-type list cache (single key, 5-minute TTL).
 *
 * Deliberately has NO hard-coded fallback: a failed read must not surface as
 * "the server knows these types". The promise rejects on failure (and the
 * failed attempt is not cached, per fetchCached), so callers can show a
 * "could not be read" state instead of a reassuring default list.
 */
const TASK_TYPES_CACHE_KEY = "task-types:list";
const TASK_TYPES_TTL_MS = 5 * 60_000;

export async function fetchTaskTypes(): Promise<TaskTypeInfo[]> {
  return fetchCached<TaskTypeInfo[]>(
    TASK_TYPES_CACHE_KEY,
    async () => {
      const data = await api.get<unknown>(paths.v1.taskTypes);
      return firstArray(data, ["types", "task_types", "data"]) as TaskTypeInfo[];
    },
    TASK_TYPES_TTL_MS,
  );
}
