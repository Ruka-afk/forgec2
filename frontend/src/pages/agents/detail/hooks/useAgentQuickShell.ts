import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { errorText } from "@/lib/errors";
import { useI18n } from "@/lib/i18n";
import { useErrorToast } from "@/lib/hooks/useErrorToast";
import { useMutation } from "@/lib/hooks/useMutation";
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
  const [expanded, setExpanded] = usePersistedState(
    persistKey ?? `agents.detail.${agentId}.quick_shell`,
    false,
  );
  const shellTouchedRef = useRef(false);

  const { t } = useI18n();
  const { warnError } = useErrorToast();
  // In-flight guard: the Enter key handler bypasses the disabled Button, so
  // rapid Enter presses would otherwise fire the same command twice on the
  // agent (dangerous for destructive/lateral commands). `mutate` supersedes
  // callbacks but does not drop concurrent calls — this ref does the dropping.
  const sendingRef = useRef(false);

  const pushEntry = useCallback((entry: ShellHistoryEntry) => {
    setHistory((prev) => [entry, ...prev].slice(0, 10));
  }, []);

  const {
    mutate: send,
    isPending: sending,
    reset: resetSend,
  } = useMutation({
    fn: async (entry: ShellHistoryEntry) => {
      const response = await api.postJson(paths.agents.command(agentId), { command: entry.command, shell: entry.shell }) as { output?: string; result?: string; message?: string };
      return response?.output || response?.result || response?.message || successMessage;
    },
    onSuccess: (result, entry) => {
      pushEntry({ ...entry, result });
      setCommand("");
      sendingRef.current = false;
    },
    onError: (err, entry) => {
      const conflict = err instanceof ApiError && err.status === 409;
      if (conflict) {
        // 409 is a deliberate command-layer outcome (approval conflict): the
        // warning tone stays, but the toast and the history row both resolve
        // their text through the shared error rules, with the same fallback
        // key the caller's pre-translated `errorMessage` stands for.
        const fallbackKey = "agents.detail_command_send_failed";
        warnError(err, fallbackKey);
        pushEntry({ ...entry, result: errorText(t, err, () => t(fallbackKey)) });
      } else {
        pushEntry({ ...entry, result: errorMessage });
      }
      setCommand("");
      sendingRef.current = false;
    },
  });

  // Abandon an in-flight command when the agent changes: its result must not
  // land in the new agent's history, and its guard must not keep blocking the
  // new agent's sends.
  useEffect(() => {
    sendingRef.current = false;
    resetSend();
  }, [agentId, resetSend]);

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

  const sendCommand = useCallback(() => {
    const cmd = command.trim();
    if (!cmd || !agentId) return;
    if (sendingRef.current) return;
    sendingRef.current = true;
    return send({ command: cmd, shell, result: "", timestamp: new Date().toISOString() });
  }, [agentId, command, shell, send]);

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
