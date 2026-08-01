"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { runTask } from "@/lib/api";
import { fetchAgentBeaconTiming, loadCommandHistory, saveCommandHistory } from "@/lib/shell";
import { getCompletions } from "@/lib/completions";
import { highlightOutput } from "@/lib/highlight";
import { Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Clock, Type, Trash2, Keyboard } from "lucide-react";
import { useI18n } from "@/lib/i18n";

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
  const { t } = useI18n();
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

  const [promptChar] = useState(() => PROMPT_CHARS[Math.floor(Math.random() * PROMPT_CHARS.length)]);
  const promptRef = useRef(`\x1b[94m${promptChar}\x1b[0m `);
  const plainPromptRef = useRef(`${promptChar} `);
  const execRef = useRef<(cmd: string) => Promise<void>>(async () => {});
  const writePromptRef = useRef<() => void>(() => {});
  const lastCommandRef = useRef<string>("");
  const abortRef = useRef<AbortController | null>(null);
  const termCleanupRef = useRef<(() => void)>(() => {});
  const [isDragging, setIsDragging] = useState(false);

  const writeln = useCallback(
    (text: string, color = "37") => termRef.current?.writeln(`\x1b[${color}m${text}\x1b[0m`),
    [],
  );

  const writePrompt = useCallback(() => termRef.current?.write(promptRef.current), []);

  const executeCommand = useCallback(
    async (cmd: string) => {
      if (!cmd.trim() || loadingRef.current || !agentId) return;
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;
      loadingRef.current = true;
      setLoading(true);
      lastCommandRef.current = cmd;
      try {
        const st = await runTask(
          agentId,
          `/agents/${agentId}/shell`,
          {
            method: "postJson",
            body: { command: cmd, shell: shellType },
            checkOnline: true,
            timeoutMs: 60000,
            signal: ac.signal,
            onStatus: (s) => {
              if (s.status === "pending" || s.status === "running") {
                writeln("⏳ waiting for agent to return output...", "90");
              }
            },
          },
        );
        if (st.status === "failed") {
          writeln(st.error || "Command failed", "31");
        } else if (st.result) {
          const highlighted = highlightOutput(st.result, lastCommandRef.current);
          termRef.current?.write(highlighted.replace(/\n/g, "\r\n"));
          termRef.current?.write("\r\n");
        } else {
          writeln("(no output)", "33");
        }
      } catch (e) {
        writeln(String(e), "31");
      } finally {
        loadingRef.current = false;
        setLoading(false);
        writePrompt();
        termRef.current?.focus();
      }
    },
    [agentId, shellType, writeln, writePrompt],
  );

  useEffect(() => { execRef.current = executeCommand; }, [executeCommand]);
  useEffect(() => { writePromptRef.current = writePrompt; }, [writePrompt]);

  useEffect(() => {
    return () => { abortRef.current?.abort(); };
  }, []);

  useEffect(() => {
    if (!containerRef.current) return;

    let term: Terminal | null = null;
    let fit: FitAddon | null = null;
    let disposed = false;

    const initTerminal = async () => {
      const [{ Terminal }, { FitAddon }] = await Promise.all([
        import("@xterm/xterm"),
        import("@xterm/addon-fit"),
      ]);
      if (disposed) return;
      // Load xterm CSS dynamically
      if (!document.querySelector('link[href*="xterm.css"]')) {
        const link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = "/css/xterm.css";
        document.head.appendChild(link);
      }

      term = new Terminal({
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
        allowProposedApi: false,
      });

      fit = new FitAddon();
      term.loadAddon(fit);
      term.open(containerRef.current!);
      fit.fit();

      termRef.current = term;
      fitRef.current = fit;

      historyRef.current = loadCommandHistory();
      term.writeln("\x1b[90mForgeC2 Shell \u00b7 xterm.js \u00b7 Enter to run \u00b7 \u2191\u2193 history \u00b7 Ctrl+L clear\x1b[0m");
      writePrompt();

      term.onData((data: string) => {
        if (loadingRef.current) return;

        const t = term;
        if (!t) return;
        if (data === "\r") {
          const cmd = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString().trim() || "";
          if (cmd.startsWith(plainPromptRef.current.trim())) {
            const trimmed = cmd.slice(plainPromptRef.current.trim().length).trim();
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
          const promptLen = plainPromptRef.current.length;
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
            const promptLen = plainPromptRef.current.length;
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
            const promptLen = plainPromptRef.current.length;
            for (let i = line.length; i > promptLen; i--) t.write("\b \b");
            t.write(cmd);
          } else {
            histIdxRef.current = historyRef.current.length;
            const line = t.buffer.active.getLine(t.buffer.active.cursorY)?.translateToString() || "";
            const promptLen = plainPromptRef.current.length;
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
          const line = t.buffer.active.getLine(t.buffer.active.cursorY);
          if (!line) return;
          const promptLen = plainPromptRef.current.length;
          const currentInput = line.translateToString().substring(promptLen).trim();
          const matches = getCompletions(currentInput, osType || "windows");

          if (matches.length === 1) {
            const completed = matches[0];
            const rawLine = line.translateToString();
            for (let i = rawLine.length - 1; i >= promptLen; i--) {
              t.write("\b \b");
            }
            t.write(completed);
          } else if (matches.length > 1) {
            t.write("\r\n");
            t.write(matches.join("  "));
            t.write("\r\n");
            t.write(promptRef.current);
            const rawLine = line.translateToString();
            t.write(rawLine.substring(promptLen));
          }
          return;
        }

        if (data.length === 1) {
          t.write(data);
        }
      });

      const ro = new ResizeObserver(() => fit?.fit());
      if (containerRef.current) ro.observe(containerRef.current);

      termCleanupRef.current = () => {
        ro.disconnect();
        term?.dispose();
      };
    };

    initTerminal();

    return () => {
      disposed = true;
      termCleanupRef.current();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [agentId, executeCommand, writeln, fontSize, writePrompt, osType]);

  useEffect(() => {
    if (termRef.current) termRef.current.options.fontSize = fontSize;
  }, [fontSize]);

  useEffect(() => {
    if (!agentId) return;
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
    <div className="h-[calc(100vh-9rem)] flex flex-col bg-background text-foreground rounded-xl overflow-hidden border border-border relative">
      {showHeader && (
        <div className="shrink-0 bg-card border-b border-border px-4 py-3 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="w-2 h-2 bg-emerald-500 rounded-full shrink-0"></span>
            <span className="font-semibold text-sm text-foreground truncate">Shell</span>
            <span className="text-xs text-muted-foreground font-mono uppercase">{shellType}</span>
            <Tooltip>
              <TooltipTrigger>
                <span className="text-xs text-muted-foreground/70">
                   <Clock className="w-3 h-3 mr-1 inline" />
                  {beaconHint || "loading..."}
                </span>
              </TooltipTrigger>
              <TooltipContent>Beacon timing</TooltipContent>
            </Tooltip>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <div className="flex items-center gap-1 text-xs text-muted-foreground mr-2">
               <Type className="w-4 h-4" />
              <Select value={String(fontSize)} onValueChange={(v) => {
                  const val = Number(v ?? 14);
                  setFontSize(val);
                  localStorage.setItem(KEY, String(val));
                }}>
                <SelectTrigger className="w-auto">
                  <SelectValue placeholder="14" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="12">12</SelectItem>
                  <SelectItem value="13">13</SelectItem>
                  <SelectItem value="14">14</SelectItem>
                  <SelectItem value="16">16</SelectItem>
                </SelectContent>
              </Select>
              </div>
            {quickCommands.map((cmd) => (
              <Button
                key={cmd}
                variant="outline"
                size="sm"
                onClick={() => runQuick(cmd)}
                disabled={loading}
                className="whitespace-nowrap"
              >
                {cmd}
              </Button>
            ))}
            <Tooltip>
              <TooltipTrigger render={<Button
                  variant="outline"
                  size="icon-sm"
                  onClick={() => { clearCommandHistory(); historyRef.current = []; histIdxRef.current = 0; termRef.current?.clear(); writeln("History cleared", "33"); }}
                  aria-label={t("common.clear_history")}
                />}>
                <Trash2 className="w-4 h-4" />
              </TooltipTrigger>
              <TooltipContent>{t("common.clear_history")}</TooltipContent>
            </Tooltip>
          </div>
        </div>
      )}
      <div
        ref={containerRef}
        className={`flex-1 min-h-0 bg-background relative ${isDragging ? "ring-2 ring-primary" : ""}`}
        onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={async (e) => {
          e.preventDefault();
          setIsDragging(false);
          const files = Array.from(e.dataTransfer.files);
          if (files.length === 0) return;
          for (const file of files) {
            const reader = new FileReader();
            reader.onload = () => {
              const b64 = (reader.result as string).split(",")[1];
              const cmd = `upload ${file.name} ${b64}`;
              if (execRef.current) execRef.current(cmd);
            };
            reader.readAsDataURL(file);
          }
        }}
      />
      {isDragging && (
        <div className="absolute inset-0 bg-primary/10 border-2 border-dashed border-primary rounded-lg flex items-center justify-center z-10 pointer-events-none">
          <span className="text-primary font-medium">Drop file to upload</span>
        </div>
      )}
      {loading && (
        <div className="shrink-0 bg-card border-t border-border px-4 py-1.5 text-xs text-emerald-400 flex items-center gap-2">
           <Spinner size="xs" /> Executing...
        </div>
      )}
      <div className="shrink-0 bg-background border-t border-border px-4 py-1.5 text-(--font-size-micro-sm) text-muted-foreground/70 flex items-center justify-between">
        <span>
           <Keyboard className="w-3 h-3 mr-1 inline" />
          {osType === "linux" ? "Ctrl+D" : "Ctrl+Z"} · Ctrl+C: interrupt
        </span>
        <span>
          Font: {fontSize}px
        </span>
      </div>
    </div>
  );
}
