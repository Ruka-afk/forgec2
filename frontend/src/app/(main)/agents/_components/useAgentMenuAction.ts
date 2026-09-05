"use client";

import { useCallback } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useInteractStore } from "@/lib/interact-store";
import type { Beacon } from "./types";
import type { AgentConfirmType } from "./useAgentModals";
import type { AgentMenuAction, AgentMenuPoint } from "./agent-menu-actions";
import { rebuildPayloadHref } from "../../generate/_components/generate-query";

type TKey = (key: string, params?: Record<string, string | number>) => string;

export interface MenuActionDeps {
  t: TKey;
  router: { push: (href: string) => void };
  setSelectedAgentId: (id: string | null) => void;
  setMenuPoint: (p: AgentMenuPoint | null) => void;
  openQuickSleep: (beacon: Beacon) => void;
  openNotesEdit: (beacon: Beacon) => void;
  setConfirm: (c: { type: AgentConfirmType; id?: string; hostname?: string } | null) => void;
}

function toastQueued(t: TKey) {
  toast.success(t("agents.detail_action_queued", { label: t("agents.screenshot") }));
}

/** Row/grid context-menu + quick-nav dispatch for the agents list. */
export function useAgentMenuAction(deps: MenuActionDeps) {
  const { t, router, setSelectedAgentId, setMenuPoint, openQuickSleep, openNotesEdit, setConfirm } = deps;

  const handleMenuAction = useCallback((action: AgentMenuAction, point: AgentMenuPoint) => {
    const id = point.beacon.id || "";
    switch (action) {
      case "interact":
      case "socks":
        useInteractStore.getState().open(id, { beacon: point.beacon });
        break;
      case "details":
        setSelectedAgentId(id);
        break;
      case "screenshot":
        api.post(paths.agents.screenshotTask(id))
          .then((d) => {
            const taskId = Number((d as { task_id?: number }).task_id);
            const queued = Number.isFinite(taskId) && taskId > 0;
            if (queued) useInteractStore.getState().revealTask(id, taskId);
            toastQueued(t);
          })
          .catch(() => toast.error(t("agents.screenshot_failed")));
        break;
      case "beacon_now":
        api.post(paths.agents.cmd(id, "beacon_now"))
          .then((d) => {
            const taskId = Number((d as { task_id?: number }).task_id);
            const queued = Number.isFinite(taskId) && taskId > 0;
            if (queued) useInteractStore.getState().revealTask(id, taskId);
            toast.success(t("agents.beacon_now_queued"));
          })
          .catch(() => toast.error(t("agents.beacon_now_failed")));
        break;
      case "rebuild": {
        if (!point.beacon.listener_id) toast.warning(t("agents.rebuild_no_listener"));
        router.push(rebuildPayloadHref(point.beacon));
        break;
      }
      case "sleep":
        openQuickSleep(point.beacon);
        break;
      case "notes":
        openNotesEdit(point.beacon);
        break;
      case "files":
        router.push(`/agents/${id}/files`);
        break;
      case "tokens":
        router.push(`/agents/${id}/token`);
        break;
      case "screen":
        router.push(`/agents/${id}/screen`);
        break;
      case "copy_id":
        navigator.clipboard.writeText(id)
          .then(() => toast.success(t("agents.detail_copied")))
          .catch(() => toast.error(t("agents.detail_copy_failed")));
        break;
      case "kill":
        setConfirm({ type: "kill", id, hostname: point.beacon.hostname || id });
        break;
      case "uninstall":
        setConfirm({ type: "uninstall", id, hostname: point.beacon.hostname || id });
        break;
      case "delete":
        setConfirm({ type: "delete", id, hostname: point.beacon.hostname || id });
        break;
    }
    setMenuPoint(null);
  }, [t, openQuickSleep, openNotesEdit, router, setConfirm, setMenuPoint, setSelectedAgentId]);

  const handleQuickNav = useCallback((beacon: Beacon, view: "shell" | "files" | "screen") => {
    const id = beacon.id || "";
    if (!id) return;
    router.push(`/agents/${id}/${view}`);
  }, [router]);

  return { handleMenuAction, handleQuickNav };
}
