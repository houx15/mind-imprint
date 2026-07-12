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
  it("does not double-prefix the tag chip: the ai message's tag is a bare criterion, not '锚定 D5'", () => {
    render(<CoachRail anchor={c.anchor} messages={c.messages} equipment={c.equipment}
      activeView="结构" onDisposition={() => {}} onOpenMethodology={() => {}} onSend={() => {}} />);
    // "锚定 ..." labels legitimately render twice now: once next to the ai
    // message's own tag chip (when the message carries an anchor), once in
    // DispositionCard's anchor pill. Neither is "锚定 D5" — the tag chip
    // itself must stay a bare criterion (e.g. "D5"), never prefixed with 锚定.
    expect(screen.getAllByText(/^锚定/).length).toBe(2);
    expect(screen.queryByText("锚定 D5")).not.toBeInTheDocument();
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

describe("CoachRail ai message anchor (5a carry-forward)", () => {
  it("renders the criterion chip and the 锚定 anchor label", () => {
    render(
      <CoachRail
        anchor="论证图 · 治理决心主张"
        messages={[{ kind: "ai", body: "补完，门禁第①条就过了。", tag: "D5", anchor: "论证图 · 治理决心主张" }]}
        equipment={STUDIO_FIXTURE.coach.equipment}
        activeView="结构"
        onDisposition={() => {}}
        onOpenMethodology={() => {}}
        onSend={() => {}}
      />,
    );
    // "D5" renders twice by design (thread tag pill + DispositionCard tag
    // pill), same as the fixture-driven tests above.
    expect(screen.getAllByText("D5").length).toBeGreaterThan(0);
    // the 锚定 label now renders once per ai message with an anchor, plus
    // once in DispositionCard's own anchor pill — both match this anchor.
    expect(screen.getAllByText(/锚定 论证图 · 治理决心主张/).length).toBeGreaterThan(0);
  });
});
