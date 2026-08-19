import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Permission } from "@/components/ui/permission";
import { useAppStore } from "@/lib/store";

describe("Permission", () => {
  beforeEach(() => {
    useAppStore.setState({ currentPermissions: null });
  });

  it("fails open while permissions are still loading (null)", () => {
    render(<Permission perms="agents.read">visible</Permission>);
    expect(screen.getByText("visible")).toBeTruthy();
  });

  it("renders children when the permission is held", () => {
    useAppStore.setState({ currentPermissions: ["agents.read", "settings.read"] });
    render(<Permission perms="agents.read">visible</Permission>);
    expect(screen.getByText("visible")).toBeTruthy();
  });

  it("hides children when the permission is missing", () => {
    useAppStore.setState({ currentPermissions: ["agents.read"] });
    render(<Permission perms="users.write">hidden</Permission>);
    expect(screen.queryByText("hidden")).toBeNull();
  });

  it("shows fallback when the permission is missing", () => {
    useAppStore.setState({ currentPermissions: ["agents.read"] });
    render(<Permission perms="users.write" fallback={<div>denied</div>}>hidden</Permission>);
    expect(screen.queryByText("hidden")).toBeNull();
    expect(screen.getByText("denied")).toBeTruthy();
  });

  it("any mode allows any held permission", () => {
    useAppStore.setState({ currentPermissions: ["roles.read"] });
    render(
      <Permission mode="any" perms={["users.read", "roles.read"]}>
        visible
      </Permission>,
    );
    expect(screen.getByText("visible")).toBeTruthy();
  });

  it("all mode requires every permission to be held", () => {
    useAppStore.setState({ currentPermissions: ["roles.read"] });
    render(
      <Permission mode="all" perms={["users.read", "roles.read"]}>
        hidden
      </Permission>,
    );
    expect(screen.queryByText("hidden")).toBeNull();
  });

  it("passes the resolved boolean to a render-prop child", () => {
    useAppStore.setState({ currentPermissions: ["agents.read"] });
    render(
      <Permission perms="agents.read">
        {(allowed) => (allowed ? <span data-testid="allowed">yes</span> : <span>no</span>)}
      </Permission>,
    );
    expect(screen.getByTestId("allowed")).toBeTruthy();
    expect(screen.queryByText("no")).toBeNull();
  });
});