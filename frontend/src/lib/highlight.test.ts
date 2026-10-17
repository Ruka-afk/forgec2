import { describe, it, expect } from "vitest";
import { highlightOutput } from "./highlight";

const CYAN = "\x1b[36m";
const GREEN = "\x1b[32m";
const YELLOW = "\x1b[33m";
const BLUE = "\x1b[34m";
const MAGENTA = "\x1b[35m";
const RED = "\x1b[31m";
const RESET = "\x1b[0m";

describe("highlightOutput", () => {
  it("passes empty, whitespace and unremarkable text through unchanged", () => {
    expect(highlightOutput("")).toBe("");
    expect(highlightOutput("   ")).toBe("   ");
    expect(highlightOutput("plain text")).toBe("plain text");
  });

  it("highlights JSON keys, strings, numbers and literals", () => {
    const out = highlightOutput('{"user": "admin", "port": 8080, "ok": true}');

    expect(out).toContain(`${CYAN}"user"${RESET}:`);
    expect(out).toContain(`: ${GREEN}"admin"${RESET}`);
    expect(out).toContain(`${YELLOW}8080${RESET}`);
    expect(out).toContain(`${MAGENTA}true${RESET}`);
  });

  it("does not touch braces that only look like JSON", () => {
    expect(highlightOutput("{oops")).toBe("{oops");
  });

  it("highlights XML tags and attributes", () => {
    const out = highlightOutput('<agent id="7"/>');

    // The attribute wrap lands inside the tag wrap, so assert the two runs.
    expect(out).toContain(`${CYAN}<agent id`);
    expect(out).toContain(`${GREEN}="7"${RESET}`);
  });

  it("highlights table output when the command marks it as such", () => {
    const listen = highlightOutput("State: LISTENING", "netstat");
    expect(listen).toContain(`${GREEN}LISTENING${RESET}`);

    const pid = highlightOutput("Image Name   PID: 4242", "tasklist");
    expect(pid).toContain(`${MAGENTA}PID: 4242${RESET}`);
  });

  it("highlights failure output in red", () => {
    const out = highlightOutput("upload failed: disk full");
    expect(out).toContain(`${RED}failed${RESET}`);
  });

  it("highlights IPs, paths and success words in plain output", () => {
    expect(highlightOutput("ping 10.0.0.5")).toContain(`${YELLOW}10.0.0.5${RESET}`);
    expect(highlightOutput("wrote /var/log/app.log")).toContain(`${BLUE}/var/log/app.log${RESET}`);
    expect(highlightOutput("transfer OK")).toContain(`${GREEN}OK${RESET}`);
  });
});
