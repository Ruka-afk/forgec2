"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import { runTask, ApiError, formatThrownError } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentBeaconTiming, loadCommandHistory, saveCommandHistory } from "@/lib/shell";
import { getCompletions } from "@/lib/completions";
import { highlightOutput } from "@/lib/highlight";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Copy, Check, Eraser, TerminalSquare } from "lucide-react";
import type { AgentStatus } from "@/types/agent";
import { useI18n } from "@/lib/i18n";
import { timeAgo } from "@/lib/utils";
import {
  SHELL_FONT_KEY,
  SHELL_TERM_BG,
  buildTerminalTheme,
  decodeShellWhitespace,
  defaultInterpreter,
  interpreterOptions,
  operatorErrorText,
  quickCommands,
  sessionPromptSeq,
  truncateUploadDisplay,
} from "@/lib/shell-ui";

const MAX_DRAG_UPLOAD_BYTES = 4 * 1024;

export default function ShellTerminal({
  agentId,
  shellType,
  osType = "windows",
  hostname,
  username,
  lastSeen,
  status,
  className,
}: {
  agentId: string;
  shellType?: string;
  osType?: string;
  hostname?: string;
  username?: string;
  ip?: string;
  lastSeen?: string;
  status?: AgentStatus;
  className?: string;
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
    return Number(localStorage.getItem(SHELL_FONT_KEY)) || 14;
  });
  const fontSizeRef = useRef(fontSize);
  fontSizeRef.current = fontSize;
  const osTypeRef = useRef(osType);
  osTypeRef.current = osType;
  const statusRef = useRef(status);
  statusRef.current = status;
  const lastSeenRef = useRef(lastSeen);
  lastSeenRef.current = lastSeen;
  const [interpreter, setInterpreter] = useState(() => shellType || defaultInterpreter(osType));
  const interpreterRef = useRef(interpreter);
  interpreterRef.current = interpreter;
  useEffect(() => {
    setInterpreter(shellType || defaultInterpreter(osType));
  }, [agentId, osType, shellType]);
  const [beaconHint, setBeaconHint] = useState("");
  const [lastOutput, setLastOutput] = useState("");
  const [copyFlash, setCopyFlash] = useState(false);
  const copyFlashTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const a11yLogRef = useRef<HTMLDivElement>(null);

  const announce = useCallback((text: string) => {
    const el = a11yLogRef.current;
    if (!el || !text) return;
    const line = document.createElement("div");
    line.textContent = text;
    el.appendChild(line);
    while (el.childElementCount > 30) el.removeChild(el.firstElementChild!);
  }, []);

  const promptOpts = { osType, hostname, username, interpreter };
  const promptRef = useRef(sessionPromptSeq(promptOpts));
  useEffect(() => {
    promptRef.current = sessionPromptSeq({ osType, hostname, username, interpreter });
  }, [osType, hostname, username, interpreter]);
  const execRef = useRef<(cmd: string) => Promise<void>>(async () => {});
  const writePromptRef = useRef<() => void>(() => {});
  const lastCommandRef = useRef<string>("");
  const abortRef = useRef<AbortController | null>(null);
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
      const stNow = statusRef.current;
      if (stNow && stNow !== "online") {
        writeln(`✕ ${tRef.current("shell.not_online_hint", {
          status: tRef.current(`agents.${stNow}_label`),
          when: timeAgo(lastSeenRef.current, tRef.current),
        })}`, "33");
        writePrompt();
        return;
      }
      abortRef.current?.abort();
      const ac = new AbortController();
      abortRef.current = ac;
      loadingRef.current = true;
      setLoading(true);
      lastCommandRef.current = cmd;
      let rendered = 0;
      let fullOut = "";
      const appendOutput = (text: string, announceText: string) => {
        text = decodeShellWhitespace(text);
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
          paths.agents.command(agentId),
          {
            method: "postJson",
            body: { command: cmd, shell: interpreterRef.current },
            checkOnline: true,
            timeoutMs: 60000,
            signal: ac.signal,
            onStatus: (s) => {
              if (s.result) appendOutput(s.result, "");
            },
          },
        );
        if (st.status === "failed") {
          const msg = operatorErrorText(st.error, tRef.current("shell.command_failed"));
          writeln(`✕ ${msg}`, "31");
          setLastOutput(fullOut || msg);
        } else if (st.result) {
          appendOutput(st.result, st.result);
          if (!st.result.endsWith("\n")) termRef.current?.write("\r\n");
          setLastOutput(st.result);
        } else {
          writeln(tRef.current("shell.no_output"), "90");
          setLastOutput("");
        }
      } catch (e) {
        if (!ac.signal.aborted) {
          const raw = e instanceof ApiError && e.status === 409 ? e.message : formatThrownError(e);
          writeln(`✕ ${operatorErrorText(raw, tRef.current("shell.command_failed"))}`, e instanceof ApiError && e.status === 409 ? "33" : "31");
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
    [agentId, writeln, writePrompt, announce],
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
      writePromptRef.current();

      term.onData((data: string) => {
        const termInst = term;
        if (!termInst) return;
        if (data === "\x03") {
          if (abortRef.current) {
            abortRef.current.abort();
            abortRef.current = null;
          }
          termInst.writeln("^C");
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
          termInst.writeln("");
          if (cmd) {
            saveCommandHistory(cmd);
            historyRef.current = loadCommandHistory();
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
            termInst.write("\b \b");
          }
          return;
        }

        if (data === "\x1b[A") {
          if (historyRef.current.length > 0 && histIdxRef.current > 0) {
            histIdxRef.current--;
            const histCmd = historyRef.current[histIdxRef.current];
            for (let i = inputRef.current.length; i > 0; i--) termInst.write("\b \b");
            inputRef.current = histCmd;
            termInst.write(histCmd);
          }
          return;
        }

        if (data === "\x1b[B") {
          if (histIdxRef.current >= 0 && histIdxRef.current < historyRef.current.length - 1) {
            histIdxRef.current++;
            const histCmd = historyRef.current[histIdxRef.current];
            for (let i = inputRef.current.length; i > 0; i--) termInst.write("\b \b");
            inputRef.current = histCmd;
            termInst.write(histCmd);
          } else if (histIdxRef.current >= 0) {
            histIdxRef.current = historyRef.current.length;
            for (let i = inputRef.current.length; i > 0; i--) termInst.write("\b \b");
            inputRef.current = "";
          }
          return;
        }

        if (data === "\x0c") {
          termInst.clear();
          writePromptRef.current();
          termInst.write(inputRef.current);
          return;
        }

        if (data === "\x09") {
          const currentInput = inputRef.current;
          const trimmed = currentInput.trim();
          const matches = getCompletions(trimmed, osTypeRef.current || "windows");

          if (matches.length === 1) {
            const completed = matches[0];
            const suffix = completed.slice(trimmed.length);
            if (suffix) {
              inputRef.current = currentInput + suffix;
              termInst.write(suffix);
            }
          } else if (matches.length > 1) {
            termInst.write("\r\n");
            termInst.write(matches.join("  "));
            termInst.write("\r\n");
            termInst.write(promptRef.current);
            termInst.write(currentInput);
          }
          return;
        }

        if (!data.startsWith("\x1b")) {
          inputRef.current += data;
          termInst.write(data);
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

  const cmds = quickCommands(osType);
  const interps = interpreterOptions(osType);
  const offline = Boolean(status && status !== "online");

  const runQuick = (cmd: string) => {
    if (loadingRef.current || offline) return;
    inputRef.current = "";
    termRef.current?.write(`${cmd}\r\n`);
    void executeCommand(cmd);
  };

  const handleCopyLast = async () => {
    if (!lastOutput) return;
    try {
      await navigator.clipboard.writeText(lastOutput);
      setCopyFlash(true);
      if (copyFlashTimerRef.current) clearTimeout(copyFlashTimerRef.current);
      copyFlashTimerRef.current = setTimeout(() => setCopyFlash(false), 1200);
    } catch { /* ignore */ }
  };

  const handleClear = () => {
    termRef.current?.clear();
    writePromptRef.current();
    termRef.current?.focus();
  };

  const iconBtn = "size-7 text-slate-300 hover:bg-white/10 hover:text-white";

  return (
    <div className={className || "relative flex h-full min-h-[20rem] flex-col overflow-hidden bg-(--shell-terminal-bg) text-slate-100"}>
      <div className="flex shrink-0 items-center gap-1.5 overflow-x-auto border-b border-white/10 px-2 py-1.5">
        <TerminalSquare className="size-3.5 shrink-0 text-sky-400/80" aria-hidden />
        <span className="sr-only">{t("shell.title")}</span>
        <Select value={interpreter} onValueChange={(v) => { if (v) setInterpreter(v); }}>
          <SelectTrigger className="h-7 w-[9rem] shrink-0 border-white/10 bg-white/5 font-mono text-xs text-slate-200" aria-label={t("shell.interpreter")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {interps.map((name) => (
              <SelectItem key={name} value={name}>{name}</SelectItem>
            ))}
          </SelectContent>
        </Select>
        <div className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto">
          {cmds.map((cmd) => (
            <Button
              key={cmd}
              variant="ghost"
              size="sm"
              onClick={() => runQuick(cmd)}
              disabled={loading || offline}
              className="h-6 shrink-0 px-2 font-mono text-(--fs-micro-sm) text-slate-400 hover:bg-white/10 hover:text-slate-100"
            >
              {cmd}
            </Button>
          ))}
        </div>
        <span className="hidden min-w-0 truncate font-mono text-(--fs-micro-sm) text-slate-500 lg:inline" title={t("shell.banner")}>
          {t("shell.connected_as", {
            user: username || "user",
            host: hostname || agentId.slice(0, 8),
            interpreter,
          })}
          {beaconHint ? ` · ${beaconHint}` : ""}
        </span>
        <Select value={String(fontSize)} onValueChange={(v) => {
            const val = Number(v ?? 14);
            setFontSize(val);
            localStorage.setItem(SHELL_FONT_KEY, String(val));
          }}>
          <SelectTrigger className="hidden h-7 w-14 border-white/10 bg-white/5 text-xs text-slate-200 md:flex" aria-label={t("shell.font_size")}>
            <SelectValue placeholder="14" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="12">12</SelectItem>
            <SelectItem value="13">13</SelectItem>
            <SelectItem value="14">14</SelectItem>
            <SelectItem value="16">16</SelectItem>
            <SelectItem value="18">18</SelectItem>
          </SelectContent>
        </Select>
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={handleCopyLast} disabled={!lastOutput} className={iconBtn} aria-label={t("shell.copy_last_output")} />}>
            {copyFlash ? <Check className="size-4 text-emerald-400" /> : <Copy className="size-4" />}
          </TooltipTrigger>
          <TooltipContent>{t("shell.copy_last_output")}</TooltipContent>
        </Tooltip>
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="icon-sm" className={iconBtn} onClick={handleClear} aria-label={t("shell.clear_screen")} />}>
            <Eraser className="size-4" />
          </TooltipTrigger>
          <TooltipContent>{t("shell.clear_screen")}</TooltipContent>
        </Tooltip>
      </div>
      <div
        ref={containerRef}
        className={`relative min-h-0 flex-1 ${isDragging ? "ring-2 ring-inset ring-sky-400/70" : ""}`}
        style={{ background: SHELL_TERM_BG }}
        onClick={() => termRef.current?.focus()}
        onDragOver={(e) => { e.preventDefault(); setIsDragging(true); }}
        onDragLeave={() => setIsDragging(false)}
        onDrop={async (e) => {
          e.preventDefault();
          setIsDragging(false);
          const files = Array.from(e.dataTransfer.files);
          if (files.length === 0) return;
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
              reader.onerror = () => {
                writeln(`${t("shell.drop_upload_read_failed")}: ${file.name}`, "31");
                resolve();
              };
              reader.onload = () => {
                const b64 = (reader.result as string).split(",")[1];
                const cmd = `upload ${file.name} ${b64}`;
                inputRef.current = `upload ${file.name}`;
                termRef.current?.write(`${truncateUploadDisplay(cmd)}\r\n`);
                const run = execRef.current ? execRef.current(cmd) : Promise.resolve();
                Promise.resolve(run).catch(() => {}).finally(resolve);
              };
              reader.readAsDataURL(file);
            });
          }
        }}
      />
      {isDragging && (
        <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center border-2 border-dashed border-sky-400/60 bg-sky-500/10">
          <span className="font-medium text-sky-200">{t("shell.drop_file")}</span>
        </div>
      )}
      <div ref={a11yLogRef} role="log" aria-live="polite" aria-atomic="false" className="sr-only" />
      <div className="flex shrink-0 items-center justify-between border-t border-white/10 px-3 py-1 font-mono text-(--fs-micro-sm) text-slate-500">
        <span className="inline-flex min-w-0 items-center gap-2 truncate">
          {loading ? (
            <>
              <Spinner size="xs" />
              <span className="text-sky-300">{t("shell.waiting_output")}</span>
            </>
          ) : (
            <span className="truncate">{t("shell.session_hint")} · {t("shell.footer_shortcuts", { exitKey: osType === "linux" ? "Ctrl+D" : "Ctrl+Z" })}</span>
          )}
        </span>
        <span className="shrink-0">{t("shell.font_label", { size: fontSize })}</span>
      </div>
    </div>
  );
}
