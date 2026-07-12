import { render, screen, fireEvent } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { StudioAnnotateCard } from "./StudioAnnotateCard";

const anchors = [
  { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "currency", author: "ai" as const, question: "数据是哪一年的？", answer: "" },
  { id: "a1", material_id: "m1", block_id: "b0", start: 0, end: 0, quote: "", dimension: "authority", author: "ai" as const, question: "谁站在这条主张背后？", answer: "" },
];
const spec = { id: "craap", name: "信源辨识卡 CRAAP / CRRAAB", category: "信息素养", purpose: "x", primitive: "annotate" } as any;

test("renders one answerable row per anchor and locks with filled anchors + risk_note", () => {
  const onSubmit = vi.fn();
  render(<StudioAnnotateCard spec={spec} anchors={anchors} onSubmit={onSubmit} onSkip={() => {}} />);
  const boxes = screen.getAllByRole("textbox");
  // one per anchor + one risk-note field
  expect(boxes).toHaveLength(anchors.length + 1);

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[0]!, { target: { value: "数据是 2019 年的，偏旧" } });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[1]!, { target: { value: "只是博主，无机构背景" } });
  // all anchors answered but risk_note still empty — still disabled.
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[anchors.length]!, { target: { value: "支撑核心数据，但单一来源有风险" } });
  expect(lockButton).not.toBeDisabled();

  fireEvent.click(lockButton);
  expect(onSubmit).toHaveBeenCalledTimes(1);
  const env = onSubmit.mock.calls[0][0];
  expect(env.anchors.find((a: any) => a.dimension === "currency").answer).toBe("数据是 2019 年的，偏旧");
  const risk = env.anchors.find((a: any) => a.dimension === "risk_note");
  expect(risk).toBeTruthy();
  expect(risk.author).toBe("student");
  expect(risk.answer).toBe("支撑核心数据，但单一来源有风险");
});
