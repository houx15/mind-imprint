import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { WritingView } from "./WritingView";
import type { WritingProjection } from "../state";

const baseProjection: WritingProjection = {
  buffer:
    "这篇文章想讨论一个常见的说法：中国是否让地球更可持续。\n\n" +
    "卫星数据显示，2000 年以来地球明显变绿，其中中国的贡献最大。",
  latestSnapshot: null,
  wordBudget: { min: 300, max: 500 },
  citationsMatched: false,
  review: { ordered: false, items: [] },
};

describe("WritingView (写作/S5 live)", () => {
  it("renders the editable buffer and calls onBufferChange as the student types", () => {
    const onBufferChange = vi.fn();
    render(<WritingView {...baseProjection} onBufferChange={onBufferChange} />);
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.value).toBe(baseProjection.buffer);
    expect(textarea).not.toHaveAttribute("readonly");

    fireEvent.change(textarea, { target: { value: "新的一句话。" } });
    expect(onBufferChange).toHaveBeenCalledWith("新的一句话。");
    expect(screen.getByText(/想听意见，点「整稿体检」/)).toBeInTheDocument();
  });

  it("commits the current buffer via onCommit", () => {
    const onCommit = vi.fn();
    render(<WritingView {...baseProjection} onCommit={onCommit} />);
    fireEvent.click(screen.getByRole("button", { name: /提交快照/ }));
    expect(onCommit).toHaveBeenCalledWith(baseProjection.buffer);
  });

  it("shows the not-yet-committed snapshotMeta and a disabled 整稿体检 button when there is no snapshot yet", () => {
    render(<WritingView {...baseProjection} />);
    expect(screen.getByText("还没有提交过快照")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /整稿体检/ })).toBeDisabled();
  });

  it("shows the snapshot meta and an enabled 整稿体检 button once a snapshot exists", () => {
    const projection: WritingProjection = {
      ...baseProjection,
      latestSnapshot: { id: "s1", seq: 3, committedAt: "2026-07-10T00:00:00Z", wordCount: 420, inBand: true },
    };
    render(<WritingView {...projection} />);
    // committedAt "2026-07-10T00:00:00Z" → MM-DD "07-10", per the binding
    // design (docs/design/思维印记_工作区.dc.html:2334 — "第 3 版快照 · 10-14 提交 ·
    // 只读": version · MM-DD · 提交 · 只读).
    expect(screen.getByText("第 3 版快照 · 07-10 提交 · 只读")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /整稿体检/ })).not.toBeDisabled();
  });

  it("renders the buffer's paragraphs in preview mode", () => {
    render(<WritingView {...baseProjection} />);
    fireEvent.click(screen.getByText("预览 · 批注"));
    const [firstPara] = baseProjection.buffer.split("\n\n");
    expect(screen.getByText(firstPara!)).toBeInTheDocument();
  });

  it("shows the 还没做体检 placeholder in preview when review has not been ordered", () => {
    render(<WritingView {...baseProjection} />);
    fireEvent.click(screen.getByText("预览 · 批注"));
    expect(screen.getByText(/还没做体检/)).toBeInTheDocument();
  });

  it("hides the 还没做体检 placeholder once review.ordered is true", () => {
    const projection: WritingProjection = { ...baseProjection, review: { ordered: true, items: [] } };
    render(<WritingView {...projection} />);
    fireEvent.click(screen.getByText("预览 · 批注"));
    expect(screen.queryByText(/还没做体检/)).not.toBeInTheDocument();
  });
});
