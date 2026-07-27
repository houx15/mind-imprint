import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DevApp } from "@/dev/DevApp";

describe("DevApp", () => {
  it("renders the cards harness by default", () => {
    render(<DevApp />);
    expect(screen.getByRole("button", { name: "卡片" })).toBeInTheDocument();
  });

  // Task 11 (spec-read-together-redesign): SourceDossier no longer owns an
  // in-place detail view — reading a source now goes through the focused
  // ReadingRoom surface (onOpenReading), which this dev harness's
  // MaterialPanel does not wire up. A row click is therefore a no-op here;
  // the list itself is still the real thing (not an inert fixture render).
  it("switches to the material panel and renders the real source list", async () => {
    render(<DevApp />);
    await userEvent.click(screen.getByRole("button", { name: "素材" }));
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();

    // The fixture's tier is already set, so 检索日志 (Task 8) legitimately
    // re-renders this title in the ledger below the list — click inside the
    // source-list card specifically, not by a page-wide text match.
    await userEvent.click(within(screen.getByTestId("dossier-source-list")).getByText("《卫星图看中国变绿》"));
    // No onOpenReading wired here — the click is a safe no-op, and the list
    // stays exactly as it was.
    expect(screen.getByTestId("dossier-source-list")).toBeInTheDocument();

    // source_opened is the server's canonical event (written by POST
    // /open) — the dev harness never had a real event producer wired to
    // SourceDossier, so it must not render a fake event log.
    expect(screen.queryByTestId("material-event-log")).not.toBeInTheDocument();
  });
});
