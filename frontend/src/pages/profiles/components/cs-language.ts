import { StreamLanguage, HighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { EditorView } from "@codemirror/view";
import {
  autocompletion,
  snippetCompletion,
  type Completion,
  type CompletionContext,
  type CompletionResult,
} from "@codemirror/autocomplete";
import { tags } from "@lezer/highlight";

/**
 * A small CS Malleable C2 stream highlighter. It deliberately tokenizes the
 * generic grammar (set statements, blocks, quoted strings, transforms) instead
 * of enumerating every CS keyword, so unknown/unsupported blocks still look
 * like valid profile text while the server reports semantic warnings.
 */
export const csLanguage = StreamLanguage.define<null>({
  name: "malleable-c2",
  startState: () => null,
  token(stream) {
    if (stream.eatSpace()) return null;
    if (stream.peek() === "#") {
      stream.skipToEnd();
      return "comment";
    }
    if (stream.peek() === '"') {
      stream.next();
      let escaped = false;
      while (!stream.eol()) {
        const ch = stream.next();
        if (escaped) escaped = false;
        else if (ch === "\\") escaped = true;
        else if (ch === '"') break;
      }
      return "string";
    }
    if (stream.eat("{")) return "bracket";
    if (stream.eat("}")) return "bracket";
    if (stream.eat(";")) return "punctuation";
    if (stream.match(/^-?\d+(?:\.\d+)?/)) return "number";
    if (stream.match(/^[A-Za-z_][\w-]*/)) {
      const word = stream.current().toLowerCase();
      if (word === "set") return "keyword";
      if (/^(http-(get|post|config|stager)|client|server|metadata|id|output|forgec2|post-ex|process-inject|stage|ssl-certificate|dns-beacon)$/.test(word)) {
        return "typeName";
      }
      return "variableName";
    }
    stream.next();
    return null;
  },
});

export const csEditorTheme = EditorView.theme({
  "&": {
    height: "100%",
    minHeight: "420px",
    fontSize: "13px",
    backgroundColor: "transparent",
    color: "var(--foreground)",
  },
  ".cm-scroller": {
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace",
    lineHeight: "1.65",
    overflow: "auto",
  },
  ".cm-content": {
    caretColor: "var(--primary)",
    padding: "14px 0 40px",
  },
  ".cm-line": {
    padding: "0 14px",
  },
  ".cm-gutters": {
    backgroundColor: "transparent",
    color: "var(--muted-foreground)",
    border: "none",
    paddingLeft: "6px",
  },
  ".cm-activeLine": {
    backgroundColor: "color-mix(in oklab, var(--muted) 55%, transparent)",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "transparent",
    color: "var(--foreground)",
  },
  "&.cm-focused": { outline: "none" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground": {
    backgroundColor: "color-mix(in oklab, var(--primary) 28%, transparent)",
  },
  ".cm-cursor": { borderLeftColor: "var(--primary)" },
  ".cm-tooltip": {
    border: "1px solid var(--border)",
    backgroundColor: "var(--popover)",
    color: "var(--popover-foreground)",
    borderRadius: "8px",
    boxShadow: "0 8px 24px rgb(0 0 0 / 0.16)",
  },
  ".cm-tooltip-autocomplete > ul > li[aria-selected]": {
    backgroundColor: "var(--accent)",
    color: "var(--accent-foreground)",
  },
  ".cm-panels": {
    backgroundColor: "var(--card)",
    color: "var(--card-foreground)",
    borderColor: "var(--border)",
  },
  ".cm-diagnostic-error": { borderLeftColor: "var(--destructive)" },
  ".cm-diagnostic-warning": { borderLeftColor: "var(--warning)" },
});

export const csHighlighting = syntaxHighlighting(HighlightStyle.define([
  { tag: tags.keyword, color: "var(--primary)", fontWeight: "600" },
  { tag: tags.typeName, color: "var(--chart-4)", fontWeight: "600" },
  { tag: tags.string, color: "var(--success-ink)" },
  { tag: tags.number, color: "var(--warning-ink)" },
  { tag: tags.comment, color: "var(--muted-foreground)", fontStyle: "italic" },
  { tag: tags.variableName, color: "var(--foreground)" },
  { tag: tags.bracket, color: "var(--muted-foreground)" },
  { tag: tags.punctuation, color: "var(--muted-foreground)" },
  { tag: tags.invalid, color: "var(--destructive)" },
]));

const topLevelBlocks = [
  "http-get", "http-post", "http-config", "http-stager", "post-ex",
  "process-inject", "stage", "ssl-certificate", "dns-beacon", "forgec2",
];

const topLevelSets = [
  "sleeptime", "jitter", "useragent", "description", "sample_name",
  "host_stage", "dns_idle", "dns_max_txt", "dns_sleep", "dns_ttl",
  "pipename", "ssh_banner", "ssh_pipename", "http_proxy",
];

const httpSets = ["uri", "verb", "parameter", "useragent"];
const configSets = ["headers", "useragent", "trust_x_forwarded_for", "block_invoke", "block_useragent"];
const forgeSets = [
  "prepend", "append", "request_prepend", "request_append",
  "content_length_jitter", "jitter_uri", "jitter_parameter", "parameter",
  "work_start", "work_end", "work_tz", "user_agents", "parameter_names",
  "request_header", "request_header_pool", "placement", "client_id",
  "client_metadata", "server_output", "method", "beacon_uri", "description",
];

const transformCompletions: Completion[] = [
  "base64", "base64url", "netbios", "netbiosu", "mask", "print",
  "append", "prepend", "xor", "urlencode", "uri-append", "header",
  "parameter", "strrep", "case",
].map((label) => ({ label, type: "function", detail: "transform" }));

const setCompletions = (values: string[]): Completion[] =>
  values.map((label) => ({ label, type: "property", detail: "set" }));

function blockPath(state: { doc: { sliceString: (from: number, to: number) => string } }, pos: number): string[] {
  const before = state.doc.sliceString(0, pos);
  const stack: string[] = [];
  for (const raw of before.split("\n")) {
    const line = raw.replace(/#.*$/, "").trim();
    const open = line.match(/^([A-Za-z_][\w-]*)\s*\{/);
    if (open) stack.push(open[1].toLowerCase());
    const closes = (line.match(/\}/g) || []).length;
    for (let i = 0; i < closes && stack.length > 0; i += 1) stack.pop();
  }
  return stack;
}

function csCompletions(context: CompletionContext): CompletionResult | null {
  const word = context.matchBefore(/[\w-]*/);
  if (!word) return null;
  if (word.from === word.to && !context.explicit) return null;
  const line = context.state.doc.lineAt(context.pos);
  const before = line.text.slice(0, context.pos - line.from);
  const path = blockPath(context.state, context.pos);
  const top = path[path.length - 1] || "";

  if (/^\s*set\s+[\w-]*$/.test(before)) {
    let values = topLevelSets;
    if (path.includes("http-get") || path.includes("http-post")) values = httpSets;
    else if (path.includes("http-config")) values = configSets;
    else if (path.includes("forgec2")) values = forgeSets;
    return { from: word.from, options: setCompletions(values) };
  }

  if (top === "client") {
    const options: Completion[] = [
      { label: "header", type: "property", detail: "client header" },
      path.includes("http-post")
        ? { label: "id", type: "keyword", detail: "client id block" }
        : { label: "metadata", type: "keyword", detail: "client metadata block" },
    ];
    return { from: word.from, options };
  }
  if (top === "server") {
    return { from: word.from, options: [{ label: "output", type: "keyword", detail: "server output block" }] };
  }
  if (top === "metadata" || top === "id" || top === "output") {
    return { from: word.from, options: transformCompletions };
  }
  if (top === "http-get" || top === "http-post") {
    return {
      from: word.from,
      options: [
        { label: "client", type: "keyword", detail: "client block" },
        { label: "server", type: "keyword", detail: "server block" },
        ...setCompletions(httpSets),
      ],
    };
  }
  if (top === "http-config") {
    return { from: word.from, options: [{ label: "set", type: "keyword" }, { label: "header", type: "property" }] };
  }
  if (top === "forgec2") {
    return { from: word.from, options: setCompletions(forgeSets) };
  }

  const options: Completion[] = [
    { label: "set", type: "keyword", detail: "profile setting" },
    ...topLevelBlocks.map((label) =>
      snippetCompletion(`${label} {\n  \${}\n}`, {
        label,
        type: "keyword",
        detail: label === "forgec2" ? "ForgeC2 extension" : "CS block",
      })),
  ];
  return { from: word.from, options };
}

export const csAutocompletion = autocompletion({ override: [csCompletions] });

export function starterProfile(name = "new_profile"): string {
  return `# ForgeC2 Malleable C2 profile: ${name}
# Edit freely using Cobalt Strike profile syntax. The forgec2 { ... } block
# preserves ForgeC2-only fields; unsupported CS blocks are kept in source and
# reported as warnings by the validator.

set sleeptime "30000";
set jitter "20";
set useragent "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36";

http-post {
  set uri "/api/v1/beacon";
  set verb "POST";
  set parameter "id";

  client {
    header "Accept" "*/*";
    header "Content-Type" "application/octet-stream";
    id {
      parameter "id";
      base64;
    }
  }

  server {
    output {
      base64;
    }
  }
}

forgec2 {
  set content_length_jitter "256";
}
`;
}
