import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { useConfirm } from "./useConfirm";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        "common.confirm": "Confirm",
        "common.cancel": "Cancel",
        "common.type_to_confirm": 'Type "{text}" to confirm',
      };
      return map[key] || key;
    },
  }),
}));

function Harness({ onResult }: { onResult: (v: boolean) => void }) {
  const { confirm, modal } = useConfirm();
  return (
    <div>
      <button onClick={() => { confirm({ message: "Proceed?", danger: true }).then(onResult); }}>
        open
      </button>
      {modal}
    </div>
  );
}

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn(), refresh: vi.fn(), prefetch: vi.fn() }) }));

describe("useConfirm", () => {
  it("resolves true when confirmed", async () => {
    const onResult = vi.fn();
    render(<Harness onResult={onResult} />);
    fireEvent.click(screen.getByText("open"));
    expect(await screen.findByText("Proceed?")).toBeDefined();
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Confirm" }));
    });
    expect(onResult).toHaveBeenCalledWith(true);
  });

  it("resolves false when cancelled", async () => {
    const onResult = vi.fn();
    render(<Harness onResult={onResult} />);
    fireEvent.click(screen.getByText("open"));
    await screen.findByText("Proceed?");
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    });
    expect(onResult).toHaveBeenCalledWith(false);
  });

  it("renders nothing before a confirm is requested", () => {
    render(<Harness onResult={vi.fn()} />);
    expect(screen.queryByText("Proceed?")).toBeNull();
  });

  it("requires typed text to match requireText before enabling confirm", async () => {
    const onResult = vi.fn();
    function TypedHarness() {
      const { confirm, modal } = useConfirm();
      return (
        <div>
          <button onClick={() => { confirm({ message: "Type to confirm", requireText: "delete" }).then(onResult); }}>
            open
          </button>
          {modal}
        </div>
      );
    }
    render(<TypedHarness />);
    fireEvent.click(screen.getByText("open"));
    await screen.findByText("Type to confirm");
    const confirmBtn = screen.getByRole("button", { name: "Confirm" }) as HTMLButtonElement;
    expect(confirmBtn.disabled).toBe(true);
    await act(async () => {
      fireEvent.change(screen.getByLabelText(/to confirm/i), { target: { value: "delete" } });
    });
    expect(confirmBtn.disabled).toBe(false);
    await act(async () => {
      fireEvent.click(confirmBtn);
    });
    expect(onResult).toHaveBeenCalledWith(true);
  });
});
