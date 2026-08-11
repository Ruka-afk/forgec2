import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { ConfirmModal } from "@/components/ui/confirm-modal";

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

describe("ConfirmModal", () => {
  it("confirms immediately when no requireText is set", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmModal open title="Delete" message="Delete this?" confirmText="Delete" onConfirm={onConfirm} onCancel={() => {}} />,
    );
    const btn = screen.getByRole("button", { name: "Delete" });
    expect(btn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("requires exact text before confirming", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmModal
        open
        title="Delete"
        message="Delete this?"
        confirmText="Delete"
        requireText="prod-box"
        onConfirm={onConfirm}
        onCancel={() => {}}
      />,
    );
    const btn = screen.getByRole("button", { name: "Delete" });
    expect(btn.hasAttribute("disabled")).toBe(true);

    const input = screen.getByRole("textbox");
    fireEvent.change(input, { target: { value: "prod-box" } });
    expect(btn.hasAttribute("disabled")).toBe(false);
    fireEvent.click(btn);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("stays disabled on partial match", () => {
    render(
      <ConfirmModal
        open
        title="Delete"
        message="Delete this?"
        confirmText="Delete"
        requireText="prod-box"
        onConfirm={() => {}}
        onCancel={() => {}}
      />,
    );
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "prod" } });
    expect(screen.getByRole("button", { name: "Delete" }).hasAttribute("disabled")).toBe(true);
  });
});