"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { usePersistedState } from "@/lib/hooks/usePersistedState";

interface ShellHistoryEntry {
  command: string;
  shell: string;
  result: string;
  timestamp: string;
}

function defaultShellForOS(os: string | undefined): string {
  return os && !/^win/i.test(os) ? "/bin/sh" : "cmd.exe";
}

export function useAgentQuickShell(agentId: string, os: string | undefined, successMessage: string, errorMessage: string, persistKey?: string) {
  const [command, setCommand] = useState("");
  const [shell, setShell] = useState(() => defaultShellForOS(os));
  const [history, setHistory] = useState<ShellHistoryEntry[]>([]);
  const [sending, setSending] = useState(false);
  const [expanded, setExpanded] = usePersistedState(
    persistKey ?? `agents.detail.${agentId}.quick_shell`,
    false,
  );
  const mountedRef = useRef(true);
  const shellTouchedRef = useRef(false);
  // In-flight guard: the Enter key handler bypasses the disabled Button, so
  // rapid Enter presses would otherwise fire the same command twice on the
  // agent (dangerous for destructive/lateral commands).
  const sendingRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  // Reset operator state when navigating to a different agent so drafts
  // and history never carry over (or get sent to the wrong agent).
  useEffect(() => {
    setCommand("");
    setHistory([]);
    shellTouchedRef.current = false;
    setShell(defaultShellForOS(os));
  }, [agentId, os]);

  // Keep the interpreter defaulted to the agent's OS until the operator
  // explicitly picks one (detail loads async, so os may arrive late).
  useEffect(() => {
    if (shellTouchedRef.current) return;
    setShell(defaultShellForOS(os));
  }, [os]);

  const changeShell = useCallback((v: string) => {
    shellTouchedRef.current = true;
    setShell(v);
  }, []);

  const sendCommand = useCallback(async () => {
    if (!command.trim() || !agentId) return;
    if (sendingRef.current) return;
    sendingRef.current = true;
    setSending(true);
    const cmd = command.trim();
    const entry: ShellHistoryEntry = { command: cmd, shell, result: "", timestamp: new Date().toISOString() };
    try {
      const response = await api.postJson(paths.agents.command(agentId), { command: cmd, shell }) as { output?: string; result?: string; message?: string };
      entry.result = response?.output || response?.result || response?.message || successMessage;
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        entry.result = err.message;
        toast.warning(err.message);
      } else {
        entry.result = errorMessage;
      }
    }
    if (!mountedRef.current) return;
    setHistory((prev) => [entry, ...prev].slice(0, 5));
    setCommand("");
    setSending(false);
    sendingRef.current = false;
  }, [agentId, command, shell, successMessage, errorMessage]);

  return {
    command,
    setCommand,
    shell,
    setShell: changeShell,
    history,
    sending,
    expanded,
    setExpanded,
    sendCommand,
  };
}