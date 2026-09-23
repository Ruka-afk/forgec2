import { describe, it, expect, beforeEach } from "vitest";
import { renderToString } from "react-dom/server";
import {
  registerTaskRenderer,
  getTaskRenderer,
  registeredRendererTypes,
  HostInfoResultView,
  PsResultView,
  NetstatResultView,
  CredsResultView,
  parseNetstatRows,
  parseCredRows,
} from "./taskRenderers";

describe("taskRenderers registry", () => {
  beforeEach(() => {
    // Registry is module-global; clear probes between tests via overwrite.
  });

  it("registers and resolves renderers by task type", () => {
    const Probe = () => null;
    registerTaskRenderer("zz-probe", Probe);
    expect(getTaskRenderer("zz-probe")).toBe(Probe);
    expect(registeredRendererTypes()).toContain("zz-probe");
  });

  it("returns undefined for unregistered types (text fallback contract)", () => {
    expect(getTaskRenderer("no-such-type")).toBeUndefined();
  });

  it("later registrations override earlier ones", () => {
    const A = () => null;
    const B = () => null;
    registerTaskRenderer("zz-override", A);
    registerTaskRenderer("zz-override", B);
    expect(getTaskRenderer("zz-override")).toBe(B);
    registerTaskRenderer("zz-override", A); // restore for other assertions
  });
});

describe("HostInfoResultView", () => {
  const good = JSON.stringify({
    platform: "windows",
    collected_at: "2026-01-01T00:00:00Z",
    sections: {
      security: {
        av_products: [{ name: "Defender", protection: "enabled", signatures: "up_to_date", state_hex: "0x061100" }],
        edr_processes: [],
      },
      system: { username: "alice", integrity: "High" },
    },
  });

  it("renders structured sections with AV state badges", () => {
    const html = renderToString(<HostInfoResultView result={good} taskType="hostinfo" />);
    expect(html).toContain("hostinfo-renderer");
    expect(html).toContain("Defender");
    expect(html).toContain("enabled");
    expect(html).toContain("security");
  });

  it("falls back to raw text when the payload is not hostinfo JSON", () => {
    const html = renderToString(
      <HostInfoResultView result="plain shell output" taskType="hostinfo" />,
    );
    expect(html).toContain("<pre");
    expect(html).toContain("plain shell output");
  });
});

describe("PsResultView", () => {
  const sample = [
    "ProcessId Name         WorkingSetMB",
    "--------- ----         ------------",
    "1234      chrome.exe   512.5",
    "56        svchost.exe  100",
  ].join("\n");

  it("renders a process table from Format-Table output", () => {
    const html = renderToString(<PsResultView result={sample} taskType="ps" />);
    expect(html).toContain("ps-renderer");
    expect(html).toContain("chrome.exe");
    expect(html).toContain("1234");
  });

  it("falls back to raw when no rows parse", () => {
    const html = renderToString(<PsResultView result="" taskType="ps" />);
    expect(html).toContain("<pre");
  });
});

describe("parseNetstatRows / NetstatResultView", () => {
  const windows = [
    "  Proto  Local Address          Foreign Address        State           PID",
    "  TCP    0.0.0.0:135            0.0.0.0:0              LISTENING       1234",
    "  TCP    10.0.0.5:49152         93.184.216.34:443      ESTABLISHED     5678",
  ].join("\n");

  it("parses Windows netstat lines", () => {
    const rows = parseNetstatRows(windows);
    expect(rows.length).toBeGreaterThanOrEqual(2);
    expect(rows[0].proto).toBe("tcp");
    expect(rows[0].local).toContain("135");
    expect(rows[0].state.toUpperCase()).toContain("LISTEN");
  });

  it("renders connection table", () => {
    const html = renderToString(<NetstatResultView result={windows} taskType="netstat" />);
    expect(html).toContain("netstat-renderer");
    expect(html).toContain("LISTENING");
  });

  it("falls back when nothing parses", () => {
    const html = renderToString(<NetstatResultView result="not netstat" taskType="netstat" />);
    expect(html).toContain("<pre");
  });
});

describe("parseCredRows / CredsResultView", () => {
  it("parses simple domain\\user:password lines", () => {
    const rows = parseCredRows("CORP\\admin:P@ssw0rd\nuser2:secret123");
    expect(rows.length).toBeGreaterThanOrEqual(2);
    expect(rows[0].username).toBeTruthy();
    expect(rows[0].kind).toBe("password");
  });

  it("parses SAM hash lines", () => {
    const rows = parseCredRows("Administrator:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::");
    expect(rows).toHaveLength(1);
    expect(rows[0].kind).toBe("hash");
    expect(rows[0].secret).toBe("31d6cfe0d16ae931b73c59d7e0c089c0");
  });

  it("masks secrets in the table view", () => {
    const html = renderToString(
      <CredsResultView result="CORP\\admin:P@ssw0rd!" taskType="creds" />,
    );
    expect(html).toContain("creds-renderer");
    expect(html).not.toContain("P@ssw0rd!");
    expect(html).toContain("admin");
  });

  it("falls back when no credentials parse", () => {
    const html = renderToString(<CredsResultView result="no creds here" taskType="creds" />);
    expect(html).toContain("<pre");
  });
});
