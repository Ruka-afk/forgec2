"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { runTask, ApiError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentBeaconTiming, loadCommandHistory, saveCommandHistory } from "@/lib/shell";
import { getCompletions } from "@/lib/completions";
import { highlightOutput } from "@/lib/highlight";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/ui/status-dot";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Clock, Type, Trash2, Keyboard, Copy, Check } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";

const PROMPT_CHARS = ["$", "#", ">", "%"];
const KEY = "forgec2_shell_fontsize";
const MAX_DRAG_UPLOAD_BYTES = 4 * 1024;

function buildTerminalTheme() {
  const cs = typeof window === "undefined" ? null : getComputedStyle(document.documentElement);
  const v = (name: string, fallback: string) => cs?.getPropertyValue(name).trim() || fallback;
  const primary = v("--primary", "#4f46e5");
  const isDark = typeof document !== "undefined" && document.documentElement.classList.contains("dark");
  return {
    background: isDark ? v("--card", "#0f172a") : v("--background", "#f8fafc"),
    foreground: v("--foreground", "#0f172a"),
    cursor: primary,
    cursorAccent: v("--background", "#020617"),
    selectionBackground: `color-mix(in oklch, ${primary} 28%, transparent)`,
    selectionForeground: v("--foreground", "#e2e8f0"),
  };
}

function fmtTime(d = new Date()) {
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false });
}

function clearCommandHistory() {
  try { localStorage.removeItem("forgec2_shell_history"); } catch { /* ignore */ }
}

export default function ShellTerminal({
  agentId,
  shellType = "cmd",
  showHeader = true,
  osType = "windows",
  className,
}: {
  agentId: string;
  shellType?: string;
  showHeader?: boolean;
  osType?: string;
  className?: string;
}) {
  const { resolved } = useTheme();
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
  const fontSizeRef = useRef(fontSize);
  fontSizeRef.current = fontSize;
  const osTypeRef = useRef(osType);
  osTypeRef.current = osType;
  const [beaconHint, setBeaconHint] = useState("");
  const [lastOutput, setLastOutput] = useState("");
  const [copyFlash, setCopyFlash] = useState(false);
  const copyFlashTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const a11yLogRef = useRef<HTMLDivElement>(null);

  // xterm renders to canvas, which screen readers cannot see. Mirror plain-text
  // output into an off-screen live region so AT users hear command results.
  const announce = useCallback((text: string) => {
    const el = a11yLogRef.current;
    if (!el || !text) return;
    const line = document.createElement("div");
    line.textContent = text;
    el.appendChild(line);
    while (el.childElementCount > 30) el.removeChild(el.firstElementChild!);
  }, []);

  const [promptChar] = useState(() => PROMPT_CHARS[Math.floor(Math.random() * PROMPT_CHARS.length)]);
  const promptRef = useRef(`\x1b[94m${promptChar}\x1b[0m `);
  const execRef = useRef<(cmd: string) => Promise<void>>(async () => {});
  const writePromptRef = useRef<() => void>(() => {});
  const lastCommandRef = useRef<string>("");
  const abortRef = useRef<AbortController | null>(null);
  const waitingWrittenRef = useRef(false);
  const inputRef = useRef("");
  const tRef = useRef(t);
  tRef.current = t;
  const termCleanupRef = useRef<(() => void)>(() => {});
  const [isDragging, setIsDragging] = useState(false);

  const writeln = useCallback(
    (text: string, color = "37") => {
      termRef.current?.writeln(`\x1b[${color}m${text}\x1b[0m`);
      announce(text);
    },
    [announce],
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
      waitingWrittenRef.current = false;
      // Header line with timestamp — separates commands visually and gives operator timeline.
      // C11 fix: for upload commands, truncate the displayed command to just the
      // filename so the header line doesn't contain thousands of base64 characters.
      const displayCmd = cmd.length > 200 && cmd.startsWith("upload ") ? `upload ${cmd.slice(7, cmd.indexOf(" ", 7))}` : cmd;
      termRef.current?.writeln(`\x1b[90m─ ${fmtTime()}  ${promptChar} ${displayCmd}\x1b[0m`);
      let rendered = 0;
      let fullOut = "";
      const appendOutput = (text: string, announceText: string) => {
        if (text.length <= rendered) return;
        const fresh = text.slice(rendered);
        rendered = text.length;
        fullOut += fresh;
        const highlighted = highlightOutput(fresh, lastCommandRef.current);
        termRef.current?.write(highlighted.replace(/\n/g, "\r\n"));
        if (announceText) announce(announceText);
      };
      try {
        const st = await runTask(
          agentId,
          paths.agents.cmd(agentId, "shell"),
          {
            method: "postJson",
            body: { command: cmd, shell: shellType },
            checkOnline: true,
            timeoutMs: 60000,
            signal: ac.signal,
            onStatus: (s) => {
              if ((s.status === "pending" || s.status === "running") && !waitingWrittenRef.current) {
                waitingWrittenRef.current = true;
                writeln(`◷ ${t("shell.waiting_output")}`, "90");
              }
              if (s.result) appendOutput(s.result, "");
            },
          },
        );
        if (st.status === "failed") {
          const msg = st.error || t("shell.command_failed");
          writeln(`✕ ${msg}`, "31");
          setLastOutput(fullOut || msg);
        } else if (st.result) {
          appendOutput(st.result, st.result);
          termRef.current?.write("\r\n");
          // success tick with duration hint
          termRef.current?.writeln(`\x1b[32m✓\x1b[0m \x1b[90m${fmtTime()}\x1b[0m`);
          setLastOutput(st.result);
        } else {
          writeln(`○ ${t("shell.no_output")}`, "33");
          setLastOutput("");
        }
      } catch (e) {
        if (!ac.signal.aborted) {
          if (e instanceof ApiError && e.status === 409) {
            writeln(`⚠ ${String(e.message)}`, "33");
          } else {
            writeln(`✕ ${String(e)}`, "31");
          }
        }
      } finally {
        loadingRef.current = false;
        setLoading(false);
        if (!ac.signal.aborted) {
          writePrompt();
          termRef.current?.focus();
        }
      }
    },
    [agentId, shellType, t, writeln, writePrompt, announce, promptChar],
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
        fontSize: fontSizeRef.current,
        fontFamily: "'JetBrains Mono', 'Cascadia Code', Consolas, monospace",
        theme: buildTerminalTheme(),
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
      term.writeln(`\x1b[90m${tRef.current("shell.banner")}\x1b[0m`);
      writePromptRef.current();

      term.onData((data: string) => {
        const t = term;
        if (!t) return;
        if (data === "\x03") {
          // Ctrl+C — interrupt the in-flight command (if any)
          if (abortRef.current) {
            abortRef.current.abort();
            abortRef.current = null;
          }
          t.writeln("^C");
          loadingRef.current = false;
          setLoading(false);
          inputRef.current = "";
          writePromptRef.current();
          return;
        }
        if (loadingRef.current) return;

        if (data === "\r") {
          const cmd = inputRef.current.trim();
          inputRef.current = "";
          t.writeln("");
          if (cmd) {
            saveCommandHistory(cmd);
            historyRef.current = loadCommandHistory();
            // C10 fix: sync histIdxRef so Up-arrow works immediately on fresh
            // mount. Without this, Up does nothing until the first Enter press
            // of the session because histIdxRef remains at -1 while the Up
            // handler requires > 0.
            histIdxRef.current = historyRef.current.length;
            execRef.current(cmd);
          } else {
            writePromptRef.current();
          }
          return;
        }

        if (data === "\u007f") {
          if (inputRef.current.length > 0) {
            inputRef.current = inputRef.current.slice(0, -1);
            t.write("\b \b");
          }
          return;
        }

        if (data === "\x1b[A") {
          if (historyRef.current.length > 0 && histIdxRef.current > 0) {
            histIdxRef.current--;
            const cmd = historyRef.current[histIdxRef.current];
            for (let i = inputRef.current.length; i > 0; i--) t.write("\b \b");
            inputRef.current = cmd;
            t.write(cmd);
          }
          return;
        }

        if (data === "\x1b[B") {
          // histIdxRef -1 means "fresh prompt" (nothing browsed yet) — Down
          // must be a no-op, not jump to the OLDEST saved entry.
          if (histIdxRef.current >= 0 && histIdxRef.current < historyRef.current.length - 1) {
            histIdxRef.current++;
            const cmd = historyRef.current[histIdxRef.current];
            for (let i = inputRef.current.length; i > 0; i--) t.write("\b \b");
            inputRef.current = cmd;
            t.write(cmd);
          } else if (histIdxRef.current >= 0) {
            histIdxRef.current = historyRef.current.length;
            for (let i = inputRef.current.length; i > 0; i--) t.write("\b \b");
            inputRef.current = "";
          }
          return;
        }

        if (data === "\x0c") {
          t.clear();
          writePromptRef.current();
          t.write(inputRef.current);
          return;
        }

        if (data === "\x09") {
          // C5 fix: only append the suffix delta between the completion and
          // the current input. Previously Tab replaced the entire input with
          // the first match which cased e.g. "dir C:\Windows" → "dir" (the
          // raw completion without the original path prefix).
          // C17 fix: trim for completion lookup but use the untrimmed length
          // for suffix calculation to preserve leading whitespace in input.
          const currentInput = inputRef.current;
          const trimmed = currentInput.trim();
          const matches = getCompletions(trimmed, osTypeRef.current || "windows");

          if (matches.length === 1) {
            const completed = matches[0];
            const suffix = completed.slice(trimmed.length);
            if (suffix) {
              inputRef.current = currentInput + suffix;
              t.write(suffix);
            }
          } else if (matches.length > 1) {
            t.write("\r\n");
            t.write(matches.join("  "));
            t.write("\r\n");
            t.write(promptRef.current);
            t.write(currentInput);
          }
          return;
        }

        if (!data.startsWith("\x1b")) {
          inputRef.current += data;
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

    initTerminal().catch((err) => {
      if (!disposed) termRef.current?.writeln(`\x1b[31m${tRef.current("shell.init_failed", { error: String(err) })}\x1b[0m`);
    });

    return () => {
      disposed = true;
      abortRef.current?.abort();
      abortRef.current = null;
      inputRef.current = "";
      if (copyFlashTimerRef.current) clearTimeout(copyFlashTimerRef.current);
      termCleanupRef.current();
      termRef.current = null;
      fitRef.current = null;
    };
  }, [agentId]);

  useEffect(() => {
    if (termRef.current) termRef.current.options.fontSize = fontSize;
    requestAnimationFrame(() => fitRef.current?.fit());
  }, [fontSize]);

  useEffect(() => {
    if (termRef.current) termRef.current.options.theme = buildTerminalTheme();
  }, [resolved]);

  useEffect(() => {
    if (!agentId) return;
    let cancelled = false;
    fetchAgentBeaconTiming(agentId)
      .then(({ interval, jitter }) => {
        if (cancelled) return;
        setBeaconHint(interval === 0
          ? t("shell.beacon_rt", { jitter })
          : t("shell.beacon_interval", { interval, jitter }));
      })
      .catch(() => {
        if (!cancelled) setBeaconHint("");
      });
    return () => { cancelled = true; };
  }, [agentId, t]);

  const quickCommands =
    osType === "linux"
      ? ["whoami", "id", "uname -a", "hostname", "ps aux", "ip addr"]
      : ["whoami", "hostname", "ipconfig", "systeminfo", "netstat -ano", "tasklist"];

  const runQuick = (cmd: string) => {
    if (loadingRef.current) return;
    inputRef.current = "";
    // No manual echo here: executeCommand writes its own timestamped header —
    // writing both printed the command twice per quick click.
    void executeCommand(cmd);
  };

  const handleCopyLast = async () => {
    if (!lastOutput) return;
    try { await navigator.clipboard.writeText(lastOutput); setCopyFlash(true); if (copyFlashTimerRef.current) clearTimeout(copyFlashTimerRef.current); copyFlashTimerRef.current = setTimeout(() => setCopyFlash(false), 1200); } catch { /* ignore */ }
  };

  const handleClear = () => {
    historyRef.current = [];
    // keep persisted history but clear session view; persisted stays for Up/Down
    termRef.current?.clear();
    termRef.current?.writeln(`\x1b[90m${t("shell.banner")}\x1b[0m`);
    writePromptRef.current();
  };

  return (
    <div className={className || "relative flex h-full min-h-[20rem] flex-col overflow-hidden rounded-lg border border-border bg-card text-foreground shadow-sm"}>
      {showHeader && (
        <div className="shrink-0 bg-card/90 backdrop-blur supports-[backdrop-filter]:bg-card/80 border-b border-border px-3 py-2.5 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="size-7 rounded-lg bg-primary/10 text-primary flex items-center justify-center shrink-0 ring-1 ring-primary/15">
              <Keyboard className="size-4" />
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-semibold text-sm tracking-tight truncate">{t("shell.title")}</span>
                <span className="inline-flex items-center gap-1 text-(--fs-micro-sm) font-bold tracking-widest uppercase px-1.5 py-0.5 rounded bg-secondary text-muted-foreground ring-1 ring-border/50">{shellType}</span>
                <span className="hidden sm:inline-flex items-center gap-1.5 text-xs text-muted-foreground/80 font-mono">
                  <StatusDot tone="success" size="sm" pulse />
                  <span className="truncate max-w-[10rem]">{agentId.slice(0, 8)}…</span>
                </span>
              </div>
              <div className="flex items-center gap-2 mt-0.5">
                <span className="inline-flex items-center gap-1 text-xs text-muted-foreground/100">
                  <Clock className="size-3" />
                  {beaconHint || t("shell.beacon_loading")}
                </span>
                {loading && <span className="inline-flex items-center gap-1 text-xs text-chart-1"><Spinner size="xs" />{t("shell.executing")}</span>}
              </div>
            </div>
          </div>
          <div className="flex items-center gap-1 shrink-0 flex-wrap justify-end">
            <div className="hidden md:flex items-center gap-1 text-xs text-muted-foreground mr-1">
              <Type className="size-3.5" />
              <Select value={String(fontSize)} onValueChange={(v) => {
                  const val = Number(v ?? 14);
                  setFontSize(val);
                  localStorage.setItem(KEY, String(val));
                }}>
                <SelectTrigger className="h-7 w-auto text-xs" aria-label={t("shell.font_size")}>
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
            <div className="hidden lg:flex items-center gap-1">
            {quickCommands.slice(0, 4).map((cmd) => (
              <Button
                key={cmd}
                variant="outline"
                size="sm"
                onClick={() => runQuick(cmd)}
                disabled={loading}
                className="whitespace-nowrap h-7 text-xs"
              >
                {cmd}
              </Button>
            ))}
            </div>
            <Tooltip>
              <TooltipTrigger render={<Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={handleCopyLast}
                  disabled={!lastOutput}
                  aria-label={t("shell.copy_last_output")}
                />}>
                {copyFlash ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
              </TooltipTrigger>
              <TooltipContent>{t("shell.copy_last_output")}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger render={<Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => { clearCommandHistory(); historyRef.current = []; histIdxRef.current = 0; termRef.current?.clear(); writeln(t("shell.history_cleared"), "33"); }}
                  aria-label={t("common.clear_history")}
                />}>
                <Trash2 className="size-4" />
              </TooltipTrigger>
              <TooltipContent>{t("common.clear_history")}</TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={handleClear} aria-label={t("shell.clear_screen")} />}>
                <span className="text-xs font-mono">⌧</span>
              </TooltipTrigger>
              <TooltipContent>{t("shell.clear_screen")}</TooltipContent>
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
          // C4 fix: serialize drag-drop uploads instead of firing them all in
          // parallel. Concurrent upload commands race the agent task queue and
          // cause partial/truncated transfers. We now wait for each upload to
          // finish before starting the next one.
          for (const file of files) {
            if (file.size > MAX_DRAG_UPLOAD_BYTES) {
              writeln(
                `${t("shell.drop_upload_too_large")} (${Math.ceil(file.size / 1024)} KB > ${Math.floor(MAX_DRAG_UPLOAD_BYTES / 1024)} KB) — ${t("shell.drop_upload_use_files")}`,
                "31",
              );
              continue;
            }
            const reader = new FileReader();
            await new Promise<void>((resolve) => {
              // onerror MUST resolve too: an unsettled promise would wedge
              // the whole upload queue and silently skip remaining files.
              reader.onerror = () => {
                writeln(`${t("shell.drop_upload_read_failed")}: ${file.name}`, "31");
                resolve();
              };
              reader.onload = () => {
                const b64 = (reader.result as string).split(",")[1];
                const cmd = `upload ${file.name} ${b64}`;
                // C4 fix: only show filename in the input area, not the raw
                // base64 payload. The full command (with b64) is sent to the
                // agent via execRef; echoing base64 into the visible input
                // spams the terminal with thousands of garbage characters.
                inputRef.current = `upload ${file.name}`;
                termRef.current?.write(`upload ${file.name}\r\n`);
                // AWAIT the execution: executeCommand refuses concurrent runs
                // (loadingRef guard), so firing without awaiting made every
                // file after the first silently vanish — the echo above still
                // showed it as typed. Serializing honors the "wait for each"
                // contract the loop was written for.
                const run = execRef.current ? execRef.current(cmd) : Promise.resolve();
                Promise.resolve(run).catch(() => {}).finally(resolve);
              };
              reader.readAsDataURL(file);
            });
          }
        }}
      />
      {isDragging && (
        <div className="absolute inset-0 bg-primary/10 border-2 border-dashed border-primary rounded-lg flex items-center justify-center z-10 pointer-events-none">
          <span className="text-primary font-medium">{t("shell.drop_file")}</span>
        </div>
      )}
      <div ref={a11yLogRef} role="log" aria-live="polite" aria-atomic="false" className="sr-only" />
      {loading && (
        <div className="shrink-0 bg-card border-t border-border px-4 py-1.5 text-xs text-chart-1 flex items-center gap-2">
           <Spinner size="xs" /> {t("shell.executing")}
        </div>
      )}
      <div className="shrink-0 bg-background border-t border-border px-4 py-1.5 text-(--fs-micro-sm) text-muted-foreground/100 flex items-center justify-between">
        <span>
           <Keyboard className="size-3 mr-1 inline" />
          {t("shell.footer_shortcuts", { exitKey: osType === "linux" ? "Ctrl+D" : "Ctrl+Z" })}
        </span>
        <span>
          {t("shell.font_label", { size: fontSize })}
        </span>
      </div>
    </div>
  );
}
