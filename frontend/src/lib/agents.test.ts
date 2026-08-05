import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { AGENTS_LIST_PATH, normalizeAgentList, fetchAgentList } from "./agents";
import { api } from "./api";

describe("AGENTS_LIST_PATH", () => {
  it("points at the API-prefixed path", () => {
    expect(AGENTS_LIST_PATH).toBe("/api/agents");
  });
});

describe("normalizeAgentList", () => {
  it("returns bare arrays as-is", () => {
    const list = [{ id: "a1" }, { id: "a2" }];
    expect(normalizeAgentList(list)).toEqual(list);
  });

  it("unwraps agents key", () => {
    const agents = [{ id: "x" }];
    expect(normalizeAgentList({ agents })).toEqual(agents);
  });

  it("unwraps Agents key", () => {
    const Agents = [{ id: "y" }];
    expect(normalizeAgentList({ Agents })).toEqual(Agents);
  });

  it("unwraps data key", () => {
    const data = [{ id: "z" }];
    expect(normalizeAgentList({ data })).toEqual(data);
  });

  it("returns empty array for null/undefined/non-object", () => {
    expect(normalizeAgentList(null)).toEqual([]);
    expect(normalizeAgentList(undefined)).toEqual([]);
    expect(normalizeAgentList("nope")).toEqual([]);
    expect(normalizeAgentList(42)).toEqual([]);
  });

  it("returns empty array for object without list keys", () => {
    expect(normalizeAgentList({ total: 0 })).toEqual([]);
  });
});

describe("fetchAgentList", () => {
  beforeEach(() => {
    vi.spyOn(api, "get");
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns agents on success", async () => {
    vi.mocked(api.get).mockResolvedValue({
      agents: [{ id: "1", hostname: "h1", status: "online", ip: "1.1.1.1" }],
    });
    const r = await fetchAgentList();
    expect(r.error).toBeNull();
    expect(r.agents).toHaveLength(1);
    expect(r.agents[0].hostname).toBe("h1");
  });

  it("returns error instead of silent empty on failure", async () => {
    vi.mocked(api.get).mockRejectedValue(new Error("unauthorized"));
    const r = await fetchAgentList();
    expect(r.agents).toEqual([]);
    expect(r.error).toBe("unauthorized");
  });

  it("filters onlineOnly", async () => {
    vi.mocked(api.get).mockResolvedValue({
      agents: [
        { id: "1", status: "online" },
        { id: "2", status: "offline" },
      ],
    });
    const r = await fetchAgentList(true);
    expect(r.agents.map((a) => a.id)).toEqual(["1"]);
  });
});
