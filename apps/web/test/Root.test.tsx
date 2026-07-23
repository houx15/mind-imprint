import { describe, it, expect, afterEach, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { Root } from "@/Root";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      listTasks: vi.fn(async () => []),
      createTask: vi.fn(),
      getMe: vi.fn(async () => { throw new Error("401"); }),
      signout: vi.fn(async () => {}),
    },
  };
});

afterEach(() => {
  window.history.pushState({}, "", "/");
});

describe("Root", () => {
  it("renders the app shell by default (its login screen, no gallery)", async () => {
    window.history.pushState({}, "", "/");
    render(<Root />);
    await waitFor(() => expect(screen.getByRole("button", { name: "登录" })).toBeTruthy());
    expect(screen.queryByTestId("envelope-json")).toBeNull();
  });

  it("renders the deterministic card gallery when ?demo is present", () => {
    window.history.pushState({}, "", "/?demo");
    render(<Root />);
    // The Harness gallery exposes the live envelope JSON; the app shell does not.
    expect(screen.getByTestId("envelope-json")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "登录" })).toBeNull();
  });

  it("no longer has a ?studio side door — it falls through to the app shell", async () => {
    window.history.pushState({}, "", "/?studio");
    render(<Root />);
    // The shell's auth gate, not a bare Studio: ?studio is retired (Slice 5d).
    await waitFor(() => expect(screen.getByRole("button", { name: "登录" })).toBeTruthy());
  });
});
