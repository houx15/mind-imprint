import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachLinkOffer } from "@/workspace/blocks/CoachLinkOffer";

// The link-in-coach → resource bridge chip. It must show a readable host, act
// ONLY on the student's tap (克制/铁律: nothing fetches or promotes on render),
// reflect the added state, and dismiss on 跳过.
describe("CoachLinkOffer", () => {
  const url = "https://www.nature.com/articles/greening?x=1";

  it("renders the host and acts only on tap", () => {
    const onAdd = vi.fn();
    const onReadTogether = vi.fn();
    const onDismiss = vi.fn();
    render(
      <CoachLinkOffer url={url} status="idle" onAdd={onAdd} onReadTogether={onReadTogether} onDismiss={onDismiss} />,
    );
    // host shown without the www. prefix; no query-string wall.
    expect(screen.getByText("nature.com")).toBeInTheDocument();
    // 克制: nothing acts on render.
    expect(onAdd).not.toHaveBeenCalled();
    expect(onReadTogether).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /加入文献库/ }));
    expect(onAdd).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: /一起读这篇/ }));
    expect(onReadTogether).toHaveBeenCalledTimes(1);
  });

  it("shows 已加入 and hides the add button when added", () => {
    render(
      <CoachLinkOffer url={url} status="added" onAdd={vi.fn()} onReadTogether={vi.fn()} onDismiss={vi.fn()} />,
    );
    expect(screen.getByText(/已加入文献库/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /加入文献库/ })).toBeNull();
    // read-together stays available even after adding.
    expect(screen.getByRole("button", { name: /一起读这篇/ })).toBeInTheDocument();
  });

  it("disables actions while adding", () => {
    const onReadTogether = vi.fn();
    render(
      <CoachLinkOffer url={url} status="adding" onAdd={vi.fn()} onReadTogether={onReadTogether} onDismiss={vi.fn()} />,
    );
    const read = screen.getByRole("button", { name: /一起读这篇/ });
    expect(read).toBeDisabled();
    fireEvent.click(read);
    expect(onReadTogether).not.toHaveBeenCalled();
  });

  it("dismisses on 跳过 without acting", () => {
    const onAdd = vi.fn();
    const onDismiss = vi.fn();
    render(
      <CoachLinkOffer url={url} status="idle" onAdd={onAdd} onReadTogether={vi.fn()} onDismiss={onDismiss} />,
    );
    fireEvent.click(screen.getByRole("button", { name: "跳过" }));
    expect(onDismiss).toHaveBeenCalled();
    expect(onAdd).not.toHaveBeenCalled();
  });
});
