import type { Screenshot } from "@/types/screenshot";
import type { KeylogTask, DownloadTask, LootData } from "@/types/loot";

export type LootTab = "screenshots" | "keylogs" | "downloads";

export function emptyLootData(): LootData {
  return { screenshots: [], keylog_tasks: [], download_tasks: [] };
}

export function normalizeLootData(result: Record<string, unknown> | null | undefined): LootData {
  if (!result) return emptyLootData();
  return {
    screenshots: (result.screenshots || []) as Screenshot[],
    keylog_tasks: (result.keylog_tasks || result.keylogs || []) as KeylogTask[],
    download_tasks: (result.download_tasks || result.downloads || []) as DownloadTask[],
  };
}
