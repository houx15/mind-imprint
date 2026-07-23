import { render, screen, fireEvent } from "@testing-library/react";
import { test, expect, vi } from "vitest";
import { ReviewView } from "@/studio/views/ReviewView";
import type { GaugeFx, SelfScoreFx, PredictionFx, ReflectionFx, DeclarationFx } from "@/studio/state";

const gauges: GaugeFx[] = [
  { code: "表D", name: "来源与证据", lit: 3, total: 4, note: "孤儿证据没接上", level: "partial" },
  { code: "表E", name: "分析", lit: 0, total: 4, note: "", level: "empty" },
  { code: "表F", name: "评估", lit: 3, total: 3, note: "", level: "full" },
  { code: "表H", name: "表达与组织", lit: 3, total: 3, note: "", level: "full" },
];

const selfScore: SelfScoreFx = {
  dims: [
    { code: "表D", name: "来源与证据", band: 2 },
    { code: "表E", name: "分析", band: -1 },
    { code: "表F", name: "评估", band: 0 },
    { code: "表H", name: "表达与组织", band: -1 },
  ],
  bands: ["还需努力", "基本达到", "稳了"],
};

const predictionUnrevealed: PredictionFx = {
  predicted: [
    { code: "表E", name: "分析" },
    { code: "表F", name: "评估" },
  ],
  actual: [],
  overlap: 0,
  revealed: false,
};

const predictionRevealed: PredictionFx = {
  predicted: [
    { code: "表E", name: "分析" },
    { code: "表F", name: "评估" },
  ],
  actual: [
    { code: "表F", name: "评估" },
    { code: "表H", name: "表达与组织" },
  ],
  overlap: 1,
  revealed: true,
};

const predictionEmpty: PredictionFx = { predicted: [], actual: [], overlap: 0, revealed: false };

const reflection: ReflectionFx = {
  text: "",
  prompts: ["整个研究过程里，你觉得最费劲的一步是什么？", "如果现在从头再来一次，你会先做哪件不一样的事？"],
};

// --- Task 9: AI 使用申报单 fixtures. Values verbatim dc.html:2179-2181 —
// 23/7/4/2/0.
const declarationUnsigned: DeclarationFx = {
  asks: 23,
  dispositions: 7,
  cardsSpontaneous: 4,
  cardsPrompted: 2,
  aiWrittenProse: 0,
  signed: false,
};

const declarationSigned: DeclarationFx = { ...declarationUnsigned, signed: true };

const noop = () => {};

const baseTerminalProps = {
  canFinish: false,
  finished: false,
  finishing: false,
  finishError: null,
  onFinish: noop,
  selfScore,
  prediction: predictionUnrevealed,
  reflection,
  onSelfScore: noop,
  onReflection: noop,
  declaration: declarationUnsigned,
  onSignDeclaration: noop,
};

test("renders code and name for each table", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  const card = screen.getByTestId("gauge-表D");
  expect(card).toHaveTextContent("表D");
  expect(card).toHaveTextContent("来源与证据");
});

test("renders the summary line from lamp sums", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  // Σlit = 3+0+3+3 = 9, Σtotal = 4+4+3+3 = 14
  expect(screen.getByText("已点亮 9/14 格")).toBeInTheDocument();
});

test("empty readiness shows 0/N and unlit cards", () => {
  const empty: GaugeFx[] = gauges.map((g) => ({ ...g, lit: 0, note: "", level: "empty" as const }));
  render(<ReviewView gauges={empty} {...baseTerminalProps} />);
  expect(screen.getByText("已点亮 0/14 格")).toBeInTheDocument();
});

test("canFinish renders the 完成任务 · 归档 button and fires onFinish on click", () => {
  const onFinish = vi.fn();
  render(<ReviewView gauges={gauges} {...baseTerminalProps} canFinish onFinish={onFinish} />);
  const button = screen.getByText("完成任务 · 归档");
  expect(button).toBeInTheDocument();
  fireEvent.click(button);
  expect(onFinish).toHaveBeenCalledTimes(1);
});

test("finished renders the inert 已归档 state and no button", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} finished />);
  expect(screen.getByText("已归档 · 成长报告已生成")).toBeInTheDocument();
  expect(screen.queryByText("完成任务 · 归档")).toBeNull();
});

test("neither canFinish nor finished renders no terminal UI", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  expect(screen.queryByText("完成任务 · 归档")).toBeNull();
  expect(screen.queryByText("已归档 · 成长报告已生成")).toBeNull();
});

test("shows finishError text when present", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} canFinish finishError="归档失败，请重试" />);
  expect(screen.getByText("归档失败，请重试")).toBeInTheDocument();
});

// --- Task 8: prediction reveal ---

test("prediction: shows predicted names and the pre-review line when not revealed", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} prediction={predictionUnrevealed} />);
  expect(screen.getByTestId("prediction-line-predicted")).toHaveTextContent("开头你预测最弱的是 分析、评估");
  expect(screen.getByTestId("prediction-line-pending")).toHaveTextContent("跑完整稿体检后，这里会对照实际");
  expect(screen.queryByTestId("prediction-line-actual")).toBeNull();
});

test("prediction: shows actual names and a descriptive overlap count once revealed", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} prediction={predictionRevealed} />);
  expect(screen.getByTestId("prediction-line-predicted")).toHaveTextContent("开头你预测最弱的是 分析、评估");
  expect(screen.getByTestId("prediction-line-actual")).toHaveTextContent("跑完这轮，评分表上实际最弱的是 评估、表达与组织");
  expect(screen.getByTestId("prediction-line-overlap")).toHaveTextContent("你的预测和实际吻合 1 项");
  expect(screen.queryByTestId("prediction-line-pending")).toBeNull();
});

test("prediction: shows the no-prediction line when predicted is empty", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} prediction={predictionEmpty} />);
  expect(screen.getByTestId("prediction-line-predicted")).toHaveTextContent("你在开头还没有预测最弱项");
});

// --- Task 8: self-score ---

test("self-score: renders one row per dim with a badge counting scored dims", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} selfScore={selfScore} />);
  expect(screen.getByTestId("self-score-badge")).toHaveTextContent("已评 2/4");
  for (const dim of selfScore.dims) {
    const row = screen.getByTestId(`self-score-row-${dim.code}`);
    expect(row).toHaveTextContent(dim.name);
    for (let band = 0; band < selfScore.bands.length; band += 1) {
      expect(screen.getByTestId(`self-score-chip-${dim.code}-${band}`)).toHaveTextContent(selfScore.bands[band]!);
    }
  }
  // the picked band is marked active; others aren't
  expect(screen.getByTestId("self-score-chip-表D-2")).toHaveAttribute("data-active", "true");
  expect(screen.getByTestId("self-score-chip-表D-0")).toHaveAttribute("data-active", "false");
});

test("self-score: clicking a chip calls onSelfScore with {scores:[{code,band}]}", () => {
  const onSelfScore = vi.fn();
  render(<ReviewView gauges={gauges} {...baseTerminalProps} selfScore={selfScore} onSelfScore={onSelfScore} />);
  fireEvent.click(screen.getByTestId("self-score-chip-表E-1"));
  expect(onSelfScore).toHaveBeenCalledWith({ scores: [{ code: "表E", band: 1 }] });
});

// --- Task 8: retro editor ---

test("retro: textarea hydrates from reflection.text and submit is disabled under 20 runes", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} reflection={{ text: "", prompts: [] }} />);
  const textarea = screen.getByTestId("reflection-textarea") as HTMLTextAreaElement;
  expect(textarea.value).toBe("");
  const submit = screen.getByTestId("reflection-submit");
  expect(submit).toBeDisabled();

  fireEvent.change(textarea, { target: { value: "太短了不到二十个字" } }); // < 20 runes
  expect(submit).toBeDisabled();
});

test("retro: hydrates from a non-empty reflection.text and enables submit at/over 20 runes", () => {
  const priorText = "这次研究里，最难的一步是判断这篇公众号文章到底能不能信，我花了很久去交叉核对。";
  const onReflection = vi.fn();
  render(
    <ReviewView
      gauges={gauges}
      {...baseTerminalProps}
      reflection={{ text: priorText, prompts: [] }}
      onReflection={onReflection}
    />,
  );
  const textarea = screen.getByTestId("reflection-textarea") as HTMLTextAreaElement;
  expect(textarea.value).toBe(priorText);
  const submit = screen.getByTestId("reflection-submit");
  expect(submit).toBeEnabled();

  fireEvent.click(submit);
  expect(onReflection).toHaveBeenCalledWith({ text: priorText });
});

test("retro: renders reflection.prompts as chips", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  for (const p of reflection.prompts) {
    expect(screen.getByText(p)).toBeInTheDocument();
  }
});

// --- Task 9: AI 使用申报单 ---

test("declaration: renders the four labels with their projected values", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} />);
  expect(screen.getByText("提问 / 追问")).toBeInTheDocument();
  expect(screen.getByText("23 次")).toBeInTheDocument();
  expect(screen.getByText("三键处置（接受/改/拒）")).toBeInTheDocument();
  expect(screen.getByText("7 次")).toBeInTheDocument();
  expect(screen.getByText("工具卡调用（自发/提示后）")).toBeInTheDocument();
  expect(screen.getByText("4 / 2")).toBeInTheDocument();
  expect(screen.getByText("AI 代写正文")).toBeInTheDocument();
  expect(screen.getByText("0 次")).toBeInTheDocument();
});

test("declaration: unsigned shows the 待你签名 pill and a sign control that calls onSignDeclaration", () => {
  const onSignDeclaration = vi.fn();
  render(<ReviewView gauges={gauges} {...baseTerminalProps} onSignDeclaration={onSignDeclaration} />);
  expect(screen.getByText("自动生成 · 待你签名")).toBeInTheDocument();
  const sign = screen.getByTestId("declaration-sign");
  fireEvent.click(sign);
  expect(onSignDeclaration).toHaveBeenCalledTimes(1);
});

test("declaration: signed renders 已签名 and replaces the sign control with the signed state", () => {
  render(<ReviewView gauges={gauges} {...baseTerminalProps} declaration={declarationSigned} />);
  expect(screen.getByText("已签名")).toBeInTheDocument();
  expect(screen.queryByText("自动生成 · 待你签名")).toBeNull();
  expect(screen.queryByTestId("declaration-sign")).toBeNull();
});
