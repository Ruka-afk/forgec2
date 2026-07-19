const CYAN = "\x1b[36m";
const GREEN = "\x1b[32m";
const YELLOW = "\x1b[33m";
const BLUE = "\x1b[34m";
const MAGENTA = "\x1b[35m";
const RED = "\x1b[31m";
const RESET = "\x1b[0m";

function detectOutputType(text: string, command: string): "json" | "xml" | "table" | "error" | "plain" {
  const trimmed = text.trim();
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try { JSON.parse(trimmed); return "json"; } catch { /* fall through */ }
  }
  if (trimmed.startsWith("<")) return "xml";
  if (command.match(/^(netstat|tasklist|ps |Get-Process|Get-Service|Get-NetTCPConnection)/i)) return "table";
  if (text.match(/\b(error|exception|failed|FAIL|ACCESS_DENIED|not found)\b/i)) return "error";
  return "plain";
}

function highlightJSON(text: string): string {
  return text
    .replace(/"([^"]+)":/g, `${CYAN}"$1"${RESET}:`)
    .replace(/: "([^"]*?)"/g, `: ${GREEN}"$1"${RESET}`)
    .replace(/: (\d+)/g, `: ${YELLOW}$1${RESET}`)
    .replace(/: (true|false|null)/g, `: ${MAGENTA}$1${RESET}`);
}

function highlightXML(text: string): string {
  return text
    .replace(/(<\/?[a-zA-Z][\s\S]*?>)/g, `${CYAN}$1${RESET}`)
    .replace(/(="[^"]*")/g, `${GREEN}$1${RESET}`);
}

function highlightTable(text: string): string {
  return text
    .replace(/(LISTENING|ESTABLISHED|CLOSE_WAIT|TIME_WAIT|SYN_SENT)/g, `${GREEN}$1${RESET}`)
    .replace(/(Running|Stopped|Paused)/g, `${GREEN}$1${RESET}`)
    .replace(/(\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d+\b)/g, `${YELLOW}$1${RESET}`)
    .replace(/\b(PID:\s*\d+|:\s*\d{2,5})\b/g, `${MAGENTA}$1${RESET}`);
}

function highlightIPs(text: string): string {
  return text.replace(/\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b/g, `${YELLOW}$1${RESET}`);
}

function highlightPaths(text: string): string {
  return text
    .replace(/([A-Z]:\\[\w\\. -]+)/g, `${BLUE}$1${RESET}`)
    .replace(/(\/[\w/. -]{2,})/g, `${BLUE}$1${RESET}`);
}

function highlightErrors(text: string): string {
  return text
    .replace(/\b(error|Error|ERROR|exception|Exception|failed|FAIL|ACCESS_DENIED|denied|Permission denied|not found|No such file)\b/g, `${RED}$1${RESET}`);
}

export function highlightOutput(text: string, command?: string): string {
  if (!text || !text.trim()) return text;

  const type = detectOutputType(text, command || "");

  switch (type) {
    case "json":
      return highlightJSON(text);
    case "xml":
      return highlightXML(text);
    case "table":
      return highlightTable(text);
    case "error":
      return highlightErrors(text);
    default: {
      let result = text;
      result = highlightIPs(result);
      result = highlightPaths(result);
      result = highlightErrors(result);
      result = result.replace(/\b(success|Success|OK|completed|DONE|True|Yes)\b/g, `${GREEN}$1${RESET}`);
      return result;
    }
  }
}
