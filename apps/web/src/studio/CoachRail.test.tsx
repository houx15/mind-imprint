import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { CoachRail } from "./CoachRail";
import { STUDIO_FIXTURE } from "./fixtures";

const c = STUDIO_FIXTURE.coach;

function baseProps() {
  return {
    anchor: c.anchor,
    messages: c.messages,
    equipment: c.equipment,
    activeView: "结构" as const,
    onDisposition: () => {},
    onOpenMethodology: () => {},
    onSend: () => {},
  };
}

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

describe("CoachRail live card slot (task 9)", () => {
  it("renders the craap proposal → annotate card and wires open", () => {
    const onOpenCard = vi.fn(), onSubmitCard = vi.fn(), onSkipCard = vi.fn();
    const spec = CARD_REGISTRY["craap"]!;
    const { rerender } = render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci1", cardId: "craap", spec, status: "proposed", anchors: [] }}
        onOpenCard={onOpenCard}
        onSubmitCard={onSubmitCard}
        onSkipCard={onSkipCard}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /打开|开始/ })); // the proposal open affordance
    expect(onOpenCard).toHaveBeenCalled();
    rerender(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci1", cardId: "craap", spec, status: "active", anchors: [] }}
        onOpenCard={onOpenCard}
        onSubmitCard={onSubmitCard}
        onSkipCard={onSkipCard}
      />,
    );
    // craap.json's primitive is "annotate" — the active branch must fork to
    // StudioAnnotateCard, not the schema-driven StudioCardSheet.
    expect(screen.getByText("作用与风险（自己写）")).toBeInTheDocument();
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
  });
});

describe("CoachRail active-card fork (task 10): annotate vs schema-driven", () => {
  it("primitive === 'annotate' renders StudioAnnotateCard, fed the live anchors, not StudioCardSheet", () => {
    const spec = CARD_REGISTRY["craap"]!;
    const anchors = [
      { id: "a1", material_id: "m1", block_id: "b1", start: 0, end: 10, quote: "示例引文", dimension: "权威性", author: "ai" as const, question: "这条来源的作者是谁？", answer: "" },
    ];
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci1", cardId: "craap", spec, status: "active", anchors }}
      />,
    );
    expect(screen.getByText("作用与风险（自己写）")).toBeInTheDocument();
    expect(screen.getByText("这条来源的作者是谁？")).toBeInTheDocument(); // the fed-in anchor's question
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
  });

  it("a non-annotate primitive keeps rendering StudioCardSheet", () => {
    const spec = CARD_REGISTRY["concession"]!;
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci2", cardId: "concession", spec, status: "active", anchors: [] }}
      />,
    );
    expect(screen.getByText("提交并钉到过程树")).toBeInTheDocument();
    expect(screen.queryByText("作用与风险（自己写）")).not.toBeInTheDocument();
  });
});
