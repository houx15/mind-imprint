import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { RenderStepContent, CourseAsset } from "@mind-imprint/contracts";

vi.mock("@/api", async (orig) => {
  const real = await orig<typeof import("@/api")>();
  return {
    ...real,
    api: {
      ...real.api,
      resolveUrl: vi.fn().mockResolvedValue("https://cdn.example.com/resolved.png"),
    },
  };
});

import { SegmentTimeline, isInteractionCorrect, orderMatches } from "@/shell/courses/SegmentTimeline";

describe("isInteractionCorrect", () => {
  const singleChoice = {
    id: "q1",
    type: "single_choice" as const,
    prompt: "?",
    options: [{ id: "a", text: "A" }, { id: "b", text: "B" }],
    correct_answer: ["b"],
    explanation: "",
    remediation_questions: [],
  };
  const multipleChoice = {
    id: "q2",
    type: "multiple_choice" as const,
    prompt: "?",
    options: [{ id: "a", text: "A" }, { id: "b", text: "B" }, { id: "c", text: "C" }],
    correct_answer: ["a", "c"],
    explanation: "",
    remediation_questions: [],
  };

  it("single_choice: exact match is correct", () => {
    expect(isInteractionCorrect(singleChoice, ["b"])).toBe(true);
  });

  it("single_choice: wrong option is incorrect", () => {
    expect(isInteractionCorrect(singleChoice, ["a"])).toBe(false);
  });

  it("multiple_choice: same set regardless of order is correct", () => {
    expect(isInteractionCorrect(multipleChoice, ["c", "a"])).toBe(true);
  });

  it("multiple_choice: partial selection is incorrect", () => {
    expect(isInteractionCorrect(multipleChoice, ["a"])).toBe(false);
  });

  it("multiple_choice: extra selection is incorrect", () => {
    expect(isInteractionCorrect(multipleChoice, ["a", "b", "c"])).toBe(false);
  });
});

describe("orderMatches", () => {
  it("matches identical sequences", () => {
    expect(orderMatches(["b", "a", "c"], ["b", "a", "c"])).toBe(true);
  });

  it("rejects a different sequence of the same ids", () => {
    expect(orderMatches(["a", "b", "c"], ["b", "a", "c"])).toBe(false);
  });

  it("rejects mismatched lengths", () => {
    expect(orderMatches(["b", "a"], ["b", "a", "c"])).toBe(false);
  });
});

describe("SegmentTimeline", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  const singleChoiceContent: RenderStepContent = {
    title: "标题",
    subtitle: "副标题",
    segments: [
      { kind: "teaching", flow_block_id: "", text: "第一段教学文本，介绍背景。", asset_ids: [], items: [] },
      { kind: "teaching", flow_block_id: "quiz1", text: "第二段教学文本，引出一个判断问题。", asset_ids: [], items: [] },
    ],
    interactions: [
      {
        id: "quiz1",
        type: "single_choice",
        prompt: "以下哪个判断更合理？",
        options: [
          { id: "a", text: "只看标题就下结论" },
          { id: "b", text: "回到材料证据里确认" },
        ],
        correct_answer: ["b"],
        explanation: "关键是把判断建立在材料证据上。",
        remediation_questions: [],
      },
    ],
    board: [],
  };

  function renderTimeline(content: RenderStepContent, onQuizAnswer = vi.fn(), assetsById: Record<string, CourseAsset> = {}) {
    render(
      <SegmentTimeline content={content} assetsById={assetsById} stepId="step-1" onQuizAnswer={onQuizAnswer} />
    );
    return { onQuizAnswer };
  }

  it("shows the first teaching segment's text immediately", () => {
    renderTimeline(singleChoiceContent);
    expect(screen.getByText("第一段教学文本，介绍背景。")).toBeInTheDocument();
    // Second segment is not revealed yet.
    expect(screen.queryByText("第二段教学文本，引出一个判断问题。")).not.toBeInTheDocument();
  });

  it("reveals more content on click and shows a continue hint while more remains", async () => {
    renderTimeline(singleChoiceContent);
    expect(screen.getByText("点击页面继续")).toBeInTheDocument();
    const root = screen.getByTestId("segment-timeline");
    await userEvent.click(root);
    expect(screen.getByText("第二段教学文本，引出一个判断问题。")).toBeInTheDocument();
  });

  it("selecting the correct option and submitting shows the explanation and calls onQuizAnswer with correct:true", async () => {
    const { onQuizAnswer } = renderTimeline(singleChoiceContent);
    const root = screen.getByTestId("segment-timeline");
    await userEvent.click(root); // reveal segment 2
    await userEvent.click(root); // reveal interaction

    await userEvent.click(screen.getByText("回到材料证据里确认"));
    await userEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(screen.getByText("关键是把判断建立在材料证据上。")).toBeInTheDocument();
    expect(onQuizAnswer).toHaveBeenCalledWith({
      stepId: "step-1",
      interactionId: "quiz1",
      selected: ["b"],
      correct: true,
    });
  });

  it("selecting the wrong option and submitting calls onQuizAnswer with correct:false", async () => {
    const { onQuizAnswer } = renderTimeline(singleChoiceContent);
    const root = screen.getByTestId("segment-timeline");
    await userEvent.click(root);
    await userEvent.click(root);

    await userEvent.click(screen.getByText("只看标题就下结论"));
    await userEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(onQuizAnswer).toHaveBeenCalledWith({
      stepId: "step-1",
      interactionId: "quiz1",
      selected: ["a"],
      correct: false,
    });
  });

  const orderingContent: RenderStepContent = {
    title: "标题",
    subtitle: "",
    segments: [
      { kind: "teaching", flow_block_id: "order1", text: "请把下面的步骤排成正确顺序。", asset_ids: [], items: [] },
    ],
    interactions: [
      {
        id: "order1",
        type: "ordering",
        prompt: "排出正确的顺序：",
        options: [
          { id: "a", text: "第一步" },
          { id: "b", text: "第二步" },
          { id: "c", text: "第三步" },
        ],
        correct_answer: ["b", "a", "c"],
        explanation: "正确顺序是第二步、第一步、第三步。",
        remediation_questions: [],
      },
    ],
    board: [],
  };

  it("ordering: reordering via up/down buttons then submitting validates against correct_answer", async () => {
    const { onQuizAnswer } = renderTimeline(orderingContent);
    const root = screen.getByTestId("segment-timeline");
    await userEvent.click(root); // reveal the interaction

    // Initial order is the authored option order: a, b, c.
    // Move "第二步" (b, currently at index 1) up above "第一步" (a).
    await userEvent.click(screen.getAllByRole("button", { name: "上移" })[1]!);
    await userEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(screen.getByText("正确顺序是第二步、第一步、第三步。")).toBeInTheDocument();
    expect(onQuizAnswer).toHaveBeenCalledWith({
      stepId: "step-1",
      interactionId: "order1",
      selected: ["b", "a", "c"],
      correct: true,
    });
  });

  it("structure segment renders a 板书 card from items", () => {
    const content: RenderStepContent = {
      title: "标题",
      subtitle: "",
      segments: [
        {
          kind: "structure",
          flow_block_id: "",
          text: "",
          asset_ids: [],
          items: [
            { label: "证据", text: "NASA 与 Nature Sustainability 的原始数据。" },
            { label: "反例", text: "中国碳排放总量全球第一。" },
          ],
        },
      ],
      interactions: [],
      board: [],
    };
    renderTimeline(content);
    expect(screen.getByText("证据")).toBeInTheDocument();
    expect(screen.getByText("NASA 与 Nature Sustainability 的原始数据。")).toBeInTheDocument();
    expect(screen.getByText("反例")).toBeInTheDocument();
  });

  it("resolves and renders an image asset referenced by a teaching segment", async () => {
    const content: RenderStepContent = {
      title: "标题",
      subtitle: "",
      segments: [
        { kind: "teaching", flow_block_id: "", text: "看这张图。", asset_ids: ["img1"], items: [] },
      ],
      interactions: [],
      board: [],
    };
    const asset: CourseAsset = { id: "img1", type: "image", title: "示意图", src: "https://api.example.com/oss?objectKey=courses%2Fimg1.png", ossKey: "", note: "" };
    renderTimeline(content, vi.fn(), { img1: asset });
    const img = await screen.findByAltText("示意图");
    expect(img).toHaveAttribute("src", "https://cdn.example.com/resolved.png");
  });
});
