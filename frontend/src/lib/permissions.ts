/**
 * Permission helpers for role-based UI gating.
 *
 * The permission strings are the wire values of /api/me → data.permissions,
 * which mirror the backend permission constants (agents.read, users.write,
 * settings.read, …). Admin users receive the full permission set, so no
 * role special-casing is needed here.
 */

export type Permissions = readonly string[] | null | undefined;

/** True when the user holds the exact permission. */
export function can(permissions: Permissions, perm: string): boolean {
  if (!permissions) return false;
  return permissions.includes(perm);
}

/** True when the user holds at least one of the given permissions. */
export function canAny(permissions: Permissions, perms: readonly string[]): boolean {
  if (!permissions || perms.length === 0) return false;
  return perms.some((p) => permissions.includes(p));
}

/** True when the user holds every one of the given permissions. */
export function canAll(permissions: Permissions, perms: readonly string[]): boolean {
  if (!permissions) return false;
  return perms.every((p) => permissions.includes(p));
}