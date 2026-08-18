"use client";

import { useAppStore } from "@/lib/store";
import { can, canAny, canAll } from "@/lib/permissions";

/**
 * Current operator's effective permissions (from /api/me → data.permissions,
 * cached in the app store by the Sidebar fetch). The store keeps the user
 * level of detail; this hook exposes the permission predicates.
 */
export function usePermissions() {
  const permissions = useAppStore((s) => s.currentPermissions);
  const role = useAppStore((s) => s.currentUserRole);
  return {
    permissions,
    role,
    can: (perm: string) => can(permissions, perm),
    canAny: (perms: readonly string[]) => canAny(permissions, perms),
    canAll: (perms: readonly string[]) => canAll(permissions, perms),
  };
}