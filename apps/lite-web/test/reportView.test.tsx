import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ReportView } from "@lite/reports/ReportView";
import type { LiteReport } from "@lite/api/reports";

/**
 * ReportView — pinned against the product owner's own bar ("colorful,
 * clear, not verbose, but interesting and appealing … let people have 分享欲")
 * and against 铁律②, the design law this report could most easily violate by
 * accident: never a score, grade, rank, streak, or comparison.
 */

afterEach(cleanup);

function report(over: Partial<LiteReport> = {}): LiteReport {
  return {
    version: 1,
    kind: "reading",
    title: "中国是否让地球变得更可持续？",
    studentName: "Phoebe",
    finishedAt: "2026-08-20T09:30:00Z",
    stats: [],
    moments: [],
    keep: null,
    gains: [],
    lensNotes: [],
    ...over,
  };
}

describe("ReportView", () => {
  it("renders a thin report without breaking", () => {
    // one stat, no moments, no keep, no gains
    const thin = report({
      stats: [{ key: "duration", label: "阅读时长", value: 6, unit: "分钟" }],
    });

    expect(() => render(<ReportView report={thin} />)).not.toThrow();

    // the one stat is visible
    expect(screen.getByText("6")).toBeTruthy();
    expect(screen.getByText("阅读时长")).toBeTruthy();

    // sections with no data render nothing — no empty headings, no
    // placeholders, no stray section labels for absent content.
    expect(screen.queryByText("金句")).toBeNull();
    expect(screen.queryByText("这次的收获")).toBeNull();
    expect(screen.queryByText("我用透镜查到的")).toBeNull();
    expect(screen.queryByText(/暂无/)).toBeNull();

    // no heading in the tree is empty — a "composed" page never leaves a
    // section title dangling with nothing under it.
    const headings = screen.getAllByRole("heading");
    for (const h of headings) {
      expect(h.textContent?.trim()).not.toBe("");
    }
  });

  it("renders no stat tiles at all when every stat is 0 — a wall of zeros reads as broken, not as a record", () => {
    const allZero = report({
      stats: [
        { key: "focusMinutes", label: "专注时长", value: 0, unit: "分钟" },
        { key: "coachTurns", label: "和印记聊了", value: 0, unit: "轮" },
        { key: "notes", label: "笔记", value: 0, unit: "条" },
        { key: "steps", label: "读完", value: 0, unit: "步" },
      ],
    });

    const { container } = render(<ReportView report={allZero} />);

    expect(screen.queryByText("专注时长")).toBeNull();
    expect(screen.queryByText("和印记聊了")).toBeNull();
    expect(screen.queryByText("笔记")).toBeNull();
    expect(screen.queryByText("读完")).toBeNull();
    // no row, no wrapper, no gap left behind for an empty stats section.
    expect(container.querySelectorAll(".flex-wrap").length).toBe(0);
  });

  it("renders only the non-zero stats from a mixed set", () => {
    const mixed = report({
      stats: [
        { key: "focusMinutes", label: "专注时长", value: 0, unit: "分钟" },
        { key: "wordsRead", label: "阅读字数", value: 860, unit: "字" },
        { key: "notes", label: "笔记", value: 0, unit: "条" },
      ],
    });

    render(<ReportView report={mixed} />);

    expect(screen.queryByText("专注时长")).toBeNull();
    expect(screen.queryByText("笔记")).toBeNull();
    expect(screen.getByText("860")).toBeTruthy();
    expect(screen.getByText("阅读字数")).toBeTruthy();
  });

  it("shows each 金句 and where it came from", () => {
    const withMoments = report({
      moments: [
        { quote: "中国的碳排放量是全球第一，这个事实我没法绕过去。", where: "第 3 段" },
        { quote: "可持续不是一句口号，是每个决定里的取舍。", where: "结论" },
      ],
    });

    render(<ReportView report={withMoments} />);

    expect(screen.getByText("中国的碳排放量是全球第一，这个事实我没法绕过去。")).toBeTruthy();
    expect(screen.getByText("可持续不是一句口号，是每个决定里的取舍。")).toBeTruthy();
    expect(screen.getByText("来自：第 3 段")).toBeTruthy();
    expect(screen.getByText("来自：结论")).toBeTruthy();
  });

  it("shows her own 收获, attributed as hers", () => {
    const withKeep = report({
      keep: { label: "我的收获", text: "光看一个国家的排放总量，会漏掉发展阶段这个变量。" },
    });

    render(<ReportView report={withKeep} />);

    expect(screen.getByText("我的收获")).toBeTruthy();
    expect(screen.getByText(/光看一个国家的排放总量，会漏掉发展阶段这个变量。/)).toBeTruthy();
  });

  it("shows each 透镜 note with the lens name, HER picked sentence honestly labelled, and the 发现", () => {
    const withLensNotes = report({
      lensNotes: [
        { lens: "信源辨识卡 CRAAP / CRRAAB", quote: "这份报告由国家能源局发布。", finding: "这句话可以在官网核实来源。" },
        { lens: "让步段 · 以退为进", quote: "反对者认为发展阶段应该被纳入考量。", finding: "她选中了对方最强的一点。" },
      ],
    });

    render(<ReportView report={withLensNotes} />);

    expect(screen.getByText("我用透镜查到的")).toBeTruthy();
    expect(screen.getByText("信源辨识卡 CRAAP / CRRAAB")).toBeTruthy();
    expect(screen.getByText("让步段 · 以退为进")).toBeTruthy();
    expect(screen.getAllByText("我选的句子：").length).toBe(2);
    expect(screen.getByText(/这份报告由国家能源局发布。/)).toBeTruthy();
    expect(screen.getByText("这句话可以在官网核实来源。")).toBeTruthy();
    expect(screen.getByText(/反对者认为发展阶段应该被纳入考量。/)).toBeTruthy();
    expect(screen.getByText("她选中了对方最强的一点。")).toBeTruthy();
  });

  it("renders nothing for 我用透镜查到的 when lensNotes is empty — no heading, no empty state", () => {
    const noLensNotes = report({ lensNotes: [] });
    render(<ReportView report={noLensNotes} />);
    expect(screen.queryByText("我用透镜查到的")).toBeNull();
  });

  it("never dresses a lens note as a 金句 pull-quote — separate visual treatment, no shared blockquote", () => {
    const mixed = report({
      moments: [{ quote: "这是一句金句。", where: "第 1 段" }],
      lensNotes: [{ lens: "信源辨识卡 CRAAP / CRRAAB", quote: "这是她选的句子。", finding: "这是发现。" }],
    });

    const { container } = render(<ReportView report={mixed} />);

    // exactly one blockquote — the 金句's — the lens note is NOT one.
    const quotes = container.querySelectorAll("blockquote");
    expect(quotes.length).toBe(1);
    expect(quotes[0]?.textContent).toContain("这是一句金句。");
    // the lens note's own text is present, but outside any blockquote.
    for (const bq of quotes) {
      expect(bq.textContent).not.toContain("这是她选的句子。");
    }
  });

  it("renders a lens note with an empty finding cleanly — no orphaned label, no dangling colon", () => {
    // Mirrors a degraded evaluate call server-side: buildReadingLensNotes
    // drops the canned fallback finding but keeps her quote (atom_report.go).
    const degraded = report({
      lensNotes: [{ lens: "信源辨识卡 CRAAP / CRRAAB", quote: "她自己选的句子", finding: "" }],
    });

    const { container } = render(<ReportView report={degraded} />);

    expect(screen.getByText("我用透镜查到的")).toBeTruthy();
    expect(screen.getByText("信源辨识卡 CRAAP / CRRAAB")).toBeTruthy();
    expect(screen.getByText("我选的句子：")).toBeTruthy();
    expect(screen.getByText(/她自己选的句子/)).toBeTruthy();
    // No dangling label/paragraph left over for the absent finding: the
    // note's card renders exactly one <p> (the quote), not a second empty
    // one for the finding it doesn't have.
    const cards = container.querySelectorAll(".rounded-mk-lg.border.border-mk-border.bg-mk-surface");
    expect(cards.length).toBe(1);
    expect(cards[0]?.querySelectorAll("p").length).toBe(1);
  });

  it("renders 2-4 gain lines when present", () => {
    const withGains = report({
      gains: ["找到了两处可以追溯到原始数据的来源", "写出了一个带反例的论证段"],
    });

    render(<ReportView report={withGains} />);

    expect(screen.getByText("找到了两处可以追溯到原始数据的来源")).toBeTruthy();
    expect(screen.getByText("写出了一个带反例的论证段")).toBeTruthy();
  });

  it("never renders a score, grade, rank or comparison", () => {
    const full = report({
      stats: [
        { key: "duration", label: "阅读时长", value: 18, unit: "分钟" },
        { key: "words", label: "写下的字数", value: 420, unit: "字" },
        { key: "sources", label: "查证的来源", value: 3, unit: "个" },
      ],
      moments: [{ quote: "证据比立场更早出现在她的段落里。", where: "第 2 段" }],
      keep: { label: "我的收获", text: "溯源比我想的更花时间，但也更让人信服。" },
      gains: ["用 CRAAP 溯源到了一手数据", "写完了让步段"],
    });

    render(<ReportView report={full} />);

    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/分数|评分|等级|排名|超过|第\s*\d+\s*名/);
  });
});
