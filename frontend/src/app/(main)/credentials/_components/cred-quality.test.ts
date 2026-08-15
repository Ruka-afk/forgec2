import { describe, expect, it } from "vitest";
import {
  credActionAllowed,
  credActionBlockReason,
  credActionEndpoint,
  CRED_HARVEST_ACTIONS,
  hasMimikatzModule,
  moduleLooksLikeMimikatz,
  parseModuleNames,
} from "./cred-quality";

describe("hasMimikatzModule", () => {
  it("accepts the same filenames the server attaches", () => {
    expect(moduleLooksLikeMimikatz("Invoke-Mimikatz.ps1")).toBe(true);
    expect(moduleLooksLikeMimikatz("mimikatz.ps1")).toBe(true);
    expect(moduleLooksLikeMimikatz("Invoke-Something.ps1")).toBe(false);
    expect(hasMimikatzModule([{ name: "Invoke-Mimikatz.ps1" }])).toBe(true);
    expect(hasMimikatzModule(["readme.txt"])).toBe(false);
  });
});

describe("parseModuleNames", () => {
  it("unwraps the /api/modules envelope", () => {
    expect(parseModuleNames({ modules: [{ name: "Invoke-Mimikatz.ps1", size: 1 }] })).toEqual(["Invoke-Mimikatz.ps1"]);
    expect(parseModuleNames({ data: ["mimikatz.ps1"] })).toEqual(["mimikatz.ps1"]);
  });
});

describe("credActionAllowed", () => {
  it("does not treat scripted dumps as Core and blocks mimikatz without a module", () => {
    expect(credActionAllowed("creds", false)).toBe(true);
    expect(credActionAllowed("hashdump", false)).toBe(true);
    expect(credActionAllowed("wifi_creds", false)).toBe(true);
    expect(credActionAllowed("mimikatz", false)).toBe(false);
    expect(credActionAllowed("dcsync", false)).toBe(false);
    expect(credActionAllowed("mimikatz", true)).toBe(true);
    expect(credActionBlockReason("mimikatz", false)).toBe("missing_module");
    expect(credActionBlockReason("creds", false)).toBeNull();
    expect(credActionEndpoint("creds_dump")).toBe("creds");
    expect(CRED_HARVEST_ACTIONS.some((a) => a.quality === "core")).toBe(false);
    expect(CRED_HARVEST_ACTIONS.find((a) => a.action === "mimikatz")?.requiresMimikatzModule).toBe(true);
  });
});
