import { describe, it, expect } from "vitest";
import { formatSize, isImageFile, joinPath, parentPath, parseDrives } from "./types";

describe("agent files helpers", () => {
  it("formatSize handles zero and units", () => {
    expect(formatSize(0)).toBe("-");
    expect(formatSize(500)).toBe("500 B");
    expect(formatSize(2048)).toMatch(/KB/);
  });

  it("isImageFile checks extensions", () => {
    expect(isImageFile("a.png")).toBe(true);
    expect(isImageFile("a.txt")).toBe(false);
  });

  it("joinPath and parentPath respect os type", () => {
    expect(joinPath("C:\\Users", "Public", "windows")).toBe("C:\\Users\\Public");
    expect(joinPath("/home", "user", "linux")).toBe("/home/user");
    expect(parentPath("C:\\Users\\Public\\", "windows")).toBe("C:\\Users\\");
    expect(parentPath("/home/user", "linux")).toBe("/home");
  });

  it("parseDrives normalizes string and object drives", () => {
    expect(parseDrives(["C:"])).toEqual([{ letter: "C:", label: "", total: 0, free: 0 }]);
    expect(parseDrives([{ letter: "D:", total_size: 10, free_space: 2 }])[0]).toMatchObject({
      letter: "D:",
      total: 10,
      free: 2,
    });
  });
});
