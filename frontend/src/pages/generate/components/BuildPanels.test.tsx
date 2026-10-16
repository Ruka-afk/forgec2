import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { DEFAULT_BINARY_FORM } from "@/types/generate";
import { BinaryPanel } from "./BuildPanels";

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({ t: (key: string) => key }),
}));

function renderBinaryPanel() {
  return render(
    <TooltipProvider delay={0}>
      <BinaryPanel
        variant="exe"
        form={DEFAULT_BINARY_FORM}
        setForm={() => {}}
        busy={false}
        result={null}
        onGenerate={() => {}}
      />
    </TooltipProvider>,
  );
}

describe("BinaryPanel ghost mode", () => {
  it("keeps the ghost mode explanation in a hover tooltip", async () => {
    renderBinaryPanel();

    // Not inline: the short hint stays visible, the full explanation is opt-in.
    expect(screen.getByText("generate.panel.ghost_mode_hint")).toBeTruthy();
    expect(screen.queryByText("generate.panel.ghost_mode_tip")).toBeNull();

    fireEvent.mouseEnter(
      screen.getByRole("button", { name: "generate.panel.ghost_mode_tip_aria" }),
    );

    expect(await screen.findByText("generate.panel.ghost_mode_tip")).toBeTruthy();
  });
});
