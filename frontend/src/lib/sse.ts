export interface ParsedSSEEvent {
	id?: string;
  event: string;
  data: string;
}

function parseFrame(frame: string): ParsedSSEEvent | null {
  let event = "message";
	let id: string | undefined;
  const data: string[] = [];

  for (const rawLine of frame.split(/\r?\n/)) {
    if (rawLine === "" || rawLine.startsWith(":")) continue;
    const separator = rawLine.indexOf(":");
    const field = separator === -1 ? rawLine : rawLine.slice(0, separator);
    let value = separator === -1 ? "" : rawLine.slice(separator + 1);
    if (value.startsWith(" ")) value = value.slice(1);

    if (field === "event") event = value;
	if (field === "id") id = value;
    if (field === "data") data.push(value);
  }

	if (data.length === 0) return null;
	const parsed: ParsedSSEEvent = { event, data: data.join("\n") };
	if (id !== undefined) parsed.id = id;
	return parsed;
}

export const SSE_MAX_BUFFER_BYTES = 2 * 1024 * 1024;

/**
 * Parses only complete SSE frames and returns the unfinished suffix. This is
 * intentionally chunk-boundary agnostic: event names, JSON and blank frame
 * delimiters may all be split across network reads.
 * O(n) regex scan with 2 MB cap to prevent unbounded growth.
 */
export function consumeSSEBuffer(buffer: string): {
  events: ParsedSSEEvent[];
  remainder: string;
  overflow: boolean;
} {
  let overflow = false;
  if (buffer.length > SSE_MAX_BUFFER_BYTES) {
    buffer = buffer.slice(buffer.length - SSE_MAX_BUFFER_BYTES);
    overflow = true;
  }
  const events: ParsedSSEEvent[] = [];
  const re = /\r?\n\r?\n/g;
  let match: RegExpExecArray | null;
  let lastIndex = 0;
  while ((match = re.exec(buffer)) !== null) {
    const frame = buffer.slice(lastIndex, match.index);
    const parsed = parseFrame(frame);
    if (parsed) events.push(parsed);
    lastIndex = match.index + match[0].length;
    if (events.length > 5000) break;
  }
  return { events, remainder: buffer.slice(lastIndex), overflow };
}

/** Parse a final unterminated frame after the response body closes. */
export function flushSSEBuffer(buffer: string): ParsedSSEEvent[] {
  if (buffer.trim() === "") return [];
  const event = parseFrame(buffer);
  return event ? [event] : [];
}
