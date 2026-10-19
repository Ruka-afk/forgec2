import { describe, expect, it } from "vitest";
import {
  PERSISTENCE_METHOD_QUALITY,
  persistenceLooksConfirmed,
  persistenceMethodQuality,
  TOKEN_ACTIONS,
  tokenActionGuaranteed,
  tokenActionQuality,
} from "./dest-quality";

describe("token dest honesty", () => {
  it("does not treat steal/impersonate as a guaranteed CS token store", () => {
    expect(tokenActionGuaranteed("steal")).toBe(false);
    expect(tokenActionGuaranteed("impersonate")).toBe(false);
    expect(tokenActionGuaranteed("make")).toBe(false);
    expect(tokenActionQuality("steal")).toBe("hardened");
    expect(tokenActionQuality("impersonate")).toBe("hardened");
    expect(TOKEN_ACTIONS.some((a) => a.quality === "core" || a.guaranteed)).toBe(false);
  });
});

describe("persistence dest honesty", () => {
  it("differentiates registry from COM/DLL hijack and rejects confirmed-install copy", () => {
    expect(persistenceMethodQuality("registry")).toBe("hardened");
    expect(persistenceMethodQuality("startup_folder")).toBe("hardened");
    expect(persistenceMethodQuality("wmi")).toBe("scripted");
    expect(persistenceMethodQuality("com_hijack")).toBe("experimental");
    expect(persistenceMethodQuality("dll_search_order")).toBe("experimental");
    expect(PERSISTENCE_METHOD_QUALITY.some((m) => m.quality === "core")).toBe(false);
    expect(persistenceLooksConfirmed("queued")).toBe(false);
    expect(persistenceLooksConfirmed("installed")).toBe(true);
  });
});
