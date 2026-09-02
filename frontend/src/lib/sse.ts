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

/**
 * Parses only complete SSE frames and returns the unfinished suffix. This is
 * intentionally chunk-boundary agnostic: event names, JSON and blank frame
 * delimiters may all be split across network reads.
 */
export function consumeSSEBuffer(buffer: string): {
  events: ParsedSSEEvent[];
  remainder: string;
} {
  const frames = buffer.split(/\r?\n\r?\n/);
  const remainder = frames.pop() ?? "";
  const events = frames
    .map(parseFrame)
    .filter((event): event is ParsedSSEEvent => event !== null);
  return { events, remainder };
}

/** Parse a final unterminated frame after the response body closes. */
export function flushSSEBuffer(buffer: string): ParsedSSEEvent[] {
  if (buffer.trim() === "") return [];
  const event = parseFrame(buffer);
  return event ? [event] : [];
}
