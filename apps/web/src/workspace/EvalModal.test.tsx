import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { EvalModal } from "./EvalModal";
import type { Evaluation } from "@mind-imprint/contracts";

const evaluation: Evaluation = {
  id: "ev1", task_id: "t1", status: "done", completed_at: null,
  created_at: "2026-06-29T00:00:00.000Z",
  narrative: "你这一程的思维印记叙述。",
  scores: [
    { dim_id: "D1", level: "L3", note: "开场即含背景+目标+约束" },
    { dim_id: "D4", level: "NA", note: "" },
  ],
};

describe("EvalModal (hierarchical reveal)", () => {
  it("renders the two faces with factual coverage chips, no aggregate level word", () => {
    render(<EvalModal evaluation={evaluation} onClose={() => {}} />);
    expect(screen.getByText("生成式驾驭")).toBeTruthy();
    expect(screen.getByText("批判式防护")).toBeTruthy();
    // driving face: D1 L3 scored; D4/D5/D7/D10 → among them D1 scored, D4 NA, others absent→NA
    // coverage chip is factual count text containing 已评 / 未涉及, never a SOLO word like 熟练 on the face row.
    expect(screen.getAllByText(/项已评/).length).toBeGreaterThan(0);
  });

  it("expands a category to reveal a scored dim (pill + note) and renders N/A neutrally", () => {
    render(<EvalModal evaluation={evaluation} onClose={() => {}} />);
    // Expand 意图与编排 (holds D1)
    fireEvent.click(screen.getByText("意图与编排"));
    expect(screen.getByText("提问清晰度")).toBeTruthy();
    expect(screen.getByText(/L3.*熟练/)).toBeTruthy();
    expect(screen.getByText("开场即含背景+目标+约束")).toBeTruthy();
    // Expand 推理与论证 (holds D4 NA + D5/D7 absent→NA) and assert neutral text
    fireEvent.click(screen.getByText("推理与论证"));
    expect(screen.getAllByText("本次未涉及").length).toBeGreaterThan(0);
  });

  it("renders the narrative and calls onClose from 回到任务", () => {
    let closed = false;
    render(<EvalModal evaluation={evaluation} onClose={() => { closed = true; }} />);
    expect(screen.getByText("你这一程的思维印记叙述。")).toBeTruthy();
    fireEvent.click(screen.getByText("回到任务"));
    expect(closed).toBe(true);
  });
});
