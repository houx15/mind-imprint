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

// FIX 2 (whole-branch review CRITICAL): `anchors.every(...)` is vacuously
// true on an empty array — studioturn.go's surfaceAnchors degrades to []
// anchors on any generator failure (parse error / provider hiccup / empty
// generation), so a zero-anchor CRAAP card was lockable the instant the
// student typed only the risk note. The server can never actually complete
// such a submit (craap.json's every_tag_present needs 5 named tags that
// don't exist here), so locking it only ever produced the exact FIX 1
// scenario this whole fix wave exists to close: a stuck-active card with
// nothing to show for it. RED without the `hasAnchors` guard (revert it and
// this fails: the button enables immediately after the risk note alone).
test("refuses to lock a zero-anchor card even once the risk note is filled", () => {
  const onSubmit = vi.fn();
  render(<StudioAnnotateCard spec={spec} anchors={[]} onSubmit={onSubmit} onSkip={() => {}} />);
  // Zero per-anchor rows + the one risk-note field.
  const boxes = screen.getAllByRole("textbox");
  expect(boxes).toHaveLength(1);

  const lockButton = screen.getByRole("button", { name: /锁定|评估完成|完成/ });
  expect(lockButton).toBeDisabled();

  fireEvent.change(boxes[0]!, { target: { value: "支撑核心数据，但单一来源有风险" } });
  expect(lockButton).toBeDisabled();

  fireEvent.click(lockButton);
  expect(onSubmit).not.toHaveBeenCalled();
});
