"use client";

import type { ReactNode } from "react";
import { usePermissions } from "@/lib/hooks/usePermissions";
import type { PermissionKey } from "@/lib/permission-keys";

/**
 * Renders children only when the current operator holds the required
 * permissions — any-of by default, or every permission with mode="all".
 * Backend enforcement stays authoritative; this is UI-only gating to hide
 * unreachable affordances. While the current operator's permissions are still
 * loading (store is null) children render — fail-open, because the backend is
 * the source of truth.
 *
 * When children is a function it receives the resolved boolean, letting a
 * single element adapt its rendering without duplicating the gate.
 */
interface PermissionProps {
  /** any = at least one (default), all = every required permission. */
  mode?: "any" | "all";
  /** Shown in place of children when not allowed. */
  fallback?: ReactNode | null;
  children: ReactNode | ((allowed: boolean) => ReactNode);
  /** Required permission(s). */
  perms: PermissionKey | readonly PermissionKey[];
}

export function Permission({
  mode = "any",
  perms,
  fallback = null,
  children,
}: PermissionProps) {
  const { permissions, canAny, canAll } = usePermissions();
  const list: readonly PermissionKey[] = Array.isArray(perms) ? perms : [perms];
  if (permissions == null) {
    return typeof children === "function" ? children(true) : children;
  }
  const allowed = mode === "all" ? canAll(list) : canAny(list);
  if (typeof children === "function") return children(allowed);
  return allowed ? children : fallback;
}