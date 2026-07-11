import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CoachRail } from "./CoachRail";
import { STUDIO_FIXTURE } from "./fixtures";

const c = STUDIO_FIXTURE.coach;

describe("CoachRail", () => {
  it("renders the anchor status, the thread, and the 装备栏 toggle", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    // the anchor renders in both the header status and the disposition
    // card's "锚定" pill, so assert presence rather than uniqueness
    expect(screen.getAllByText(new RegExp(c.anchor)).length).toBeGreaterThan(0);
    expect(screen.getByText("孤儿证据")).toBeInTheDocument(); // the flag message label
  });
  it("renders exactly ONE 装备栏 toggle (the composer's) — not a duplicate from EquipmentBar", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    expect(screen.getAllByTitle("装备栏 · 工具卡")).toHaveLength(1);
  });
  it("does not double the 锚定 label: the disposition card's tag pill is a bare criterion, not a second 锚定", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    // exactly one "锚定 ..." label should render (DispositionCard's anchor
    // pill); the ai message's tag must be the bare criterion (e.g. "D5"),
    // not "锚定 D5" — otherwise this rail shows "锚定" twice.
    expect(screen.getAllByText(/^锚定/).length).toBe(1);
    // "D5" renders twice by design: once as the thread message's own tag
    // pill, once as DispositionCard's tag pill — neither is prefixed with 锚定.
    expect(screen.getAllByText("D5").length).toBeGreaterThan(0);
  });

  it("sends composer text", () => {
    const onSend = vi.fn();
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={onSend} />);
    fireEvent.change(screen.getByPlaceholderText(/发给印记/), { target: { value: "我加了一条证据" } });
    fireEvent.click(screen.getByLabelText(/发送|send/i));
    expect(onSend).toHaveBeenCalledWith("我加了一条证据");
  });
});
