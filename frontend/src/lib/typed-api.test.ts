import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { fetchAgentsPage } from "./typed-api";
import { api } from "./api";
import { paths } from "./api-paths";

describe("fetchAgentsPage", () => {
  beforeEach(() => {
    vi.spyOn(api, "get");
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("returns agents + total from the data envelope", async () => {
    vi.mocked(api.get).mockResolvedValue({
      success: true,
      data: [{ id: "a1", hostname: "h1" }],
      total: 1,
    });
    const r = await fetchAgentsPage();
    expect(r.agents).toHaveLength(1);
    expect(r.agents[0].hostname).toBe("h1");
    expect(r.total).toBe(1);
    expect(api.get).toHaveBeenCalledWith(paths.agents.list(""), { unwrap: false });
  });

  it("falls back to the agents key and coerces string totals", async () => {
    vi.mocked(api.get).mockResolvedValue({ agents: [{ id: "a2" }], total: "42" });
    const r = await fetchAgentsPage();
    expect(r.agents).toHaveLength(1);
    expect(r.total).toBe(42);
  });

  it("falls back to list length when total is absent", async () => {
    vi.mocked(api.get).mockResolvedValue({ data: [{ id: "a1" }, { id: "a2" }] });
    const r = await fetchAgentsPage();
    expect(r.agents).toHaveLength(2);
    expect(r.total).toBe(2);
  });

  it("serializes only present query fields", async () => {
    vi.mocked(api.get).mockResolvedValue({ data: [], total: 0 });
    await fetchAgentsPage({
      page: 2,
      page_size: 50,
      status: "online",
      sort_key: "hostname",
      sort_dir: "asc",
      search: "",
      os: undefined,
    });
    const [callPath] = vi.mocked(api.get).mock.calls[0] as [string];
    expect(callPath).toBe(
      paths.agents.list("?page=2&page_size=50&status=online&sort_key=hostname&sort_dir=asc"),
    );
  });

  it("returns empty list when the envelope has no list keys", async () => {
    vi.mocked(api.get).mockResolvedValue({ success: false, error: "boom" });
    const r = await fetchAgentsPage();
    expect(r.agents).toEqual([]);
    expect(r.total).toBe(0);
  });
});