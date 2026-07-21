import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioMatrixCard } from "./StudioMatrixCard";

// Minimal two-row / three-column spec (mirrors a perspective-matrix-shaped
// card's params, without importing the real 立场主张/依据/盲区 vocabulary —
// this test's columns are made up, proving the host reads them from
// spec.params rather than hardcoding them).
const spec = {
  id: "made-up-matrix",
  name: "多视角卡",
  category: "知识工具",
  purpose: "补齐这道题遗漏的视角",
  primitive: "matrix",
  params: {
    cols: [
      { id: "view", label: "看法", q: "这个人怎么看？" },
      { id: "reason", label: "理由", q: "为什么这么想？" },
      { id: "gap", label: "没想到", q: "他们会漏掉什么？" },
    ],
    min_items: 1,
    row_prompt: "这是谁的视角？",
  },
} as any;

test("seeds working state from persisted anchors", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "甲视角", dimension: "view", author: "student" as const, question: "", answer: "觉得可行" },
    { id: "a1", material_id: "", block_id: "", start: 0, end: 0, quote: "甲视角", dimension: "reason", author: "student" as const, question: "", answer: "引用了数据" },
    { id: "a2", material_id: "", block_id: "", start: 0, end: 0, quote: "甲视角", dimension: "gap", author: "student" as const, question: "", answer: "没考虑成本" },
  ];
  render(<StudioMatrixCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />);
  expect((screen.getByPlaceholderText("这是谁的视角？") as HTMLTextAreaElement).value).toBe("甲视角");
  expect((screen.getByPlaceholderText("这个人怎么看？") as HTMLTextAreaElement).value).toBe("觉得可行");
  expect((screen.getByPlaceholderText("为什么这么想？") as HTMLTextAreaElement).value).toBe("引用了数据");
  expect((screen.getByPlaceholderText("他们会漏掉什么？") as HTMLTextAreaElement).value).toBe("没考虑成本");
  expect(screen.getByText("看法")).toBeInTheDocument();
});

test("re-seeds when cardInstanceId changes", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "旧视角", dimension: "view", author: "student" as const, question: "", answer: "旧看法" },
  ];
  const { rerender } = render(
    <StudioMatrixCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByDisplayValue("旧视角")).toBeInTheDocument();

  rerender(<StudioMatrixCard spec={spec} cardInstanceId="ci2" anchors={[]} onSubmit={() => {}} onSkip={() => {}} />);
  expect(screen.queryByDisplayValue("旧视角")).not.toBeInTheDocument();
});

test("locking submits an envelope whose card_id is the spec id and anchors round-trip the state", () => {
  const onSubmit = vi.fn();
  render(<StudioMatrixCard spec={spec} cardInstanceId="ci1" onSubmit={onSubmit} onSkip={() => {}} />);

  fireEvent.click(screen.getByText(/添加一个视角/));
  fireEvent.change(screen.getByPlaceholderText("这是谁的视角？"), { target: { value: "丙视角" } });
  fireEvent.change(screen.getByPlaceholderText("这个人怎么看？"), { target: { value: "反对" } });
  fireEvent.change(screen.getByPlaceholderText("为什么这么想？"), { target: { value: "担心风险" } });
  fireEvent.change(screen.getByPlaceholderText("他们会漏掉什么？"), { target: { value: "没看到长期收益" } });

  fireEvent.click(screen.getByRole("button", { name: /完成并钉到过程树/ }));

  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.card_id).toBe("made-up-matrix");
  const viewAnchor = env.anchors.find((a: any) => a.dimension === "view");
  expect(viewAnchor).toBeTruthy();
  expect(viewAnchor.quote).toBe("丙视角");
  expect(viewAnchor.answer).toBe("反对");
  expect(viewAnchor.author).toBe("student");
  const reasonAnchor = env.anchors.find((a: any) => a.dimension === "reason");
  expect(reasonAnchor.answer).toBe("担心风险");
  const gapAnchor = env.anchors.find((a: any) => a.dimension === "gap");
  expect(gapAnchor.answer).toBe("没看到长期收益");
});

test("skipping calls onSkip with the scaffold's event_trace", () => {
  const onSkip = vi.fn();
  render(<StudioMatrixCard spec={spec} cardInstanceId="ci1" onSubmit={() => {}} onSkip={onSkip} />);
  fireEvent.click(screen.getByText(/跳过这张卡/));
  expect(onSkip).toHaveBeenCalledTimes(1);
  expect(Array.isArray(onSkip.mock.calls[0][0])).toBe(true);
});

test("an incomplete persisted row (label + one cell) rehydrates and leaves the lock disabled", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "乙视角", dimension: "view", author: "student" as const, question: "", answer: "部分认同" },
  ];
  render(<StudioMatrixCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />);
  expect((screen.getByPlaceholderText("这是谁的视角？") as HTMLTextAreaElement).value).toBe("乙视角");
  expect((screen.getByPlaceholderText("这个人怎么看？") as HTMLTextAreaElement).value).toBe("部分认同");
  expect((screen.getByPlaceholderText("为什么这么想？") as HTMLTextAreaElement).value).toBe("");
  expect((screen.getByPlaceholderText("他们会漏掉什么？") as HTMLTextAreaElement).value).toBe("");
  expect(screen.getByRole("button", { name: /完成并钉到过程树/ })).toBeDisabled();
});
