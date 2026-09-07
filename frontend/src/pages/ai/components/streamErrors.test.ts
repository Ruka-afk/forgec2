import { describe, expect, it, vi } from "vitest";
import { readAIResponseError } from "./streamErrors";

vi.mock("@/lib/api", () => ({ handleUnauthorized: vi.fn() }));

describe("readAIResponseError", () => {
  it("returns a structured JSON error", async () => {
    const response = new Response(JSON.stringify({ error: "AI is disabled" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
    await expect(readAIResponseError(response)).resolves.toBe("AI is disabled");
  });

  it("does not expose an HTML proxy body", async () => {
    const response = new Response("<html>gateway details</html>", {
      status: 502,
      headers: { "content-type": "text/html" },
    });
    await expect(readAIResponseError(response)).resolves.toBeNull();
  });
});
