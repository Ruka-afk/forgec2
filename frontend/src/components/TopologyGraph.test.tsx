import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import TopologyGraph from "./TopologyGraph";

/** Minimal stand-in for window.vis.Network; records construction. */
class FakeNetwork {
  static instances: FakeNetwork[] = [];
  container: HTMLElement | undefined;
  constructor(container?: HTMLElement) {
    this.container = container;
    FakeNetwork.instances.push(this);
  }
  on() {}
  setOptions() {}
  setData() {}
  stabilize() {}
  destroy() {}
}

const PROPS = {
  data: { nodes: [{ id: "a", label: "A" }], edges: [] },
  meshData: { nodes: [], edges: [] },
  useMeshSource: false,
  physicsEnabled: false,
  loading: false,
  onNodeClick: () => {},
};

const SCRIPT_SELECTOR = 'script[src="/js/vis-network.min.js"]';
const bundleScript = () => document.head.querySelector(SCRIPT_SELECTOR) as HTMLScriptElement | null;

describe("TopologyGraph bundle loading", () => {
  beforeEach(() => {
    FakeNetwork.instances = [];
  });

  afterEach(() => {
    vi.useRealTimers();
    delete window.vis;
  });

  it("creates the network without fetching the bundle when vis is already loaded", () => {
    window.vis = { Network: FakeNetwork };
    const { container } = render(<TopologyGraph {...PROPS} />);
    expect(container.querySelector('[role="img"]')).not.toBeNull();
    expect(FakeNetwork.instances).toHaveLength(1);
    expect(bundleScript()).toBeNull();
  });

  it("explains a failed bundle fetch and retries on demand", () => {
    window.vis = undefined;
    const { container } = render(<TopologyGraph {...PROPS} />);

    const script = bundleScript();
    expect(script).not.toBeNull();
    act(() => {
      script!.dispatchEvent(new Event("error"));
    });

    // Fallback replaces the canvas instead of leaving it silently blank.
    expect(container.querySelector('[role="img"]')).toBeNull();
    const retry = screen.getByRole("button");

    // Retry re-fetches the bundle (window.vis is still unset until it loads).
    // The effect re-run is a passive effect, so it must flush inside act().
    act(() => {
      fireEvent.click(retry);
    });
    const retried = bundleScript();
    expect(retried).not.toBeNull();
    expect(FakeNetwork.instances).toHaveLength(0);

    window.vis = { Network: FakeNetwork };
    act(() => {
      retried!.dispatchEvent(new Event("load"));
    });
    expect(FakeNetwork.instances).toHaveLength(1);
    expect(container.querySelector('[role="img"]')).not.toBeNull();
  });

  it("times out a fetch that neither loads nor errors", () => {
    vi.useFakeTimers();
    window.vis = undefined;
    const { container } = render(<TopologyGraph {...PROPS} />);
    expect(container.querySelector('[role="img"]')).not.toBeNull();

    act(() => {
      vi.advanceTimersByTime(15_000);
    });

    expect(container.querySelector('[role="img"]')).toBeNull();
    expect(screen.getByRole("button")).toBeTruthy();
  });
});
