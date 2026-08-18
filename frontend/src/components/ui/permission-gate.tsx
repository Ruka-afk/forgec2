"use client";

import type { ReactNode } from "react";
import { usePermissions } from "@/lib/hooks/usePermissions";

/**
 * Renders children only when the current operator holds at least one of the
 * required permissions (any-of semantics — use canAll via usePermissions when
 * every permission must be held). Backend enforcement stays authoritative;
 * this is UI-only gating to hide unreachable affordances.
 */
export function PermissionGate({ perms, fallback = null, children }: {
  perms: readonly string[];
  fallback?: ReactNode;
  children: ReactNode;
}) {
  const { canAny } = usePermissions();
  return canAny(perms) ? children : fallback;
}