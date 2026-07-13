import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
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

    // The fixture's tier is already set, so 检索日志 (Task 8) legitimately
    // re-renders this title in the ledger below the list — click inside the
    // source-list card specifically, not by a page-wide text match.
    await userEvent.click(within(screen.getByTestId("dossier-source-list")).getByText("《卫星图看中国变绿》"));
    const log = screen.getByTestId("material-event-log");
    expect(log).toHaveTextContent("source_opened");
    expect(log).toHaveTextContent("src-blog-china-greening");
  });
});
