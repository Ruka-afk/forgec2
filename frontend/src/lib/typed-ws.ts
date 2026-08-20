"use client";

import { useMemo } from "react";
import { onWSMessage, type WSMessage } from "@/lib/wsContext";
import { useWebSocket } from "@/lib/useWebSocket";
import type { WSEventMap, WSEventName } from "@/lib/ws-events";

/** A typed WS frame: the wire message (with `type`) carrying a registered payload. */
type TypedWSMessage<N extends WSEventName> = WSMessage & WSEventMap[N];

function matchSet<N extends WSEventName>(names: N | readonly N[]): ReadonlySet<string> {
  return new Set(Array.isArray(names) ? names : [names]);
}

/** Runtime type guard: narrows a raw frame to a registered event. */
export function isWSEvent<N extends WSEventName>(msg: WSMessage, name: N): msg is TypedWSMessage<N> {
  return msg.type === name;
}

function matches<N extends WSEventName>(set: ReadonlySet<string>, msg: WSMessage): msg is TypedWSMessage<N> {
  return typeof msg.type === "string" && set.has(msg.type);
}

/**
 * Subscribe to one or more registered event types over the module-global WS
 * channel (usable outside React, e.g. the task poller in api.ts).
 * Returns an unsubscribe function.
 */
export function subscribeTyped<N extends WSEventName>(
  names: N | readonly N[],
  cb: (ev: TypedWSMessage<N>) => void,
): () => void {
  const set = matchSet(names);
  return onWSMessage((msg) => {
    if (matches<N>(set, msg)) cb(msg);
  });
}

/**
 * Subscribe to one or more registered event types within a component (uses
 * the provider-scoped channel like useWebSocket, same ref-based latest
 * callback semantics). No useCallback needed at call sites.
 */
export function useTypedWS<N extends WSEventName>(names: N | readonly N[], cb: (ev: TypedWSMessage<N>) => void): void {
  const set = useMemo(() => matchSet(names), [names]);
  useWebSocket((msg) => {
    if (matches<N>(set, msg)) cb(msg);
  });
}