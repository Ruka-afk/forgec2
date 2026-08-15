/** Payload generate is only valid when a listener is selected. */
export function canGenerateFromListener(listenerId?: string | null): boolean {
  return Boolean(String(listenerId ?? "").trim());
}
