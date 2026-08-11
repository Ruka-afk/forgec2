import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useAppStore } from "@/lib/store";
import CommandPalette from "@/components/CommandPalette";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      if (key === "palette.placeholder") return "Type to jump…";
      if (key === "palette.title") return "Command palette";
      if (key === "palette.no_results") return "No results";
      if (key === "palette.search_for") return `Search for "${params?.query}"`;
      return key;
    },
  }),
}));

describe("CommandPalette", () => {
  beforeEach(() => {
    useAppStore.setState({ commandPaletteOpen: false });
  });

  it("renders nothing while closed", () => {
    const { container } = render(<CommandPalette />);
    expect(container.firstChild).toBeNull();
  });

  it("lists all nav pages when open", () => {
    useAppStore.setState({ commandPaletteOpen: true });
    render(<CommandPalette />);
    expect(screen.getByLabelText(/type to jump/i)).toBeTruthy();
    expect(screen.getByText("nav.dashboard")).toBeTruthy();
    expect(screen.getByText("nav.beacons")).toBeTruthy();
    expect(screen.getByText("nav.settings")).toBeTruthy();
  });

  it("filters pages by query and offers search fallback", () => {
    useAppStore.setState({ commandPaletteOpen: true });
    render(<CommandPalette />);
    const input = screen.getByLabelText(/type to jump/i);
    fireEvent.change(input, { target: { value: "bloodhound" } });
    expect(screen.getByText("nav.bloodhound")).toBeTruthy();
    expect(screen.queryByText("nav.dashboard")).toBeNull();
    fireEvent.change(input, { target: { value: "kerberoast" } });
    expect(screen.getByText(/search for "kerberoast"/i)).toBeTruthy();
  });

  it("closes on Escape", () => {
    useAppStore.setState({ commandPaletteOpen: true });
    render(<CommandPalette />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(useAppStore.getState().commandPaletteOpen).toBe(false);
  });

  it("opens and closes via Ctrl+K", () => {
    render(<CommandPalette />);
    expect(useAppStore.getState().commandPaletteOpen).toBe(false);
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(useAppStore.getState().commandPaletteOpen).toBe(true);
    fireEvent.keyDown(window, { key: "k", ctrlKey: true });
    expect(useAppStore.getState().commandPaletteOpen).toBe(false);
  });
});