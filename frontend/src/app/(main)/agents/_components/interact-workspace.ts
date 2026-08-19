import type { InteractTab } from "@/lib/interact-storage";

export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  const tag = target.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  return Boolean(target.closest(".xterm, [role='textbox']"));
}

export function tabFromDigit(key: string): InteractTab | null {
  if (key === "1") return "shell";
  if (key === "2") return "files";
  if (key === "3") return "tasks";
  return null;
}