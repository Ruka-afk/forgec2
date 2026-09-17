import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { withChartData } from "./withChartData";
import { api } from "@/lib/api";

vi.mock("@/lib/api", () => ({
  api: { get: vi.fn() },
}));

describe("withChartData render boundary", () => {
  it("contains a chart render crash and recovers on retry", async () => {
    vi.mocked(api.get).mockResolvedValue({ ok: true });
    let throws = true;
    const FlakyChart = () => {
      if (throws) throw new Error("chart boom");
      return <div data-testid="chart-ok">chart</div>;
    };
    const Chart = withChartData<boolean, Record<string, never>>(
      () => <FlakyChart />,
      "/chart-x",
      (raw) => raw as boolean,
    );

    // Silence React's error logging for the intentional crash.
    const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      render(<Chart />);
      await waitFor(() => expect(screen.getByRole("alert")).toBeTruthy());
      // Boundary fallback, not a page-level blank: retry is offered.
      const retry = screen.getByRole("button");
      throws = false;
      fireEvent.click(retry);
      await waitFor(() => expect(screen.getByTestId("chart-ok")).toBeTruthy());
      expect(screen.queryByRole("alert")).toBeNull();
    } finally {
      errSpy.mockRestore();
    }
  });
});
