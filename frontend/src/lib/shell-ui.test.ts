import { describe, expect, it } from "vitest";
import {
  agentIdentityTitle,
  defaultInterpreter,
  decodeShellWhitespace,
  interpreterOptions,
  operatorErrorText,
  pickAgentField,
  quickCommands,
  sessionPromptLabel,
  truncateUploadDisplay,
} from "./shell-ui";

describe("shell-ui", () => {
  it("picks interpreters and quick commands by OS", () => {
    expect(defaultInterpreter("windows")).toBe("cmd.exe");
    expect(defaultInterpreter("linux")).toBe("/bin/sh");
    expect(interpreterOptions("windows")).toContain("powershell.exe");
    expect(interpreterOptions("linux")).toContain("/bin/bash");
    expect(quickCommands("windows")[0]).toBe("whoami");
    expect(quickCommands("linux")).toContain("uname -a");
  });

  it("builds an operator prompt from host/user/interpreter", () => {
    expect(sessionPromptLabel({ osType: "windows", hostname: "PC", username: "alice" })).toBe("PC\\alice>");
    expect(sessionPromptLabel({ osType: "windows", hostname: "PC", interpreter: "powershell.exe" })).toBe("PS PC>");
    expect(sessionPromptLabel({ osType: "linux", hostname: "box", username: "root" })).toBe("root@box$");
    expect(sessionPromptLabel({ osType: "windows" })).toBe("agent>");
  });

  it("formats identity and agent fields", () => {
    expect(agentIdentityTitle("HOST", "bob", "uuid")).toBe("HOST · bob");
    expect(agentIdentityTitle("", "", "47fe7bd2-aaaa")).toBe("47fe7bd2");
    expect(pickAgentField({ Hostname: "X", hostname: "y" }, "hostname", "Hostname")).toBe("y");
    expect(pickAgentField({ Hostname: "X" }, "hostname", "Hostname")).toBe("X");
  });

  it("hides at-rest ciphertext from the operator", () => {
    expect(operatorErrorText("FC2ENC:abcd", "failed")).toBe("failed");
    expect(operatorErrorText("access denied", "failed")).toBe("access denied");
    expect(operatorErrorText(undefined, "failed")).toBe("failed");
  });

  it("restores encoded whitespace without decoding arbitrary HTML", () => {
    expect(decodeShellWhitespace("A&#x20;B&#32;C&nbsp;D")).toBe("A B C D");
    expect(decodeShellWhitespace("&lt;script&gt;&#65;")).toBe("&lt;script&gt;&#65;");
  });

  it("truncates inline upload display", () => {
    const cmd = `upload note.txt ${"A".repeat(400)}`;
    expect(truncateUploadDisplay(cmd)).toBe("upload note.txt");
    expect(truncateUploadDisplay("whoami")).toBe("whoami");
  });
});
