import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useFilesView } from "./useFilesView";
import type { FileEntry } from "./types";

const entries: FileEntry[] = [
  { name: "b.txt", is_dir: false, size: 200, mod_time: "2024-01-02" },
  { name: "adir", is_dir: true, size: 0, mod_time: "2024-01-03" },
  { name: "a.txt", is_dir: false, size: 100, mod_time: "2024-01-01" },
];

describe("useFilesView", () => {
  it("sorts dirs first, then by name ascending by default", () => {
    const { result } = renderHook(() => useFilesView(entries, "C:\\"));
    expect(result.current.sortedEntries.map((e) => e.name)).toEqual(["adir", "a.txt", "b.txt"]);
  });

  it("toggles sort key and direction", () => {
    const { result } = renderHook(() => useFilesView(entries, "C:\\"));
    act(() => {
      result.current.toggleSort("size");
    });
    expect(result.current.sortedEntries.map((e) => e.name)).toEqual(["adir", "a.txt", "b.txt"]);
    act(() => {
      result.current.toggleSort("size");
    });
    expect(result.current.sortedEntries.map((e) => e.name)).toEqual(["adir", "b.txt", "a.txt"]);
  });

  it("toggles selection and clears it on navigate", () => {
    const { result, rerender } = renderHook(({ path }) => useFilesView(entries, path), {
      initialProps: { path: "C:\\" },
    });
    act(() => {
      result.current.toggleSelect("a.txt");
    });
    expect(result.current.selected.has("a.txt")).toBe(true);
    rerender({ path: "C:\\sub" });
    expect(result.current.selected.size).toBe(0);
  });
});
