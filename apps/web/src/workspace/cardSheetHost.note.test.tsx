import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";
import { CardSheetHost } from "./CardSheetHost";

const spec = {
  id: "t", category: "信息素养", name: "测试卡", purpose: "p", trigger_condition: "tc", rubric_tags: [],
  steps: [{
    key: "s1", title: "第一步", disclose: "always",
    methodology: { why: "因为重要", how: "这样做", when: "卡住时" },
    fields: [{ type: "textarea", key: "x", label: "L" }],
  }],
} as unknown as CardSpec;

const instance: CardInstance = {
  id: "ci-1", card_id: "t", task_id: "task-1", parent_node_id: null, status: "active",
  field_values: {}, event_trace: [], rubric_tags: [], anchors: [], created_at: "2024-01-01T00:00:00.000Z", completed_at: null,
};

describe("CardSheetHost per-step note_open", () => {
  it("records note_open(step_key) when a step's 方法 panel is expanded", () => {
    const onSubmit = vi.fn();
    render(<CardSheetHost cardInstance={instance} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} onSkip={vi.fn()} />);
    fireEvent.click(screen.getByText("方法"));
    fireEvent.click(screen.getByText("提交并钉到过程树"));
    expect(onSubmit).toHaveBeenCalledOnce();
    const finalInstance = onSubmit.mock.calls[0]![1] as CardInstance;
    const noteEvents = finalInstance.event_trace.filter((e) => e.kind === "note_open");
    expect(noteEvents).toHaveLength(1);
    expect((noteEvents[0] as { step_key: string }).step_key).toBe("s1");
  });

  it("no longer shows the global '这个工具怎么用' button", () => {
    render(<CardSheetHost cardInstance={instance} spec={spec} onSubmit={vi.fn()} onClose={vi.fn()} onSkip={vi.fn()} />);
    expect(screen.queryByText("这个工具怎么用")).toBeNull();
  });
});
