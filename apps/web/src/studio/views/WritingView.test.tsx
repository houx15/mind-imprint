import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { WritingView } from "./WritingView";
import type { WritingProjection, WritingReviewItem } from "../state";

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

  it("commits the current buffer via onCommit and switches to preview once the commit succeeds (spec §7)", async () => {
    const onCommit = vi.fn().mockResolvedValue(undefined);
    render(<WritingView {...baseProjection} onCommit={onCommit} />);
    fireEvent.click(screen.getByRole("button", { name: /提交快照/ }));
    expect(onCommit).toHaveBeenCalledWith(baseProjection.buffer);

    // The textarea (edit mode) disappears and preview's own not-yet-reviewed
    // placeholder appears — the view switched to 预览 · 批注 on its own,
    // without the student having to click the tab.
    expect(await screen.findByText(/还没做体检/)).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("does not switch to preview when onCommit rejects (a failed commit must not be shown as captured)", async () => {
    const onCommit = vi.fn().mockRejectedValue(new Error("commit blew up"));
    render(<WritingView {...baseProjection} onCommit={onCommit} />);
    fireEvent.click(screen.getByRole("button", { name: /提交快照/ }));
    expect(onCommit).toHaveBeenCalledWith(baseProjection.buffer);

    // Let the rejected promise settle, then confirm the view stayed on
    // 编辑 · 安静 — the textarea is still here, not the preview pane.
    await waitFor(() => expect(onCommit).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("textbox")).toBeInTheDocument();
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

  it("calls onOrderReview with the latest snapshot's id when 整稿体检 is clicked", () => {
    const onOrderReview = vi.fn();
    const projection: WritingProjection = {
      ...baseProjection,
      latestSnapshot: { id: "snap-9", seq: 1, committedAt: "2026-07-10T00:00:00Z", wordCount: 400, inBand: true },
    };
    render(<WritingView {...projection} onOrderReview={onOrderReview} />);
    fireEvent.click(screen.getByRole("button", { name: /整稿体检/ }));
    expect(onOrderReview).toHaveBeenCalledWith("snap-9");
  });

  describe("整稿体检 work order (Task 10)", () => {
    const reviewItem: WritingReviewItem = {
      interventionId: "i1",
      criterion: "表E 分析",
      band: "5–6 段",
      evidence: "第 2 段接住了反方，但「变绿→可持续」的跳步没补上。",
      missing: "「可持续」的定义还没写出来。",
      fix: "补上「可持续」的定义，把跳步写成推理",
      disposition: null,
    };
    const orderedProjection: WritingProjection = {
      ...baseProjection,
      review: { ordered: true, items: [reviewItem] },
    };

    function openPreview() {
      fireEvent.click(screen.getByText("预览 · 批注"));
    }

    it("renders the criterion, band chip, evidence, missing, and the three keys verbatim", () => {
      render(<WritingView {...orderedProjection} />);
      openPreview();
      expect(screen.getByText("整稿体检 · 段落 ⇄ 评分表")).toBeInTheDocument();
      expect(screen.getByText("一稿一检 · 只读")).toBeInTheDocument();
      expect(screen.getByText(/它只告诉你.*它从不替你改句子——改，是你自己的事/)).toBeInTheDocument();
      expect(screen.getByText("表E 分析")).toBeInTheDocument();
      expect(screen.getByText("5–6 段")).toBeInTheDocument();
      expect(screen.getByText(reviewItem.evidence)).toBeInTheDocument();
      expect(screen.getByText(`还缺：${reviewItem.missing}`)).toBeInTheDocument();
      expect(screen.getByText(`建议：${reviewItem.fix}`)).toBeInTheDocument();
      expect(screen.getByText("保持原样")).toBeInTheDocument();
      expect(screen.getByText("我来改")).toBeInTheDocument();
      expect(screen.getByText("说明为什么不改")).toBeInTheDocument();
    });

    it("does not render the fix/three-keys block when an item has no fix", () => {
      const noFixItem: WritingReviewItem = { ...reviewItem, interventionId: "i2", fix: "" };
      render(<WritingView {...baseProjection} review={{ ordered: true, items: [noFixItem] }} />);
      openPreview();
      expect(screen.queryByText("我来改")).not.toBeInTheDocument();
    });

    it("clicking 我来改 + a >=15-rune reason calls onReviewDisposition(iid, 'rewrite', reason)", () => {
      const onReviewDisposition = vi.fn();
      render(<WritingView {...orderedProjection} onReviewDisposition={onReviewDisposition} />);
      openPreview();

      fireEvent.click(screen.getByText("我来改"));
      const reason = "这一段的推理跳步确实需要我自己重新组织一下";
      fireEvent.change(screen.getByPlaceholderText(/写下你的理由/), { target: { value: reason } });
      fireEvent.click(screen.getByText("记录处置"));

      expect(onReviewDisposition).toHaveBeenCalledWith("i1", "rewrite", reason);
    });

    it("does not call onReviewDisposition when the reason is under 15 runes", () => {
      const onReviewDisposition = vi.fn();
      render(<WritingView {...orderedProjection} onReviewDisposition={onReviewDisposition} />);
      openPreview();

      fireEvent.click(screen.getByText("说明为什么不改"));
      fireEvent.change(screen.getByPlaceholderText(/写下你的理由/), { target: { value: "太短" } });
      fireEvent.click(screen.getByText("记录处置"));

      expect(onReviewDisposition).not.toHaveBeenCalled();
    });

    it("shows the persisted disposition action as the selected key", () => {
      const disposedItem: WritingReviewItem = {
        ...reviewItem,
        disposition: { action: "accept", reason: "这一段先保持原样，等我想清楚论证再改" },
      };
      render(<WritingView {...baseProjection} review={{ ordered: true, items: [disposedItem] }} />);
      openPreview();
      const keepKey = screen.getByText("保持原样");
      expect(keepKey).toHaveStyle({ background: "#2A3B7A", color: "#fff" });
    });

    it("the citations control calls onAttestCitations(true)", () => {
      const onAttestCitations = vi.fn();
      render(<WritingView {...orderedProjection} citationsMatched={false} onAttestCitations={onAttestCitations} />);
      openPreview();
      fireEvent.click(screen.getByRole("checkbox"));
      expect(onAttestCitations).toHaveBeenCalledWith(true);
    });
  });
});
