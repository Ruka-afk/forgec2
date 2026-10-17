import { describe, expect, it } from "vitest";
import { getCompletions } from "./completions";

describe("getCompletions", () => {
  it("returns the first 20 windows commands for empty input", () => {
    const out = getCompletions("", "windows");
    expect(out).toHaveLength(20);
    expect(out[0]).toBe("whoami");
  });

  it("treats whitespace-only input as empty", () => {
    expect(getCompletions("   ", "windows")).toEqual(getCompletions("", "windows"));
  });

  it("uses the linux catalogue only for linux agents", () => {
    expect(getCompletions("ipconfig", "linux")).toEqual([]);
    expect(getCompletions("ipconfig", "windows")).toEqual(["ipconfig", "ipconfig /all"]);
    expect(getCompletions("id", "linux")).toEqual(["id"]);
    // Any other OS string falls back to the windows catalogue.
    expect(getCompletions("mimikatz", "darwin")).toEqual(["mimikatz"]);
  });

  it("matches prefixes case-insensitively and keeps only matching commands", () => {
    expect(getCompletions("WHO", "windows")).toEqual(["whoami"]);
    expect(getCompletions("get-e", "windows")).toEqual(["Get-ExecutionPolicy"]);
    expect(getCompletions("ls", "linux")).toEqual(["ls -la", "ls -la /tmp"]);
  });

  it("filters the subcommands of the matched base command", () => {
    expect(getCompletions("netstat -a", "windows")).toEqual(["-ano", "-anob"]);
    expect(getCompletions("netstat -p", "windows")).toEqual(["-p tcp", "-p udp"]);
    expect(getCompletions("reg q", "windows")).toEqual(["query"]);
    expect(getCompletions("ipconfig /", "windows")).toEqual(["/all", "/release", "/renew", "/flushdns"]);
    // The base command lookup is case-insensitive too.
    expect(getCompletions("NETSTAT -A", "windows")).toEqual(["-ano", "-anob"]);
  });

  it("joins multi-part prefixes when filtering subcommands", () => {
    expect(getCompletions("curl -x post", "linux")).toEqual(["-X POST"]);
    expect(getCompletions("powershell -c", "windows")).toEqual(["-Command"]);
    expect(getCompletions("powershell -e", "windows")).toEqual(["-EncodedCommand"]);
    expect(getCompletions("powershell -", "windows")).toEqual(["-Command", "-EncodedCommand", "-File", "-NoProfile"]);
  });

  it("falls back to command-prefix matching for bases without subcommands", () => {
    expect(getCompletions("net u", "windows")).toEqual(["net user", "net use"]);
    expect(getCompletions("net user", "windows")).toEqual(["net user"]);
  });

  it("does not leak subcommands into bare one-word prefixes", () => {
    // A single token never enters the subcommand branch, even for bases that have them.
    expect(getCompletions("reg", "windows")).toEqual(["reg query", "reg add", "reg delete"]);
    expect(getCompletions("dir", "windows")).toEqual(["dir"]);
  });

  it("returns an empty list when nothing matches", () => {
    expect(getCompletions("zzz", "windows")).toEqual([]);
    expect(getCompletions("netstat -z", "windows")).toEqual([]);
    expect(getCompletions("net zzz", "windows")).toEqual([]);
  });
});
