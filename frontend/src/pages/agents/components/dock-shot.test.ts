import { describe, expect, it } from "vitest";
import { screenshotDataUrl, shouldRefreshDockShot } from "./dock-shot";

describe("screenshotDataUrl", () => {
  it("reads the data-url from the JSON envelope", () => {
    expect(screenshotDataUrl({ image: "data:image/png;base64,aaa" })).toBe("data:image/png;base64,aaa");
    expect(screenshotDataUrl({ data: { image: "data:image/png;base64,bbb" } })).toBe("data:image/png;base64,bbb");
    expect(screenshotDataUrl({ image: "not-an-image" })).toBe("");
    expect(screenshotDataUrl(null)).toBe("");
  });
});

describe("shouldRefreshDockShot", () => {
  it("refreshes only on a completed screenshot task", () => {
    expect(shouldRefreshDockShot({ type: "task_update", task_type: "screenshot", status: "completed" })).toBe(true);
    expect(shouldRefreshDockShot({ type: "task_update", task_type: "screenshot", status: "pending" })).toBe(false);
    expect(shouldRefreshDockShot({ type: "task_update", task_type: "ps", status: "completed" })).toBe(false);
  });
});
