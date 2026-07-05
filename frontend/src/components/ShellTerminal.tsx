"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { apiPostJson } from "@/lib/api";
import { fetchAgentBeaconTiming, loadCommandHistory, saveCommandHistory } from "@/lib/shell";

const PROMPT_CHARS = ["$", "#", ">", "%"];
const KEY = "forgec2_shell_fontsize";

function clearCommandHistory() {
  try { localStorage.removeItem("forgec2_shell_history"); } catch { /* ignore */ }
}

export default function ShellTerminal({
  agentId,
  shellType = "cmd",
  showHeader = true,
  osType = "windows",
}: {
  agentId: string;
  shellType?: string;
  showHeader?: boolean;
  osType?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fitRef = useRef<FitAddon | null>(null);
  const historyRef = useRef<string[]>([]);
  const histIdxRef = useRef(-1);
  const loadingRef = useRef(false);
  const [loading, setLoading] = useState(false);
  const [fontSize, setFontSize] = useState(() => {
    if (typeof window === "undefined") return 14;
    return Number(localStorage.getItem(KEY)) || 14;
  });
  const [beaconHint, setBeaconHint] = useState("");

  const promptRef = useRef(`\x1b[94m${PROMPT_CHARS[Math.floor(Math.random() * PROMPT_CHARS.length)]}\x1b[0m `);
  const execRef = useRef<(cmd: string) => Promise<void>>(async () => {});
  const writePromptRef = useRef<() => void>(() => {});

  const writeln = useCallback(
    (text: string, color = "37") => termRef.current?.writeln(`\x1b[${color}m${text}\x1b[0m`),
    [],
  );

  const writePrompt = useCallback(() => termRef.current?.write(promptRef.current), []);

  const executeCommand = useCallback(
    async (cmd: string) => {
      if (!cmd.trim() || loadingRef.current) return;
      loadingRef.current = true;
      setLoading(true);
      try {
        const result = await apiPostJson<{ stdout?: string; stderr?: string; error?: string }>(
          `/agents/${agentId}/shell`,
          { command: cmd, shell: shellType },
        );
        if (result.stdout) {
          const out = result.stdout;
          termRef.current?.write(out.replace(/\n/g, "\r\n"));
          termRef.current?.write("\r\n");
        } else if (result.stderr) {
          writeln(result.stderr, "31");
        } else {
          writeln(result.error || "Command failed", "31");
        }
    } catch (e) {
      writeln(String(e), "31");
    } finally {
      loadingRef.current = false;
      setLoading(false);
      writePrompt();
      termRef.current?.focus();
    }
  }, [agentId, shellType, writeln, writePrompt]);

  useEffect(() => { execRef.current = executeCommand; }, [executeCommand]);
  useEffect(() => { writePromptRef.current = writePrompt; }, [writePrompt]);

  useEffect(() => {
    if (!containerRef.current) return;

    const term = new Terminal({
      cursorBlink: true,
      fontSize,
      fontFamily: "'JetBrains Mono', 'Cascadia Code', Consolas, monospace",
      theme: {
        background: "#020617",
        foreground: "#e2e8f0",
        cursor: "#34d399",
        selectionBackground: "#334155",
      },
      scrollback: 5000,
      convertEol: true,
    });

    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(containerRef.current);
    fit.fit();

    termRef.current = term;
    fitRef.current = fit;

    historyRef.current = loadCommandHistory();
    term.writeln("\x1b[90mForgeC2 Shell \u00b7 xterm.js \u00b7 Enter to run \u00b7 \u2191\u2193 history \u00b7 Ctrl+L clear\x1b[0m");
    writePrompt();

    term.onData((data) => {
      if (loadingRef.current) return;

      const t = term;
      if (data === "\r") {
        const cmd = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString().trim() || "";
        if (cmd.startsWith(promptRef.current.trim())) {
          const trimmed = cmd.slice(promptRef.current.trim().length).trim();
          if (trimmed) {
            t.writeln("");
            saveCommandHistory(trimmed);
            historyRef.current = loadCommandHistory();
            histIdxRef.current = historyRef.current.length;
            execRef.current(trimmed);
          } else {
            t.writeln("");
            writePromptRef.current();
          }
        } else {
          t.writeln("");
          writePromptRef.current();
        }
        return;
      }

      if (data === "\u007f") {
        const line = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString() || "";
        const promptLen = promptRef.current.trim().length;
        if (line.length > promptLen) {
          t.write("\b \b");
        }
        return;
      }

      if (data === "\x1b[A") {
        if (historyRef.current.length > 0 && histIdxRef.current > 0) {
          histIdxRef.current--;
          const cmd = historyRef.current[histIdxRef.current];
          const line = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString() || "";
          const promptLen = promptRef.current.trim().length;
          for (let i = line.length; i > promptLen; i--) t.write("\b \b");
          t.write(cmd);
        }
        return;
      }

      if (data === "\x1b[B") {
        if (histIdxRef.current < historyRef.current.length - 1) {
          histIdxRef.current++;
          const cmd = historyRef.current[histIdxRef.current];
          const line = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString() || "";
          const promptLen = promptRef.current.trim().length;
          for (let i = line.length; i > promptLen; i--) t.write("\b \b");
          t.write(cmd);
        } else {
          histIdxRef.current = historyRef.current.length;
          const line = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString() || "";
          const promptLen = promptRef.current.trim().length;
          for (let i = line.length; i > promptLen; i--) t.write("\b \b");
        }
        return;
      }

      if (data === "\x0c") {
        t.clear();
        writePromptRef.current();
        return;
      }

      if (data === "\x09") {
        return;
      }

      if (data.length === 1) {
        t.write(data);
      }
    });

    const ro = new ResizeObserver(() => fit.fit());
    ro.observe(containerRef.current);

    return () => {
      ro.disconnect();
      term.dispose();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [agentId, executeCommand, writeln]);

  useEffect(() => {
    if (termRef.current) termRef.current.options.fontSize = fontSize;
  }, [fontSize]);

  useEffect(() => {
    fetchAgentBeaconTiming(agentId).then(({ interval, jitter }) => {
      setBeaconHint(interval === 0 ? `Real-time \u00b1${jitter}%` : `${interval}s \u00b1${jitter}%`);
    });
  }, [agentId]);

  const quickCommands =
    osType === "linux"
      ? ["whoami", "id", "uname -a", "hostname", "ps aux", "ip addr"]
      : ["whoami", "hostname", "ipconfig", "systeminfo", "netstat -ano", "tasklist"];

  const runQuick = (cmd: string) => {
    if (loadingRef.current) return;
    termRef.current?.write(`\r\n\x1b[33m${promptRef.current}\x1b[37m${cmd}\x1b[0m\r\n`);
    executeCommand(cmd);
  };

  return (
    <div className="h-[calc(100vh-9rem)] flex flex-col bg-slate-950 text-slate-100 rounded-2xl overflow-hidden border border-slate-700">
      {showHeader && (
        <div className="shrink-0 bg-slate-900 border-b border-slate-700 px-4 py-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="w-2 h-2 bg-emerald-500 rounded-full shrink-0"></span>
            <span className="font-semibold text-sm text-slate-100 truncate">Shell</span>
            <span className="text-xs text-slate-500 font-mono uppercase">{shellType}</span>
            <span className="text-xs text-slate-600" title="Beacon timing">
              <i className="fa-solid fa-clock mr-1"></i>
              {beaconHint || "loading..."}
            </span>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <div className="flex items-center gap-1 text-xs text-slate-400 mr-2">
              <i className="fa-solid fa-text-height"></i>
              <select
                value={fontSize}
                onChange={(e) => {
                  const v = Number(e.target.value);
                  setFontSize(v);
                  localStorage.setItem(KEY, String(v));
                }}
                className="bg-slate-800 border border-slate-600 rounded px-1 py-0.5 text-xs text-slate-200 cursor-pointer"
              >
                <option value={12}>12</option>
                <option value={13}>13</option>
                <option value={14}>14</option>
                <option value={16}>16</option>
                </select>
              </div>
            {quickCommands.map((cmd) => (
              <button
                key={cmd}
                onClick={() => runQuick(cmd)}
                disabled={loading}
                className="px-2 py-1 text-xs bg-slate-800 hover:bg-slate-700 border border-slate-600 rounded text-slate-300 disabled:opacity-50 whitespace-nowrap"
              >
                {cmd}
              </button>
            ))}
            <button
              onClick={() => { clearCommandHistory(); historyRef.current = []; histIdxRef.current = 0; writeln("History cleared", "33"); }}
              className="px-2 py-1 text-xs bg-slate-800 hover:bg-slate-700 border border-slate-600 rounded text-slate-400"
              title="Clear history"
            >
              <i className="fa-solid fa-trash"></i>
            </button>
          </div>
        </div>
      )}
      <div ref={containerRef} className="flex-1 min-h-0" style={{ background: "#020617" }} />
      {loading && (
        <div className="shrink-0 bg-slate-900 border-t border-slate-700 px-4 py-1.5 text-xs text-emerald-400 flex items-center gap-2">
          <i className="fa-solid fa-circle-notch fa-spin"></i> Executing...
        </div>
      )}
      <div className="shrink-0 bg-slate-950 border-t border-slate-700 px-4 py-1.5 text-[10px] text-slate-600 flex items-center justify-between">
        <span>
          <i className="fa-solid fa-keyboard mr-1"></i>
          Tab: trigger complete · {osType === "linux" ? "Ctrl+D" : "Ctrl+Z"} · Ctrl+C: interrupt
        </span>
        <span>
          Font: {fontSize}px
        </span>
      </div>
    </div>
  );
}
