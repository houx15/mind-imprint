import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "./DevApp";

describe("DevApp", () => {
  it("renders the cards harness by default", () => {
    render(<DevApp />);
    expect(screen.getByRole("button", { name: "卡片" })).toBeInTheDocument();
  });

  it("switches to the store panel", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "存储" }));
    expect(screen.getByRole("button", { name: "新建任务" })).toBeInTheDocument();
  });
});
