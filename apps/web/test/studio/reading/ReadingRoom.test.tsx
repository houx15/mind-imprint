import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom } from "@/studio/reading/ReadingRoom";

const SOURCE: MaterialSource = {
  id: "m1", title: "NASA 气候报告", sourceUrl: "", kind: "article", origin: "nasa.gov",
  blocks: [{ id: "b0", text: "全球平均气温持续上升。" }, { id: "b1", text: "因此这项政策必然失败。" }],
  locked: false, role: "", tier: "", takeaway: "", anchors: [], timeSpentS: 0,
  lateralRead: false, isLateralInstrument: false, siftSkipped: false, lateralRelation: "", lateralJudgment: "",
};

// Task 10: ReadingRoom now owns the read-together loop internally
// (`useReadingLoop`), so it needs `projectId` + an `api` slice — this test
// never triggers the loop (no coach send, no card), so a bare object with
// no calls actually made is enough; readTurn is present only to satisfy the
// type, never invoked.
const NOOP_API = {
  readTurn: async function* () {},
  summonCard: async function* () {},
  activateProjectCard: async () => {},
  evaluateCardSelection: async () => {
    throw new Error("not used in this test");
  },
  submitProjectCard: async function* () {},
  skipProjectCard: async () => {},
  putReadingBrief: async () => {},
  getTakeawayDraft: async () => {
    throw new Error("not used in this test");
  },
  postFinalizeReading: async () => {
    throw new Error("not used in this test");
  },
};

describe("ReadingRoom", () => {
  it("renders the article and returns via back", () => {
    const onBack = vi.fn();
    render(<ReadingRoom projectId="p1" referenceId="r1" source={SOURCE} onBack={onBack} api={NOOP_API} />);
    // The title shows in both the topbar brand and the article header.
    expect(screen.getAllByText("NASA 气候报告").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/全球平均气温持续上升/)).toBeInTheDocument();
    // The chat starts with the coach greeting and the reading-deck starters.
    expect(screen.getByText(/这条来源可信吗/)).toBeInTheDocument();
    fireEvent.click(screen.getByText(/返回工作区/));
    expect(onBack).toHaveBeenCalled();
  });
});

// #4/#5: the reference's persisted bib (abstract/journal/author/year/url) shows
// in the article header — abstract as a collapsible 摘要 block, a metadata line,
// and a 打开原文 external link.
describe("ReadingRoom — bibliographic metadata header (#4/#5)", () => {
  it("renders the abstract, metadata line, and 打开原文 link from bib", () => {
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        bib={{
          author: "Chen, C. et al.",
          year: "2019",
          journal: "Nature Sustainability",
          abstract: "China's greening trend is real but its drivers are contested.",
          url: "https://doi.org/10.1038/s41893-019-0220-7",
        }}
        onBack={() => {}}
        api={NOOP_API}
      />,
    );
    // metadata line
    expect(screen.getByText("Chen, C. et al.")).toBeInTheDocument();
    expect(screen.getByText("Nature Sustainability")).toBeInTheDocument();
    // abstract (context) is present
    expect(screen.getByText(/greening trend is real/)).toBeInTheDocument();
    // 打开原文 external link points at the source url, opens in a new tab safely
    const link = screen.getByRole("link", { name: /打开原文/ });
    expect(link).toHaveAttribute("href", "https://doi.org/10.1038/s41893-019-0220-7");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows no abstract block or link when bib is absent", () => {
    render(<ReadingRoom projectId="p1" referenceId="r1" source={SOURCE} onBack={() => {}} api={NOOP_API} />);
    expect(screen.queryByText(/打开原文/)).toBeNull();
    expect(screen.queryByText("摘要")).toBeNull();
  });
});

// S2 (Task 9): brief-in banner — pre-filled from enter-reading's
// `suggestedReason` seed, editable, saves via putReadingBrief. Every save
// always sends all 3 ReadingBrief fields (reason/focus/phaseTag) — the
// endpoint is full-replace, so a partial body would wipe whichever it omits.
describe("ReadingRoom — brief banner (S2)", () => {
  it("seeds the reason from suggestedReason and saves all 3 fields on blur", () => {
    const putReadingBrief = vi.fn(async () => {});
    const api = { ...NOOP_API, putReadingBrief };
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        suggestedReason="带着「问题」读这篇，我想验证："
        onBack={() => {}}
        api={api}
      />,
    );

    // The seeded reason is shown up front — no click needed to see it.
    expect(screen.getByText(/你读这篇是为了：带着「问题」读这篇，我想验证：/)).toBeInTheDocument();

    // Click to edit, change the text, blur to save.
    fireEvent.click(screen.getByText(/你读这篇是为了：/));
    const input = screen.getByLabelText("你读这篇是为了");
    expect(input).toHaveValue("带着「问题」读这篇，我想验证：");
    fireEvent.change(input, { target: { value: "验证这篇能不能支持我的论点" } });
    fireEvent.blur(input);

    // All three ReadingBrief fields are always sent, even though only reason
    // was edited here — focus/phaseTag stay at their current (empty) values
    // rather than being omitted.
    expect(putReadingBrief).toHaveBeenCalledWith("p1", "r1", {
      readingReason: "验证这篇能不能支持我的论点",
      readingFocus: "",
      phaseTag: "",
    });
  });

  it("saves phaseTag immediately on select, alongside reason/focus", () => {
    const putReadingBrief = vi.fn(async () => {});
    const api = { ...NOOP_API, putReadingBrief };
    render(<ReadingRoom projectId="p1" referenceId="r1" source={SOURCE} onBack={() => {}} api={api} />);

    fireEvent.change(screen.getByLabelText("这篇材料用在哪个阶段"), { target: { value: "支持论点" } });

    expect(putReadingBrief).toHaveBeenCalledWith("p1", "r1", {
      readingReason: "",
      readingFocus: "",
      phaseTag: "支持论点",
    });
  });

  // REGRESSION (Task 9 fix): the banner used to seed briefPhase from "" and
  // briefReason from suggestedReason (never the persisted value), so editing
  // ONLY the reason on a reopened source resent phase_tag: "" — silently
  // wiping the already-saved phaseTag, because putReadingBrief is a
  // full-replace PUT. This must FAIL before ReadingRoom seeds briefReason/
  // briefPhase from the persisted readingReason/phaseTag props, and PASS
  // after.
  it("preserves the persisted phaseTag when only the reason is edited on a reopened source", () => {
    const putReadingBrief = vi.fn(async () => {});
    const api = { ...NOOP_API, putReadingBrief };
    render(
      <ReadingRoom
        projectId="p1"
        referenceId="r1"
        source={SOURCE}
        readingReason="验证碳排放反例"
        phaseTag="反例检验"
        onBack={() => {}}
        api={api}
      />,
    );

    // The persisted reason shows up front (not suggestedReason, which is
    // absent here) — reopening the source must not lose it either.
    expect(screen.getByText(/你读这篇是为了：验证碳排放反例/)).toBeInTheDocument();

    // Edit ONLY the reason.
    fireEvent.click(screen.getByText(/你读这篇是为了：/));
    const input = screen.getByLabelText("你读这篇是为了");
    fireEvent.change(input, { target: { value: "换一个更准确的说法" } });
    fireEvent.blur(input);

    // The save must carry the EDITED reason AND the PRESERVED phaseTag — not
    // "" for phaseTag, which would silently wipe the saved value.
    expect(putReadingBrief).toHaveBeenCalledWith("p1", "r1", {
      readingReason: "换一个更准确的说法",
      readingFocus: "",
      phaseTag: "反例检验",
    });
  });
});

// S2 (Task 9): 完成这篇 finalize panel — getTakeawayDraft seeds a read-only
// record + editable synthesis fields; confirming calls postFinalizeReading
// with her edited values (never the seed verbatim, never the record).
describe("ReadingRoom — 完成这篇 finalize panel (S2)", () => {
  it("fetches the draft, shows the read-only record + editable synthesis, and confirms with her edits", async () => {
    const getTakeawayDraft = vi.fn(async () => ({
      record: {
        findings: ["政策目标模糊。"],
        credibility: { verdict: "中等可信", why: "官方数据但样本有限。" },
        keyQuotes: [{ quote: "植被覆盖上升。", why: "支持结论。" }],
      },
      suggestedNewLeads: ["查一下具体地区数据"],
      suggestedProposalImpact: "这篇支持我的论点，但需要补充地区细节。",
    }));
    const postFinalizeReading = vi.fn(async () => ({}) as never);
    const api = { ...NOOP_API, getTakeawayDraft, postFinalizeReading };

    render(<ReadingRoom projectId="p1" referenceId="r1" source={SOURCE} onBack={() => {}} api={api} />);

    fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
    expect(getTakeawayDraft).toHaveBeenCalledWith("p1", "r1");

    // The assembled record renders read-only.
    await screen.findByText("政策目标模糊。");
    expect(screen.getByText(/中等可信/)).toBeInTheDocument();
    expect(screen.getByText(/植被覆盖上升/)).toBeInTheDocument();

    // The two synthesis fields are editable and seeded from the suggestions.
    const leadsField = screen.getByPlaceholderText(/这篇给你带来了什么新的线索/) as HTMLTextAreaElement;
    const impactField = screen.getByPlaceholderText(/这篇对你的论点有什么影响/) as HTMLTextAreaElement;
    expect(leadsField).toHaveValue("查一下具体地区数据");
    expect(impactField).toHaveValue("这篇支持我的论点，但需要补充地区细节。");

    fireEvent.change(leadsField, { target: { value: "查一下具体地区数据\n再找一篇反例" } });
    fireEvent.change(impactField, { target: { value: "编辑后的影响描述" } });

    fireEvent.click(screen.getByRole("button", { name: "确认归纳" }));

    await screen.findByText("已归纳 ✓");
    expect(postFinalizeReading).toHaveBeenCalledWith("p1", "r1", {
      newLeads: ["查一下具体地区数据", "再找一篇反例"],
      proposalImpact: "编辑后的影响描述",
    });
  });
});
