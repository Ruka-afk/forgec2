"use client";

import { useCallback, useEffect, useRef } from "react";
import { usePathname, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { AgentInteractDock } from "@/app/(main)/agents/_components/AgentInteractDock";
import { isEditableTarget, tabFromDigit } from "@/app/(main)/agents/_components/interact-workspace";
import { shouldCloseOnNavigate, useInteractStore } from "@/lib/interact-store";

export default function GlobalInteractDock() {
  const router = useRouter();
  const pathname = usePathname();
  const prevPath = useRef(pathname);
  const prevFocus = useRef<HTMLElement | null>(null);
  const agentId = useInteractStore((s) => s.agentId);
  const beacon = useInteractStore((s) => s.beacon);
  const height = useInteractStore((s) => s.height);
  const tab = useInteractStore((s) => s.tab);
  const pinned = useInteractStore((s) => s.pinned);
  const close = useInteractStore((s) => s.close);
  const setTab = useInteractStore((s) => s.setTab);
  const setHeight = useInteractStore((s) => s.setHeight);
  const togglePin = useInteractStore((s) => s.togglePin);
  const setBeacon = useInteractStore((s) => s.setBeacon);

  const openAgentDetails = useCallback(() => {
    if (agentId) router.push(`/agents/${agentId}`);
  }, [router, agentId]);

  const handleClose = useCallback(() => {
    prevFocus.current?.focus();
    close();
  }, [close]);

  useEffect(() => {
    if (shouldCloseOnNavigate(pinned, prevPath.current, pathname, agentId)) close();
    prevPath.current = pathname;
  }, [pathname, pinned, agentId, close]);

  useEffect(() => {
    if (agentId) prevFocus.current = document.activeElement as HTMLElement | null;
  }, [agentId]);

  useEffect(() => {
    if (!agentId || beacon?.hostname) return;
    const ac = new AbortController();
    api
      .get(paths.agents.one(agentId), { signal: ac.signal })
      .then((data) => {
        const raw = (data.agent || data) as Record<string, unknown>;
        setBeacon({
          id: agentId,
          hostname: String(raw.hostname ?? ""),
          username: String(raw.username ?? ""),
          ip: String(raw.ip ?? ""),
          os: String(raw.os ?? ""),
          status: (raw.status as "online" | "offline" | "stale") || "offline",
        });
      })
      .catch(() => { /* keep id-only snapshot */ });
    return () => ac.abort();
  }, [agentId, beacon?.hostname, setBeacon]);

  useEffect(() => {
    if (!agentId) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey) return;
      if (isEditableTarget(e.target)) return;
       if (e.key === "Escape") {
         if (document.querySelector('[role="menu"]')) return;
         handleClose();
         return;
       }
      const next = tabFromDigit(e.key);
      if (next) {
        e.preventDefault();
        setTab(next);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [agentId, handleClose, setTab]);

  if (!agentId) return null;

  return (
    <AgentInteractDock
      beacon={beacon || { id: agentId }}
      height={height}
      pinned={pinned}
      tab={tab}
      onTabChange={setTab}
      onHeightChange={setHeight}
      onTogglePin={togglePin}
      onClose={handleClose}
      onOpenDetails={openAgentDetails}
    />
  );
}
