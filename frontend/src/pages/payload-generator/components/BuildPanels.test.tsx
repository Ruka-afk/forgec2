import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { DEFAULT_BINARY_FORM } from "@/types/generate";
import { BinaryPanel } from "./BuildPanels";

// jsdom has no PointerEvent; the checkbox primitive constructs one while
// handling clicks. Aliasing it to MouseEvent keeps fireEvent.click working
// (browsers provide the real thing).
if (typeof window.PointerEvent === "undefined") {
  (window as unknown as { PointerEvent: typeof MouseEvent }).PointerEvent = MouseEvent;
}

vi.mock("@/lib/i18n", () => ({
  getLocale: () => "en",
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

// ─── Agent module selector ─────────────────────────────────────

const MODULE_CATALOG = [
  { name: "transports_advanced", title: "Advanced transports", description: "grpc/quic/wss", tasks: ["beacon"], special: "transports" },
  { name: "screen", title: "Screen and desktop", description: "screenshots", tasks: ["screenshot"] },
  { name: "p2p", title: "P2P discovery", description: "gossip", tasks: ["p2p_list"] },
];

function renderModulePanel(form: typeof DEFAULT_BINARY_FORM, setForm: (updater: unknown) => void) {
  return render(
    <TooltipProvider delay={0}>
      <BinaryPanel
        variant="exe"
        form={form}
        setForm={setForm as never}
        busy={false}
        result={null}
        onGenerate={() => {}}
        agentModules={MODULE_CATALOG}
      />
    </TooltipProvider>,
  );
}

function openModuleSection() {
  fireEvent.click(screen.getByText(/generate\.modules\.title/));
}

describe("BinaryPanel agent modules", () => {
  it("hides the selector when the catalogue could not be read", () => {
    renderBinaryPanel();
    expect(screen.queryByText(/generate\.modules\.title/)).toBeNull();
  });

  it("treats an empty selection as all modules and unchecks to an explicit list", () => {
    const setForm = vi.fn();
    renderModulePanel({ ...DEFAULT_BINARY_FORM }, setForm);
    openModuleSection();

    // Empty list = all: every catalogue entry starts checked.
    const p2p = screen.getByRole("checkbox", { name: /^p2p\b/ }) as HTMLElement;
    expect(p2p.getAttribute("aria-checked")).toBe("true");

    fireEvent.click(p2p);
    expect(setForm).toHaveBeenCalledWith({ ...DEFAULT_BINARY_FORM, modules: ["transports_advanced", "screen"] });
  });

  it("normalizes a full re-selection back to the empty all-modules sentinel", () => {
    const setForm = vi.fn();
    renderModulePanel({ ...DEFAULT_BINARY_FORM, modules: ["transports_advanced", "screen"] }, setForm);
    openModuleSection();

    const p2p = screen.getByRole("checkbox", { name: /^p2p\b/ }) as HTMLElement;
    expect(p2p.getAttribute("aria-checked")).toBe("false");

    fireEvent.click(p2p);
    // Back to everything → "" (empty list) so the build follows the catalogue.
    expect(setForm).toHaveBeenCalledWith({ ...DEFAULT_BINARY_FORM, modules: [] });
  });

  it("locks the advanced transports checkbox in a light build", () => {
    const setForm = vi.fn();
    renderModulePanel({ ...DEFAULT_BINARY_FORM, slim: true }, setForm);
    openModuleSection();

    const transports = screen.getByRole("checkbox", { name: /^transports_advanced\b/ }) as HTMLElement;
    expect(transports.getAttribute("aria-disabled")).toBe("true");
    expect(transports.getAttribute("aria-checked")).toBe("false");
    expect(screen.getByText("generate.modules.slim_locked")).toBeTruthy();
  });
});
