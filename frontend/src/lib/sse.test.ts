import { describe, expect, it } from "vitest";
import { consumeSSEBuffer, flushSSEBuffer } from "./sse";

describe("consumeSSEBuffer", () => {
  it("keeps an event split across network chunks", () => {
    const first = consumeSSEBuffer("event: progre");
    expect(first.events).toEqual([]);

    const second = consumeSSEBuffer(first.remainder + "ss\ndata: {\"stage\":\"analyzing\"}\n\n");
    expect(second.events).toEqual([
      { event: "progress", data: '{"stage":"analyzing"}' },
    ]);
    expect(second.remainder).toBe("");
  });

  it("joins multiline data according to the SSE specification", () => {
    const parsed = consumeSSEBuffer("event: text\ndata: first\ndata: second\n\n");
    expect(parsed.events).toEqual([{ event: "text", data: "first\nsecond" }]);
  });

  it("flushes a final frame without a trailing blank line", () => {
    expect(flushSSEBuffer("event: done\ndata: ok")).toEqual([
      { event: "done", data: "ok" },
    ]);
  });
});
