import { describe, it, expect, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Root } from "./Root";

afterEach(() => {
  window.history.pushState({}, "", "/");
});

describe("Root", () => {
  it("renders the app shell by default (its login screen, no gallery)", () => {
    window.history.pushState({}, "", "/");
    render(<Root />);
    expect(screen.getByRole("button", { name: "登录" })).toBeTruthy();
    expect(screen.queryByTestId("envelope-json")).toBeNull();
  });

  it("renders the deterministic card gallery when ?demo is present", () => {
    window.history.pushState({}, "", "/?demo");
    render(<Root />);
    // The Harness gallery exposes the live envelope JSON; the app shell does not.
    expect(screen.getByTestId("envelope-json")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "登录" })).toBeNull();
  });
});
