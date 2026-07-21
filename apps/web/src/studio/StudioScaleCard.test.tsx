import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioScaleCard } from "./StudioScaleCard";

// Minimal two-stop spec (mirrors certainty-spectrum.json's params shape,
// without importing the real vocabulary — this test's stops are made up,
// proving the host reads them from spec.params.buckets rather than
// hardcoding them, even though the Scale component's own prop is `stops`).
const spec = {
  id: "made-up-scale",
  name: "光谱卡",
  category: "知识工具",
  purpose: "把结论放到光谱上",
  primitive: "scale",
  params: {
    buckets: [
      { id: "低", label: "低确定度", hint: "不太确定" },
      { id: "高", label: "高确定度", hint: "很确定" },
    ],
    min_items: 1,
    item_prompt: "你的结论是什么？",
    reason_prompt: "为什么",
    rewrite_prompt: "重写这句话",
  },
} as any;

test("seeds working state from persisted anchors", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "第一个结论", dimension: "高", author: "student" as const, question: "", answer: "有强证据" },
  ];
  render(<StudioScaleCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />);
  expect((screen.getByPlaceholderText("你的结论是什么？") as HTMLTextAreaElement).value).toBe("第一个结论");
  expect((screen.getByPlaceholderText("为什么") as HTMLTextAreaElement).value).toBe("有强证据");
  // "高确定度" renders twice — once in the decorative axis header, once as
  // the item row's selected stop chip — so assert on the (on-styled)
  // clickable chip button rather than a bare text match.
  expect(screen.getByRole("button", { name: "高确定度" })).toBeInTheDocument();
});

test("re-seeds when cardInstanceId changes", () => {
  const anchors = [
    { id: "a0", material_id: "", block_id: "", start: 0, end: 0, quote: "旧的结论", dimension: "高", author: "student" as const, question: "", answer: "旧理由" },
  ];
  const { rerender } = render(
    <StudioScaleCard spec={spec} cardInstanceId="ci1" anchors={anchors} onSubmit={() => {}} onSkip={() => {}} />,
  );
  expect(screen.getByDisplayValue("旧的结论")).toBeInTheDocument();

  rerender(<StudioScaleCard spec={spec} cardInstanceId="ci2" anchors={[]} onSubmit={() => {}} onSkip={() => {}} />);
  expect(screen.queryByDisplayValue("旧的结论")).not.toBeInTheDocument();
});

test("locking submits an envelope whose card_id is the spec id and anchors round-trip the state", () => {
  const onSubmit = vi.fn();
  render(<StudioScaleCard spec={spec} cardInstanceId="ci1" onSubmit={onSubmit} onSkip={() => {}} />);

  fireEvent.click(screen.getByText(/添加一句/));
  fireEvent.change(screen.getByPlaceholderText("你的结论是什么？"), { target: { value: "新结论" } });
  fireEvent.click(screen.getByRole("button", { name: "高确定度" }));
  fireEvent.change(screen.getByPlaceholderText("为什么"), { target: { value: "因为理由" } });
  fireEvent.change(screen.getByPlaceholderText("重写这句话"), { target: { value: "重写后的句子" } });

  fireEvent.click(screen.getByRole("button", { name: /完成并钉到过程树/ }));

  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.card_id).toBe("made-up-scale");
  const itemAnchor = env.anchors.find((a: any) => a.quote === "新结论");
  expect(itemAnchor).toBeTruthy();
  expect(itemAnchor.dimension).toBe("高");
  expect(itemAnchor.answer).toBe("因为理由");
  expect(itemAnchor.author).toBe("student");
  const rewriteAnchor = env.anchors.find((a: any) => a.dimension === "rewrite");
  expect(rewriteAnchor).toBeTruthy();
  expect(rewriteAnchor.answer).toBe("重写后的句子");
});

test("skipping calls onSkip with the scaffold's event_trace", () => {
  const onSkip = vi.fn();
  render(<StudioScaleCard spec={spec} cardInstanceId="ci1" onSubmit={() => {}} onSkip={onSkip} />);
  fireEvent.click(screen.getByText(/跳过这张卡/));
  expect(onSkip).toHaveBeenCalledTimes(1);
  expect(Array.isArray(onSkip.mock.calls[0][0])).toBe(true);
});
