import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { BuildResult, BuildSignatureBadge } from "./BuildResult";
import type { BuildSignature } from "@/types/generate";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string, vars?: { name?: string }) => (vars?.name ? `${key}:${vars.name}` : key),
  }),
}));

const signature = (status: BuildSignature["status"], signer = "CN=Test", issuer = "CN=Test"): BuildSignature => ({
  status,
  signer,
  issuer,
});

describe("BuildSignatureBadge", () => {
  it("labels a valid trusted signature as valid", () => {
    render(<BuildSignatureBadge signature={signature("valid", "CN=Signer", "CN=Root")} />);
    expect(screen.getByText("generate.signature.valid")).toBeTruthy();
    expect(screen.getByText("generate.signature.signer:CN=Signer")).toBeTruthy();
  });

  it("does not label a self-signed signature as valid", () => {
    render(<BuildSignatureBadge signature={signature("untrusted_root")} />);
    expect(screen.getByText("generate.signature.self_signed")).toBeTruthy();
    expect(screen.queryByText("generate.signature.valid")).toBeNull();
  });

  it("distinguishes an untrusted non-self-signed root", () => {
    render(<BuildSignatureBadge signature={signature("untrusted_root", "CN=Signer", "CN=Private CA")} />);
    expect(screen.getByText("generate.signature.untrusted_root")).toBeTruthy();
  });

  it("renders invalid and unsigned states", () => {
    const { rerender } = render(<BuildSignatureBadge signature={signature("invalid")} />);
    expect(screen.getByText("generate.signature.invalid")).toBeTruthy();
    rerender(<BuildSignatureBadge signature={signature("unsigned", "", "")} />);
    expect(screen.getByText("generate.signature.unsigned")).toBeTruthy();
  });
});

describe("BuildResult", () => {
  it("renders the signature even when a successful build has no result text", () => {
    render(<BuildResult busy={false} result="" signature={signature("valid")} />);
    expect(screen.getByText("generate.signature.valid")).toBeTruthy();
  });

  it("stays empty when there is neither a result nor a signature", () => {
    const { container } = render(<BuildResult busy={false} result="" />);
    expect(container.textContent).toBe("");
  });
});
