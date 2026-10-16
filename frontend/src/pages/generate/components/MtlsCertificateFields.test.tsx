import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MtlsCertificateFields } from "./MtlsCertificateFields";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

describe("MtlsCertificateFields", () => {
  it("renders the current PEM values", () => {
    render(<MtlsCertificateFields value={{ cert: "CERT", key: "KEY", ca: "CA" }} onChange={() => {}} />);

    expect((screen.getByLabelText("generate.mtls_cert") as HTMLTextAreaElement).value).toBe("CERT");
    expect((screen.getByLabelText("generate.mtls_key") as HTMLTextAreaElement).value).toBe("KEY");
    expect((screen.getByLabelText("generate.mtls_ca") as HTMLTextAreaElement).value).toBe("CA");
  });

  it("reports edits with the other fields preserved", () => {
    const onChange = vi.fn();
    render(<MtlsCertificateFields value={{ cert: "CERT", key: "", ca: "CA" }} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText("generate.mtls_key"), { target: { value: "NEW-KEY" } });

    expect(onChange).toHaveBeenCalledWith({ cert: "CERT", key: "NEW-KEY", ca: "CA" });
  });

  it("warns while the certificate/key pair is incomplete", () => {
    const { rerender } = render(<MtlsCertificateFields value={{ cert: "CERT", key: "", ca: "" }} onChange={() => {}} />);

    expect(screen.getByText("generate.mtls_pair_required")).toBeTruthy();

    rerender(<MtlsCertificateFields value={{ cert: "CERT", key: "KEY", ca: "" }} onChange={() => {}} />);
    expect(screen.queryByText("generate.mtls_pair_required")).toBeNull();
  });
});
