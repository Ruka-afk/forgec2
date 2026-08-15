import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { useAppStore } from "@/lib/store";
import CommandPalette from "@/components/CommandPalette";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock }),
}));

vi.mock("@/lib/i18n", () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string>) => {
      if (key === "palette.placeholder") return "Type to jump…";
      if (key === "palette.title") return "Command palette";
      if (key === "palette.no_results") return "No results";
      if (key === "palette.agents") return "Agents";
      if (key === "palette.search_for") return `Search for "${params?.query}"`;
      return key;
    },
  }),
}));

vi.mock("@/lib/hooks/useAgentList", () => ({
  useAgentList: () => ({
    agents: [
      { id: "a1", hostname: "desktop-win11", username: "corp\\jdoe", ip: "10.0.1.5", os: "windows" },
      { id: "a2", hostname: "srv-linux-01", username: "root", ip: "10.0.2.10", os: "linux" },
    ],
    loading: false,
    error: null,
    refresh: vi.fn(),
  }),
}));

describe("CommandPalette", () => {
  beforeEach(() => {
    useAppStore.setState({ commandPaletteOpen: false });
    pushMock.mockClear();
  });

  it("renders nothing while closed", () => {
    render(<CommandPalette />);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByLabelText(/type to jump/i)).toBeNull();
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

  it("lists matching agents under the Agents group", () => {
    useAppStore.setState({ commandPaletteOpen: true });
    render(<CommandPalette />);
    const input = screen.getByLabelText(/type to jump/i);
    fireEvent.change(input, { target: { value: "desktop-win" } });
    expect(screen.getByText("desktop-win11")).toBeTruthy();
    expect(screen.getByText("corp\\jdoe · 10.0.1.5")).toBeTruthy();
    expect(screen.queryByText("srv-linux-01")).toBeNull();
  });

  it("navigates to the agent page on selection", () => {
    useAppStore.setState({ commandPaletteOpen: true });
    render(<CommandPalette />);
    const input = screen.getByLabelText(/type to jump/i);
    fireEvent.change(input, { target: { value: "srv-linux" } });
    fireEvent.click(screen.getByText("srv-linux-01"));
    expect(pushMock).toHaveBeenCalledWith("/agents/a2");
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