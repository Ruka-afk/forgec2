import { createRef } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AIComposer } from "./AIComposer";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

function renderComposer(overrides: Partial<React.ComponentProps<typeof AIComposer>> = {}) {
  const props: React.ComponentProps<typeof AIComposer> = {
    input: "next question",
    loading: false,
    messageCount: 1,
    maxLength: 16000,
    textareaRef: createRef<HTMLTextAreaElement>(),
    onChange: vi.fn(),
    onKeyDown: vi.fn(),
    onSend: vi.fn(),
    onStop: vi.fn(),
    ...overrides,
  };
  render(<AIComposer {...props} />);
  return props;
}

describe("AIComposer", () => {
  it("keeps the draft editable while a response is streaming", () => {
    const props = renderComposer({ loading: true });
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;

    expect(textarea.disabled).toBe(false);
    expect(textarea.getAttribute("aria-busy")).toBe("true");
    expect(textarea.maxLength).toBe(16000);
    fireEvent.change(textarea, { target: { value: "draft while waiting" } });
    expect(props.onChange).toHaveBeenCalledWith("draft while waiting");
    expect(screen.getByRole("button", { name: "ai.stop_generation" })).toBeTruthy();
  });

  it("still disables input when AI is not configured", () => {
    renderComposer({ disabled: true, loading: false });
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).disabled).toBe(true);
  });
});
