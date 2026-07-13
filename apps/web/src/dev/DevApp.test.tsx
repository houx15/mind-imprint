import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "./DevApp";

describe("DevApp", () => {
  it("renders the cards harness by default", () => {
    render(<DevApp />);
    expect(screen.getByRole("button", { name: "卡片" })).toBeInTheDocument();
  });

  it("switches to the material panel and logs source_opened on open", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "素材" }));
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();

    await userEvent.click(screen.getByText("《卫星图看中国变绿》"));
    const log = screen.getByTestId("material-event-log");
    expect(log).toHaveTextContent("source_opened");
    expect(log).toHaveTextContent("src-blog-china-greening");
  });
});
