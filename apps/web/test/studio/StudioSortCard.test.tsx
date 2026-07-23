import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioSortCard } from "@/studio/StudioSortCard";

// Minimal three-bucket spec (mirrors fact-opinion-value.json's params shape,
// without importing the real vocabulary — this test's buckets are made up,
// proving the host reads them from spec.params rather than hardcoding them).
const spec = {
  id: "made-up-sort",
  name: "分类卡",
  category: "知识工具",
  purpose: "把陈述分成三类",
  primitive: "sort",
  params: {
    buckets: [
      { id: "红", label: "红桶", hint: "红色的" },
      { id: "蓝", label: "蓝桶", hint: "蓝色的" },
    ],
    min_items: 1,
    item_prompt: "写一句话",
    reason_prompt: "为什么",
  },
} as any;

test("seeds working state from persisted anchors", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "第一句话", dimension: "红", author: "student" as const, question: "", answer: "因为它是红的" },
  ];
  render(<StudioSortCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />);
  expect((screen.getByPlaceholderText("写一句话") as HTMLTextAreaElement).value).toBe("第一句话");
  expect((screen.getByPlaceholderText("为什么") as HTMLTextAreaElement).value).toBe("因为它是红的");
  expect(screen.getByText("红桶")).toBeInTheDocument();
});

test("re-seeds when cardInstanceId changes", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "旧的一句话", dimension: "红", author: "student" as const, question: "", answer: "旧理由" },
  ];
  const { rerender } = render(
    <StudioSortCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByDisplayValue("旧的一句话")).toBeInTheDocument();

  rerender(<StudioSortCard spec={spec} cardInstanceId="ci2" anchors={[]} onSubmit={() => {}} onSkip={() => {}} />);
  expect(screen.queryByDisplayValue("旧的一句话")).not.toBeInTheDocument();
});

test("locking submits an envelope whose card_id is the spec id and anchors round-trip the state", () => {
  const onSubmit = vi.fn();
  render(<StudioSortCard spec={spec} cardInstanceId="ci1" onSubmit={onSubmit} onSkip={() => {}} />);

  fireEvent.click(screen.getByText(/添加一句/));
  fireEvent.change(screen.getByPlaceholderText("写一句话"), { target: { value: "新句子" } });
  fireEvent.click(screen.getByText("红桶"));
  fireEvent.change(screen.getByPlaceholderText("为什么"), { target: { value: "因为理由" } });

  fireEvent.click(screen.getByRole("button", { name: /完成并钉到过程树/ }));

  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.card_id).toBe("made-up-sort");
  const anchor = env.anchors.find((a: any) => a.quote === "新句子");
  expect(anchor).toBeTruthy();
  expect(anchor.dimension).toBe("红");
  expect(anchor.answer).toBe("因为理由");
  expect(anchor.author).toBe("student");
});

test("skipping calls onSkip with the scaffold's event_trace", () => {
  const onSkip = vi.fn();
  render(<StudioSortCard spec={spec} cardInstanceId="ci1" onSubmit={() => {}} onSkip={onSkip} />);
  fireEvent.click(screen.getByText(/跳过这张卡/));
  expect(onSkip).toHaveBeenCalledTimes(1);
  expect(Array.isArray(onSkip.mock.calls[0][0])).toBe(true);
});
