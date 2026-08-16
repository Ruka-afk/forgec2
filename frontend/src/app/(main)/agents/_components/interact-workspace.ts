export const INTERACT_TABS = ["shell", "files", "tasks"] as const;
export type InteractTab = (typeof INTERACT_TABS)[number];

export const DOCK_HEIGHT_MIN = 180;
export const DOCK_HEIGHT_MAX = 640;
export const DOCK_HEIGHT_DEFAULT = 360;
export const INTERACT_STORAGE_KEY = "forgec2_interact_dock";

export function clampDockHeight(height: number, max = DOCK_HEIGHT_MAX): number {
  if (!Number.isFinite(height)) return DOCK_HEIGHT_DEFAULT;
  return Math.min(max, Math.max(DOCK_HEIGHT_MIN, Math.round(height)));
}

export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  return Boolean(target.closest(".xterm, [role='textbox']"));
}

export interface InteractPrefs {
  agentId: string | null;
  height: number;
  tab: InteractTab;
  pinned: boolean;
}

export function defaultInteractPrefs(): InteractPrefs {
  return { agentId: null, height: DOCK_HEIGHT_DEFAULT, tab: "shell", pinned: false };
}

export function readInteractPrefs(storage?: Pick<Storage, "getItem"> | null): InteractPrefs {
  const fallback = defaultInteractPrefs();
  try {
    const raw = (storage ?? (typeof sessionStorage !== "undefined" ? sessionStorage : null))?.getItem(INTERACT_STORAGE_KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as Partial<InteractPrefs>;
    const tab = INTERACT_TABS.includes(parsed.tab as InteractTab) ? (parsed.tab as InteractTab) : "shell";
    return {
      agentId: typeof parsed.agentId === "string" && parsed.agentId ? parsed.agentId : null,
      height: clampDockHeight(typeof parsed.height === "number" ? parsed.height : DOCK_HEIGHT_DEFAULT),
      tab,
      pinned: parsed.pinned === true,
    };
  } catch {
    return fallback;
  }
}

export function writeInteractPrefs(
  prefs: InteractPrefs,
  storage?: Pick<Storage, "setItem"> | null,
): void {
  try {
    (storage ?? (typeof sessionStorage !== "undefined" ? sessionStorage : null))?.setItem(
      INTERACT_STORAGE_KEY,
      JSON.stringify({
        agentId: prefs.pinned ? prefs.agentId : null,
        height: clampDockHeight(prefs.height),
        tab: prefs.tab,
        pinned: prefs.pinned,
      }),
    );
  } catch {
    /* private mode */
  }
}

export function tabFromDigit(key: string): InteractTab | null {
  if (key === "1") return "shell";
  if (key === "2") return "files";
  if (key === "3") return "tasks";
  return null;
}
