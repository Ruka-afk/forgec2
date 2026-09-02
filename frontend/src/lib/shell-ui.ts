/** Pure helpers for the interactive shell UI (prompt, identity, error text). */

export const SHELL_TERM_BG = "#0b1220";
export const SHELL_FONT_KEY = "forgec2_shell_fontsize";
export const SHELL_HISTORY_KEY = "forgec2_shell_history";

export function defaultInterpreter(osType?: string): string {
  return osType === "linux" ? "/bin/sh" : "cmd.exe";
}

export function interpreterOptions(osType?: string): string[] {
  return osType === "linux" ? ["/bin/sh", "/bin/bash"] : ["cmd.exe", "powershell.exe"];
}

export function quickCommands(osType?: string): string[] {
  return osType === "linux"
    ? ["whoami", "id", "uname -a", "hostname", "ps aux", "ip addr"]
    : ["whoami", "hostname", "ipconfig", "systeminfo", "netstat -ano", "tasklist"];
}

export function sessionPromptLabel(opts: {
  osType?: string;
  hostname?: string;
  username?: string;
  interpreter?: string;
}): string {
  const host = (opts.hostname || "").trim() || "agent";
  const user = (opts.username || "").trim();
  const interp = opts.interpreter || defaultInterpreter(opts.osType);
  if (interp.toLowerCase().includes("powershell")) return `PS ${host}>`;
  if (opts.osType === "linux") return `${user || "root"}@${host}$`;
  return user ? `${host}\\${user}>` : `${host}>`;
}

export function sessionPromptSeq(opts: {
  osType?: string;
  hostname?: string;
  username?: string;
  interpreter?: string;
}): string {
  return `\x1b[38;2;125;211;252m${sessionPromptLabel(opts)}\x1b[0m `;
}

export function agentIdentityTitle(hostname?: string, username?: string, fallbackId?: string): string {
  const host = (hostname || "").trim() || (fallbackId ? fallbackId.slice(0, 8) : "agent");
  const user = (username || "").trim();
  return user ? `${host} · ${user}` : host;
}

export function pickAgentField(ag: Record<string, unknown>, ...keys: string[]): string {
  for (const k of keys) {
    const v = ag[k];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  return "";
}

/** Hide at-rest ciphertext if AfterFind missed a field. */
export function operatorErrorText(raw: string | undefined, fallback: string): string {
  if (!raw) return fallback;
  if (raw.startsWith("FC2ENC:") || raw.startsWith("FC2EXT:")) return fallback;
  return raw;
}

/** Restore whitespace entities produced by text/HTML transport boundaries. */
export function decodeShellWhitespace(text: string): string {
  return text
    .replace(/&nbsp;/gi, " ")
    .replace(/&#(?:x([0-9a-f]+)|(\d+));/gi, (entity, hex: string | undefined, decimal: string | undefined) => {
      const codePoint = Number.parseInt(hex ?? decimal ?? "", hex ? 16 : 10);
      if (codePoint === 160) return " ";
      return codePoint === 9 || codePoint === 10 || codePoint === 13 || codePoint === 32
        ? String.fromCodePoint(codePoint)
        : entity;
    });
}

export function truncateUploadDisplay(cmd: string): string {
  if (cmd.length > 200 && cmd.startsWith("upload ")) {
    const nameEnd = cmd.indexOf(" ", 7);
    return `upload ${nameEnd > 7 ? cmd.slice(7, nameEnd) : cmd.slice(7)}`;
  }
  return cmd;
}

export function buildTerminalTheme() {
  return {
    background: SHELL_TERM_BG,
    foreground: "#e2e8f0",
    cursor: "#38bdf8",
    cursorAccent: SHELL_TERM_BG,
    selectionBackground: "rgba(56, 189, 248, 0.28)",
    selectionForeground: "#f8fafc",
    black: "#0f172a",
    red: "#f87171",
    green: "#4ade80",
    yellow: "#fbbf24",
    blue: "#38bdf8",
    magenta: "#c084fc",
    cyan: "#22d3ee",
    white: "#e2e8f0",
    brightBlack: "#64748b",
    brightRed: "#fca5a5",
    brightGreen: "#86efac",
    brightYellow: "#fde68a",
    brightBlue: "#7dd3fc",
    brightMagenta: "#d8b4fe",
    brightCyan: "#67e8f9",
    brightWhite: "#f8fafc",
  };
}
