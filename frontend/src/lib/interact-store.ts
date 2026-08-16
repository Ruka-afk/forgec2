"use client";

import { create } from "zustand";
import type { Beacon } from "@/app/(main)/agents/_components/types";
import {
  defaultInteractPrefs,
  readInteractPrefs,
  writeInteractPrefs,
  type InteractTab,
} from "@/app/(main)/agents/_components/interact-workspace";

export function isAgentSessionPath(path: string, agentId: string): boolean {
  if (!agentId || !path) return false;
  const prefix = `/agents/${agentId}`;
  return path === prefix || path.startsWith(`${prefix}/`);
}

/** Unpinned dock closes on navigation, except dest pages of the same session. */
export function shouldCloseOnNavigate(
  pinned: boolean,
  fromPath: string,
  toPath: string,
  dockAgentId?: string | null,
): boolean {
  if (pinned || fromPath === toPath) return false;
  if (dockAgentId && isAgentSessionPath(toPath, dockAgentId)) return false;
  return true;
}

export function nextRevealKind(dockAgentId: string | null, targetAgentId: string): "reveal" | "offer" {
  return dockAgentId && dockAgentId === targetAgentId ? "reveal" : "offer";
}

const boot = typeof window !== "undefined" ? readInteractPrefs() : defaultInteractPrefs();

interface InteractState {
  agentId: string | null;
  beacon: Beacon | null;
  height: number;
  tab: InteractTab;
  pinned: boolean;
  expandedTaskId: number | null;
  open: (id: string, opts?: { tab?: InteractTab; beacon?: Beacon; expandedTaskId?: number }) => void;
  close: () => void;
  setTab: (tab: InteractTab) => void;
  setHeight: (height: number) => void;
  togglePin: () => void;
  setBeacon: (beacon: Beacon | null) => void;
  revealTask: (agentId: string, taskId: number) => "reveal" | "offer";
}

function persist(): void {
  const s = useInteractStore.getState();
  writeInteractPrefs({
    agentId: s.agentId,
    height: s.height,
    tab: s.tab,
    pinned: s.pinned,
  });
}

export const useInteractStore = create<InteractState>((set, get) => ({
  agentId: boot.pinned ? boot.agentId : null,
  beacon: boot.pinned && boot.agentId ? { id: boot.agentId } : null,
  height: boot.height,
  tab: boot.tab,
  pinned: boot.pinned,
  expandedTaskId: null,

  open: (id, opts) => {
    const prev = get();
    const sameAgent = prev.agentId === id;
    set({
      agentId: id,
      tab: opts?.tab ?? prev.tab,
      // Never leak the previous agent's identity into the new session.
      beacon: opts?.beacon ?? (sameAgent ? prev.beacon : { id }),
      expandedTaskId: opts?.expandedTaskId ?? (sameAgent ? prev.expandedTaskId : null),
    });
    persist();
  },

  close: () => {
    set({ agentId: null, beacon: null, pinned: false, expandedTaskId: null });
    persist();
  },

  revealTask: (agentId, taskId) => {
    const kind = nextRevealKind(get().agentId, agentId);
    if (kind === "reveal") {
      set({ tab: "tasks", expandedTaskId: taskId });
      persist();
    }
    return kind;
  },

  setTab: (tab) => {
    set({ tab });
    persist();
  },

  setHeight: (height) => {
    set({ height });
    persist();
  },

  togglePin: () => {
    set({ pinned: !get().pinned });
    persist();
  },

  setBeacon: (beacon) => set({ beacon }),
}));
