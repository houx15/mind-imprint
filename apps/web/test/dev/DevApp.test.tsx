import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "@/dev/DevApp";

describe("DevApp", () => {
  it("renders the cards harness by default", () => {
    render(<DevApp />);
    expect(screen.getByRole("button", { name: "卡片" })).toBeInTheDocument();
  });

  it("switches to the material panel and opens a source into the detail view", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "素材" }));
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();

    // The fixture's tier is already set, so 检索日志 (Task 8) legitimately
    // re-renders this title in the ledger below the list — click inside the
    // source-list card specifically, not by a page-wide text match.
    await userEvent.click(within(screen.getByTestId("dossier-source-list")).getByText("《卫星图看中国变绿》"));
    expect(screen.getByText("返回信源列表")).toBeInTheDocument();

    // source_opened is the server's canonical event (written by POST
    // /open) — the dev harness never had a real event producer wired to
    // SourceDossier, so it must not render a fake event log.
    expect(screen.queryByTestId("material-event-log")).not.toBeInTheDocument();
  });
});
