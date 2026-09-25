import { beforeEach, describe, expect, it, vi } from "vitest";

const { getMock } = vi.hoisted(() => ({ getMock: vi.fn() }));

vi.mock("@/lib/api", () => ({
  api: { get: (...args: unknown[]) => getMock(...args) },
}));

async function loadModule() {
  vi.resetModules();
  return import("./ai-status");
}

describe("fetchAIStatus", () => {
  beforeEach(() => {
    getMock.mockReset();
  });

  it("reports enabled + key from a successful read", async () => {
    const { fetchAIStatus } = await loadModule();
    getMock.mockResolvedValueOnce({ enabled: true, has_api_key: true });

    const st = await fetchAIStatus();
    expect(st).toMatchObject({ enabled: true, hasApiKey: true, known: true });
  });

  it("never caches a failed read as 'disabled'", async () => {
    const { fetchAIStatus } = await loadModule();
    getMock.mockRejectedValueOnce(new Error("ai status endpoint down"));

    const failed = await fetchAIStatus();
    expect(failed.known).toBe(false);
    expect(failed.enabled).toBe(false);

    // The poison entry must not survive: the next caller re-reads the server.
    getMock.mockResolvedValueOnce({ enabled: true, has_api_key: true });
    const recovered = await fetchAIStatus();
    expect(recovered).toMatchObject({ enabled: true, hasApiKey: true, known: true });
    expect(getMock).toHaveBeenCalledTimes(2);
  });

  it("caches a successful read within the TTL", async () => {
    const { fetchAIStatus } = await loadModule();
    getMock.mockResolvedValue({ enabled: false, has_api_key: true });

    const first = await fetchAIStatus();
    const second = await fetchAIStatus();

    expect(first.known).toBe(true);
    expect(second.known).toBe(true);
    // enabled:false here is a real answer (AI switched off), so it is cached.
    expect(second.enabled).toBe(false);
    expect(getMock).toHaveBeenCalledTimes(1);
  });

  it("invalidateAIStatus forces an immediate re-read", async () => {
    const { fetchAIStatus, invalidateAIStatus } = await loadModule();
    getMock.mockResolvedValue({ enabled: true, has_api_key: true });

    await fetchAIStatus();
    invalidateAIStatus();
    await fetchAIStatus();

    expect(getMock).toHaveBeenCalledTimes(2);
  });
});
