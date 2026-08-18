"use client";

import type { ReactNode } from "react";
import { usePermissions } from "@/lib/hooks/usePermissions";

/**
 * Renders children only when the current operator holds at least one of the
 * required permissions (any-of semantics — use canAll via usePermissions when
 * every permission must be held). Backend enforcement stays authoritative;
 * this is UI-only gating to hide unreachable affordances. While the current
 * operator's permissions are still loading (store is null) children render —
 * fail-open, because the backend is the source of truth.
 */
export function PermissionGate({ perms, fallback = null, children }: {
  perms: readonly string[];
  fallback?: ReactNode;
  children: ReactNode;
}) {
  const { permissions, canAny } = usePermissions();
  if (permissions == null) return children;
  return canAny(perms) ? children : fallback;
}