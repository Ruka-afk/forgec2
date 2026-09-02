import { describe, expect, it } from "vitest";
import { describeToolOutput, extractAICitations, formatAIRunDuration } from "./aiOutput";

describe("describeToolOutput", () => {
  it("formats JSON and derives record counts", () => {
    const view = describeToolOutput('结果:\n{"agents":[{"id":1},{"id":2}],"ok":true}');
    expect(view.isJson).toBe(true);
    expect(view.itemCount).toBe(2);
    expect(view.status).toBe("success");
    expect(view.formatted).toContain('"agents"');
  });

  it("recognizes structured and textual failures", () => {
    expect(describeToolOutput('{"error":"permission denied"}').status).toBe("error");
    expect(describeToolOutput("command failed").status).toBe("error");
  });

  it("surfaces partial-result metadata from compacted tool output", () => {
    const view = describeToolOutput('{"data":{"agents":[{"id":1}]},"_meta":{"partial":true,"original_bytes":12000}}');
    expect(view.partial).toBe(true);
    expect(view.originalBytes).toBe(12000);
    expect(view.itemCount).toBe(1);
  });
});

describe("formatAIRunDuration", () => {
  it("keeps short and long durations readable", () => {
    expect(formatAIRunDuration(320)).toBe("320ms");
    expect(formatAIRunDuration(2400)).toBe("2.4s");
    expect(formatAIRunDuration(65_000)).toBe("1m 5s");
  });
});

describe("extractAICitations", () => {
  it("extracts unique English and Chinese source markers", () => {
    expect(extractAICitations("a [source: runbook.md#chunk-2] b [来源: notes.txt#attachment] [source: runbook.md#chunk-2]"))
      .toEqual(["runbook.md#chunk-2", "notes.txt#attachment"]);
  });
});
