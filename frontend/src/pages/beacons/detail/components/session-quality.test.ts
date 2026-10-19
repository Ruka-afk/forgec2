import { describe, expect, it } from "vitest";
import {
  isExperimentalDesktop,
  isGuaranteedElevate,
  sessionActionQuality,
} from "./session-quality";

describe("sessionActionQuality", () => {
  it("does not present privesc_check as Core elevation or RDP as Core", () => {
    expect(sessionActionQuality("privesc_check")).toBe("hardened");
    expect(isGuaranteedElevate("privesc_check")).toBe(false);
    expect(isGuaranteedElevate("elevate")).toBe(true);
    expect(sessionActionQuality("screenshot")).toBe("hardened");
    expect(sessionActionQuality("keylogger_start")).toBe("hardened");
    expect(sessionActionQuality("keylogger_dump")).toBe("hardened");
    expect(sessionActionQuality("remote-desktop")).toBe("experimental");
    expect(isExperimentalDesktop("remote-desktop")).toBe(true);
    expect(isExperimentalDesktop("shell")).toBe(false);
    expect(sessionActionQuality("shell")).toBe("core");
  });
});
