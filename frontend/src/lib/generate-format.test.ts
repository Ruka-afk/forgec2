import { describe, expect, it } from "vitest";
import { defaultPayloadFormat, isPayloadFormat, PAYLOAD_FORMATS } from "./generate-format";

describe("payload format picker", () => {
  it("defaults to exe and rejects unknown keys", () => {
    expect(defaultPayloadFormat()).toBe("exe");
    expect(defaultPayloadFormat("dll")).toBe("dll");
    expect(defaultPayloadFormat("nope")).toBe("exe");
    expect(isPayloadFormat("oneliner")).toBe(true);
    expect(isPayloadFormat("stager")).toBe(false);
    expect(PAYLOAD_FORMATS).toHaveLength(6);
  });
});
