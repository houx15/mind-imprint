import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import { CoachRail } from "@/studio/CoachRail";
import { STUDIO_FIXTURE } from "@/studio/fixtures";

// Fakes for the voice stack (AsrStream + MicCapture). Defined via vi.hoisted
// so the vi.mock factories below — which vitest hoists above these imports —
// can reference them. Each fake tracks its instances so a test can reach
// into the most recent one and drive its callbacks (partial/final/error) or
// control whether mic.start() resolves or rejects (permission denial).
const { FakeAsrStream, FakeMicCapture } = vi.hoisted(() => {
  class FakeAsrStream {
    static instances: FakeAsrStream[] = [];
    partialCb: ((t: string) => void) | null = null;
    finalCb: ((t: string) => void) | null = null;
    errorCb: ((m: string) => void) | null = null;
    stopped = false;
    constructor() {
      FakeAsrStream.instances.push(this);
    }
    onPartial(cb: (t: string) => void) {
      this.partialCb = cb;
    }
    onFinal(cb: (t: string) => void) {
      this.finalCb = cb;
    }
    onError(cb: (m: string) => void) {
      this.errorCb = cb;
    }
    sendPCM() {}
    stop() {
      this.stopped = true;
    }
  }
  class FakeMicCapture {
    static instances: FakeMicCapture[] = [];
    static startBehavior: "resolve" | "reject" = "resolve";
    static rejectMessage = "麦克风权限被拒绝";
    stopped = false;
    constructor() {
      FakeMicCapture.instances.push(this);
    }
    async start(_onPcm: (pcm: Int16Array) => void) {
      if (FakeMicCapture.startBehavior === "reject") {
        throw new Error(FakeMicCapture.rejectMessage);
      }
    }
    stop() {
      this.stopped = true;
    }
  }
  return { FakeAsrStream, FakeMicCapture };
});

vi.mock("@/api/voice", () => ({ AsrStream: FakeAsrStream }));
vi.mock("@/audio/capture", () => ({ MicCapture: FakeMicCapture }));

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

  it("primitive === 'graph' (Toulmin) renders only a handoff nudge to the 结构 pane, NOT a second interactive sheet", () => {
    const spec = CARD_REGISTRY["toulmin"]!;
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci-t", cardId: "toulmin", spec, status: "active", anchors: [], materialId: "" }}
      />,
    );
    // The nudge points at the center pane…
    expect(screen.getByText(/论证结构在「结构」环节里搭/)).toBeInTheDocument();
    // …and the rail renders NEITHER a StudioCardSheet (would double-submit)…
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
    // …NOR the Graph builder's own lock button (that lives in the 结构 pane).
    expect(screen.queryByRole("button", { name: /全部锁定，完成论证/ })).not.toBeInTheDocument();
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

  it("primitive === 'compare' (SIFT) forks to StudioCompareCard, not StudioCardSheet — branched on primitive, never on the card id", () => {
    const spec = CARD_REGISTRY["sift"]!;
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci3", cardId: "sift", spec, status: "active", anchors: [], materialId: "mat-blog" }}
        materials={[{
          id: "mat-blog", title: "《卫星图看中国变绿》", sourceUrl: "", kind: "article", origin: "fetched",
          blocks: [], locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0, lateralRead: false, isLateralInstrument: false, siftSkipped: false,
          lateralRelation: "", lateralJudgment: "",
        }]}
      />,
    );
    expect(screen.getByRole("button", { name: "锁定这张卡" })).toBeInTheDocument();
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
    expect(screen.queryByText("作用与风险（自己写）")).not.toBeInTheDocument();
  });

  // Whole-branch review: the SIFT card's own "添加信源" affordance used to be
  // wired here to onSelectStation("S3") — a no-op (a compare card only ever
  // surfaces while already on S3) that also never rendered on the seeded
  // project (it only showed when lateralCandidates.length === 0). Deleted
  // rather than rewired — StudioCompareCard now shows only honest
  // informational text with no dead button; CoachRail carries no
  // onSelectStation prop for it at all.
  it("shows no 添加信源 button on the SIFT card when no independent source exists yet — the center pane's own flow covers it", () => {
    const spec = CARD_REGISTRY["sift"]!;
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci3", cardId: "sift", spec, status: "active", anchors: [], materialId: "mat-blog" }}
        materials={[{
          id: "mat-blog", title: "《卫星图看中国变绿》", sourceUrl: "", kind: "article", origin: "fetched",
          blocks: [], locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0, lateralRead: false, isLateralInstrument: false, siftSkipped: false,
          lateralRelation: "", lateralJudgment: "",
        }]}
      />,
    );
    expect(screen.getByText(/还没有独立来源/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "添加信源" })).not.toBeInTheDocument();
  });

  // Made-up bucket vocabulary (not the real fact-opinion-value.json labels) —
  // proves the fork renders StudioSortCard, not that any particular card's
  // wording made it through, and keeps this file config-agnostic like the
  // host itself.
  it("primitive === 'sort' forks to StudioSortCard, not StudioCardSheet", () => {
    const spec = {
      id: "made-up-sort",
      name: "分类卡",
      category: "知识工具",
      purpose: "把陈述分类",
      primitive: "sort",
      params: {
        buckets: [
          { id: "红", label: "红桶", hint: "" },
          { id: "蓝", label: "蓝桶", hint: "" },
        ],
        min_items: 1,
        item_prompt: "写一句话",
        reason_prompt: "为什么",
      },
    } as any;
    // Sort only renders bucket chips per row, so seed one row via a
    // persisted anchor (the same rehydration path StudioSortCard uses).
    const anchors = [
      { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "一句话", dimension: "红", author: "student" as const, question: "", answer: "" },
    ];
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci4", cardId: "made-up-sort", spec, status: "active", anchors }}
      />,
    );
    expect(screen.getByText("红桶")).toBeInTheDocument();
    expect(screen.getByText("蓝桶")).toBeInTheDocument();
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
    expect(screen.queryByText("作用与风险（自己写）")).not.toBeInTheDocument();
  });

  // Made-up stop vocabulary (not the real certainty-spectrum.json labels) —
  // same rationale as the sort test above.
  it("primitive === 'scale' forks to StudioScaleCard, not StudioCardSheet", () => {
    const spec = {
      id: "made-up-scale",
      name: "光谱卡",
      category: "知识工具",
      purpose: "把结论放到光谱上",
      primitive: "scale",
      params: {
        buckets: [
          { id: "低", label: "低确定度", hint: "" },
          { id: "高", label: "高确定度", hint: "" },
        ],
        min_items: 1,
        item_prompt: "你的结论是什么？",
        reason_prompt: "为什么",
        rewrite_prompt: "重写这句话",
      },
    } as any;
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci5", cardId: "made-up-scale", spec, status: "active", anchors: [] }}
      />,
    );
    expect(screen.getByText("低确定度")).toBeInTheDocument();
    expect(screen.getByText("高确定度")).toBeInTheDocument();
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
    expect(screen.queryByText("作用与风险（自己写）")).not.toBeInTheDocument();
  });

  // Made-up column vocabulary (not the real perspective-matrix.json
  // 立场主张/依据/盲区 labels) — same rationale as the sort/scale tests above.
  it("primitive === 'matrix' forks to StudioMatrixCard, not StudioCardSheet", () => {
    const spec = {
      id: "made-up-matrix",
      name: "多视角卡",
      category: "知识工具",
      purpose: "补齐遗漏的视角",
      primitive: "matrix",
      params: {
        cols: [
          { id: "view", label: "看法", q: "这个人怎么看？" },
          { id: "reason", label: "理由", q: "为什么这么想？" },
        ],
        min_items: 1,
        row_prompt: "这是谁的视角？",
      },
    } as any;
    // Matrix only renders its column headers per row (there is no decorative
    // axis header like Scale's), so seed one row via a persisted anchor —
    // the same rehydration path StudioMatrixCard uses.
    const anchors = [
      { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "一个视角", dimension: "view", author: "student" as const, question: "", answer: "" },
    ];
    render(
      <CoachRail
        {...baseProps()}
        card={{ cardInstanceId: "ci6", cardId: "made-up-matrix", spec, status: "active", anchors }}
      />,
    );
    expect(screen.getByText("看法")).toBeInTheDocument();
    expect(screen.getByText("理由")).toBeInTheDocument();
    expect(screen.queryByText("提交并钉到过程树")).not.toBeInTheDocument();
    expect(screen.queryByText("作用与风险（自己写）")).not.toBeInTheDocument();
  });
});

describe("CoachRail voice input (5d review IMPORTANT: the mic must not be inert)", () => {
  beforeEach(() => {
    FakeAsrStream.instances = [];
    FakeMicCapture.instances = [];
    FakeMicCapture.startBehavior = "resolve";
  });

  it("clicking the mic starts capture, shows a recording state, and lands a transcript in the editable composer", async () => {
    render(<CoachRail {...baseProps()} />);
    const micBtn = screen.getByTitle("语音输入");

    fireEvent.click(micBtn);

    // Capture actually started: a MicCapture + AsrStream were instantiated,
    // not just a click handler firing into the void.
    expect(await screen.findByText("正在录音… 说完点麦克风结束")).toBeInTheDocument();
    expect(FakeMicCapture.instances).toHaveLength(1);
    expect(FakeAsrStream.instances).toHaveLength(1);

    // A transcript (interim or final) lands where the student can see and
    // edit it — the composer textarea — and is never auto-sent.
    const asr = FakeAsrStream.instances[0]!;
    act(() => {
      asr.partialCb?.("我口述的");
    });
    const textarea = screen.getByPlaceholderText(/发给印记/) as HTMLTextAreaElement;
    expect(textarea.value).toBe("我口述的");
    act(() => {
      asr.finalCb?.("我口述的一句话");
    });
    expect(textarea.value).toBe("我口述的一句话");
    expect(screen.queryByText(/正在录音/)).toBeInTheDocument(); // still recording, not auto-sent/closed

    // Clicking the mic again ends the recording and releases mic + socket.
    fireEvent.click(micBtn);
    expect(screen.queryByText(/正在录音/)).not.toBeInTheDocument();
    expect(FakeMicCapture.instances[0]!.stopped).toBe(true);
    expect(FakeAsrStream.instances[0]!.stopped).toBe(true);
  });

  it("surfaces a mic/ASR error to the student instead of failing silently", async () => {
    FakeMicCapture.startBehavior = "reject";
    render(<CoachRail {...baseProps()} />);

    fireEvent.click(screen.getByTitle("语音输入"));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(FakeMicCapture.rejectMessage);
    // Not stuck showing a live recording state after the failure.
    expect(screen.queryByText(/正在录音/)).not.toBeInTheDocument();
  });

  it("surfaces an ASR stream error raised mid-recording", async () => {
    render(<CoachRail {...baseProps()} />);
    fireEvent.click(screen.getByTitle("语音输入"));
    await screen.findByText("正在录音… 说完点麦克风结束");

    const asr = FakeAsrStream.instances[0]!;
    act(() => {
      asr.errorCb?.("语音连接中断");
    });

    expect(await screen.findByRole("alert")).toHaveTextContent("语音连接中断");
    expect(screen.queryByText(/正在录音/)).not.toBeInTheDocument();
  });
});
