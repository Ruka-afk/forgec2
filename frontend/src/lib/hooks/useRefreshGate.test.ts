import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useRefreshGate } from "./useRefreshGate";

describe("useRefreshGate", () => {
  it("asks for the loading state until data has arrived once", () => {
    const { result } = renderHook(() => useRefreshGate());

    expect(result.current.beginRefresh()).toBe(true);
    expect(result.current.beginRefresh()).toBe(true);

    act(() => result.current.markLoaded());

    expect(result.current.beginRefresh()).toBe(false);
    expect(result.current.beginRefresh()).toBe(false);
  });

  it("keeps the gate open when a load fails before ever succeeding", () => {
    const { result } = renderHook(() => useRefreshGate());

    // A failed first load must not be mistaken for loaded data.
    expect(result.current.beginRefresh()).toBe(true);
    expect(result.current.beginRefresh()).toBe(true);
  });

  it("always shows progress for a forced, user-initiated reload", () => {
    const { result } = renderHook(() => useRefreshGate());
    act(() => result.current.markLoaded());

    expect(result.current.beginRefresh()).toBe(false);
    expect(result.current.beginRefresh(true)).toBe(true);
  });
});
