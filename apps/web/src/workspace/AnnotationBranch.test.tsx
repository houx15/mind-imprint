import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { CardInstance, CardSpec } from "@mind-imprint/contracts";
import { AnnotationBranch } from "./AnnotationBranch";

const spec = { id: "sift_craap", name: "CRAAP", category: "信息素养", mode: "annotation", steps: [], rubric_tags: [], purpose: "", trigger_condition: "" } as unknown as CardSpec;

function cardWith(anchors: CardInstance["anchors"]): CardInstance {
  return { id: "c1", card_id: "sift_craap", task_id: "t1", parent_node_id: null, status: "active", field_values: {}, event_trace: [], rubric_tags: [], anchors, created_at: "1", completed_at: null };
}

const aiAnchor = { id: "a0", material_id: "m1", block_id: "b0", start: 0, end: 5, quote: "美航局发现", dimension: "权威性 · Authority", author: "ai" as const, question: "这处「美航局发现」——转载者是权威吗？", answer: "" };

describe("AnnotationBranch", () => {
  it("renders the card name and each anchor's dimension + question", () => {
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText("CRAAP · 在真实材料上核查")).toBeInTheDocument();
    expect(screen.getByText("权威性 · Authority")).toBeInTheDocument();
    expect(screen.getByText("这处「美航局发现」——转载者是权威吗？")).toBeInTheDocument();
  });

  it("submits with the edited anchor answers and completed status", () => {
    const onSubmit = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} />);
    fireEvent.change(screen.getByPlaceholderText(/写下你的判断/), { target: { value: "只是转载，存疑" } });
    fireEvent.click(screen.getByText("提交并钉到过程树"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
    const [, final] = onSubmit.mock.calls[0];
    expect(final.status).toBe("completed");
    expect(final.anchors[0].answer).toBe("只是转载，存疑");
  });

  it("adds a student self-question anchor", () => {
    const onSubmit = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={onSubmit} onClose={vi.fn()} />);
    fireEvent.click(screen.getByText(/自己向印记提问/));
    fireEvent.change(screen.getByPlaceholderText(/写下你自己的问题/), { target: { value: "这个数据是哪年的？" } });
    fireEvent.click(screen.getByText("提交并钉到过程树"));
    const [, final] = onSubmit.mock.calls[0];
    const student = final.anchors.find((a: any) => a.author === "student");
    expect(student?.question).toBe("这个数据是哪年的？");
  });

  it("close calls onClose", () => {
    const onClose = vi.fn();
    render(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={vi.fn()} onClose={onClose} />);
    fireEvent.click(screen.getByText("收起"));
    expect(onClose).toHaveBeenCalledWith("c1");
  });

  it("renders anchors that arrive asynchronously after mount", () => {
    const { rerender } = render(<AnnotationBranch card={cardWith([])} spec={spec} onSubmit={vi.fn()} onClose={vi.fn()} />);
    expect(screen.queryByText("这处「美航局发现」——转载者是权威吗？")).toBeNull();
    rerender(<AnnotationBranch card={cardWith([aiAnchor])} spec={spec} onSubmit={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText("这处「美航局发现」——转载者是权威吗？")).toBeInTheDocument();
  });
});
