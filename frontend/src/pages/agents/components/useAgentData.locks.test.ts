import { renderHook, waitFor, act } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAgentData } from "./useAgentData";

const { getMock, postJsonMock } = vi.hoisted(() => ({
  getMock: vi.fn(),
  postJsonMock: vi.fn(),
}));

vi.mock("@/lib/api", () => ({
  api: {
    get: (...args: unknown[]) => getMock(...args),
    postJson: (...args: unknown[]) => postJsonMock(...args),
  },
}));

const t = (key: string) => key;

function route(url: string) {
  if (typeof url === "string" && url.startsWith("/collab/agents")) return "locks";
  if (typeof url === "string" && url.includes("/tags")) return "tags";
  return "agents";
}

describe("useAgentData lock state", () => {
  beforeEach(() => {
    getMock.mockReset();
    postJsonMock.mockReset();
    // The list + tags sources are incidental here; only locks are under test.
    getMock.mockImplementation((url: string) => {
      const which = route(url);
      if (which === "locks") return Promise.resolve({ agents: [] });
      if (which === "tags") return Promise.resolve([]);
      return Promise.resolve({ agents: [], total: 0 });
    });
    postJsonMock.mockResolvedValue({ tags: {} });
  });

  it("reports the lock table as unread until a read has actually succeeded", async () => {
    const { result } = renderHook(() => useAgentData(t));
    await waitFor(() => expect(getMock).toHaveBeenCalled());

    // The dangerous default: before any successful read every agent would
    // otherwise render as "unlocked".
    expect(result.current.locksUnread).toBe(true);
    expect(result.current.locksLoaded).toBe(false);
  });

  it("keeps the lock table unread after a failed read instead of showing unlocked", async () => {
    const { result } = renderHook(() => useAgentData(t));
    await waitFor(() => expect(getMock).toHaveBeenCalled());

    getMock.mockImplementation((url: string) => {
      const which = route(url);
      if (which === "locks") return Promise.reject(new Error("collab down"));
      if (which === "tags") return Promise.resolve([]);
      return Promise.resolve({ agents: [], total: 0 });
    });

    await act(async () => {
      result.current.loadLocks();
    });

    expect(result.current.locksError).toBeTruthy();
    expect(result.current.locksUnread).toBe(true);
    expect(result.current.locksLoaded).toBe(false);
  });

  it("marks locks known only after a successful read and keeps holders on later failure", async () => {
    const { result } = renderHook(() => useAgentData(t));
    await waitFor(() => expect(getMock).toHaveBeenCalled());

    getMock.mockImplementation((url: string) => {
      const which = route(url);
      if (which === "locks") {
        return Promise.resolve({ agents: [{ id: "a1", locked_by: "operator-x" }] });
      }
      if (which === "tags") return Promise.resolve([]);
      return Promise.resolve({ agents: [], total: 0 });
    });

    await act(async () => {
      result.current.loadLocks();
    });

    await waitFor(() => expect(result.current.locksUnread).toBe(false));
    expect(result.current.agentLocks.a1).toBe("operator-x");
    expect(result.current.locksError).toBeNull();

    getMock.mockImplementation((url: string) => {
      const which = route(url);
      if (which === "locks") return Promise.reject(new Error("collab down"));
      if (which === "tags") return Promise.resolve([]);
      return Promise.resolve({ agents: [], total: 0 });
    });

    await act(async () => {
      result.current.loadLocks();
    });

    // Stale but real data beats a false "unlocked", and the table is no longer
    // marked unread because it was read successfully at least once.
    expect(result.current.agentLocks.a1).toBe("operator-x");
    expect(result.current.locksUnread).toBe(false);
    expect(result.current.locksError).toBeTruthy();
  });
});
