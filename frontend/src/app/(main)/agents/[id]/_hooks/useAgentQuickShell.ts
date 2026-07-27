"use client";

import { useCallback, useState } from "react";
import { api } from "@/lib/api";

interface ShellHistoryEntry {
  command: string;
  shell: string;
  result: string;
  timestamp: string;
}

export function useAgentQuickShell(agentId: string, successMessage: string, errorMessage: string) {
  const [command, setCommand] = useState("");
  const [shell, setShell] = useState("cmd.exe");
  const [history, setHistory] = useState<ShellHistoryEntry[]>([]);
  const [sending, setSending] = useState(false);
  const [expanded, setExpanded] = useState(false);

  const sendCommand = useCallback(async () => {
    if (!command.trim() || !agentId) return;
    setSending(true);
    const cmd = command.trim();
    const entry: ShellHistoryEntry = { command: cmd, shell, result: "", timestamp: new Date().toISOString() };
    try {
      const response = await api.postJson(`/agents/${agentId}/command`, { command: cmd, shell }) as { output?: string; result?: string; message?: string };
      entry.result = response?.output || response?.result || response?.message || successMessage;
    } catch {
      entry.result = errorMessage;
    }
    setHistory((prev) => [entry, ...prev].slice(0, 5));
    setCommand("");
    setSending(false);
  }, [agentId, command, shell, successMessage, errorMessage]);

  return {
    command,
    setCommand,
    shell,
    setShell,
    history,
    sending,
    expanded,
    setExpanded,
    sendCommand,
  };
}
