import { describe, expect, it } from "vitest";
import {
  buildFileScanForm,
  FILE_SCAN_DEFAULT_RULES,
  parseFileScanResult,
} from "./file-scan";

describe("file-scan", () => {
  it("builds bounded form fields with safe defaults", () => {
    expect(
      buildFileScanForm({
        root: "/home/alice",
        pattern: " ",
        rules: FILE_SCAN_DEFAULT_RULES,
      }),
    ).toEqual({
      path: "/home/alice",
      pattern: "*",
      rules: "email,phone,bank_card",
      include_hidden: "0",
      max_depth: "8",
      max_files: "200",
      max_file_bytes: String(1024 * 1024),
      max_total_bytes: String(32 * 1024 * 1024),
      max_findings: "500",
      timeout_seconds: "30",
    });
  });

  it("parses raw unmasked findings and summary counters", () => {
    const parsed = parseFileScanResult(
      JSON.stringify({
        root: "C:\\Users\\alice",
        rules: ["email", "bank_card"],
        files_scanned: 12,
        files_skipped: 3,
        skipped_binary: 2,
        skipped_large: 1,
        bytes_scanned: 4096,
        findings: [
          {
            path: "C:\\Users\\alice\\notes.txt",
            name: "notes.txt",
            rule: "email",
            value: "alice@example.com",
            line: 4,
            occurrences: 2,
          },
          {
            path: "C:\\Users\\alice\\card.csv",
            rule: "bank_card",
            value: "4111 1111 1111 1111",
            line: 1,
            occurrences: 1,
          },
        ],
        counts: { email: 1, bank_card: 1 },
        errors: 1,
        truncated: true,
        truncated_reason: "max_findings",
        elapsed_ms: 44,
      }),
    );
    expect(parsed).not.toBeNull();
    expect(parsed?.findings[0].value).toBe("alice@example.com");
    expect(parsed?.findings[1].name).toBe("card.csv");
    expect(parsed?.filesScanned).toBe(12);
    expect(parsed?.skippedBinary).toBe(2);
    expect(parsed?.bytesScanned).toBe(4096);
    expect(parsed?.counts).toEqual({ email: 1, bank_card: 1 });
    expect(parsed?.truncated).toBe(true);
    expect(parsed?.truncatedReason).toBe("max_findings");
    expect(parsed?.elapsedMs).toBe(44);
  });

  it("rejects ACKs and malformed scan documents", () => {
    expect(parseFileScanResult("")).toBeNull();
    expect(parseFileScanResult("not json")).toBeNull();
    expect(parseFileScanResult(`{"success":true}`)).toBeNull();
  });
});
