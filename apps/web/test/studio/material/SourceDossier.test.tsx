import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import type { Anchor, MaterialSource } from "@mind-imprint/contracts";
import { SourceDossier } from "@/studio/material/SourceDossier";

// SourceLog (Task 8) legitimately re-renders a logged source's title in the
// ledger below the list, so once a source carries a tier/takeaway its title
// text is no longer unique on the page — open it by clicking inside the
// source-list card, not by a page-wide text match.
function openSourceByTitle(title: string) {
  fireEvent.click(within(screen.getByTestId("dossier-source-list")).getByText(title));
}

// The calibration scenario (AGENTS.md): Phoebe, "中国是否让地球变得更可持续？" —
// a self-media blog riffing on real NASA/Boston University satellite findings
// (Chen et al. 2019, Nature Sustainability, DOI 10.1038/s41893-019-0220-7),
// then the actual NASA/Nature Sustainability finding itself as the source she
// traces back to. Wire-shaped (MaterialSource), not the deleted fixture.

const BLOG_ID = "src-blog-china-greening";
const NASA_ID = "src-nasa-nature-sustainability";

const authorityAnchor: Anchor = {
  id: "span-blog-authority",
  material_id: BLOG_ID,
  block_id: "b1",
  start: 21,
  end: 33,
  quote: "根据 NASA 卫星数据",
  dimension: "权威性",
  author: "ai",
  question: "「根据 NASA 卫星数据」——这条往上追，原始出处是谁？能找到 NASA 或论文本身吗，还是只是这篇公众号自己转述的？",
  answer: "",
};

const purposeAnchor: Anchor = {
  id: "span-blog-purpose",
  material_id: BLOG_ID,
  block_id: "b3",
  start: 0,
  end: 63,
  quote: "很难不把这读成一个信号",
  dimension: "目的性",
  author: "ai",
  question: "作者把「变绿」直接等同于「环保政策奏效」「更可持续」，这个推论站得住吗？有没有被这篇文章悄悄绕开的对立事实（比如碳排放）？",
  answer: "",
};

const blogArticle: MaterialSource = {
  id: BLOG_ID,
  title: "《卫星图看中国变绿》",
  sourceUrl: "https://mp.weixin.qq.com/s/china-greening-satellite",
  kind: "article",
  origin: "fetched",
  blocks: [
    {
      id: "b1",
      text: "过去二十年里发生了一件几乎没人注意到的事：根据 NASA 卫星数据，地球比 2000 年整整绿了一圈，而这背后最大的推手，是中国。",
    },
    {
      id: "b2",
      text: "变化大到能从太空里看见。2000 到 2017 年间，NASA 的 MODIS 卫星记录到全球绿叶面积增加了 5%，相当于新增了一整片亚马逊雨林那么大的绿色；仅占全球陆地面积 9% 的中国和印度，就贡献了这其中三分之一以上的增量。",
    },
    {
      id: "b3",
      text: "很难不把这读成一个信号：那个曾经和雾霾、燃煤电厂划等号的国家，如今悄悄成了地球变绿背后最大的力量——中国的环保政策，正在起效。",
    },
  ],
  locked: false,
  role: "触发关注的入口——数据引用听着权威，但结论被作者悄悄放大了，需要横向核实。",
  tier: "二手转述",
  takeaway: "",
  timeSpentS: 240,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
  anchors: [authorityAnchor, purposeAnchor],
};

const nasaSummary: MaterialSource = {
  id: NASA_ID,
  title: "Chen et al. (2019), Nature Sustainability",
  sourceUrl: "https://doi.org/10.1038/s41893-019-0220-7",
  kind: "paper",
  origin: "fetched",
  blocks: [
    {
      id: "b1",
      text: "基于 NASA MODIS 卫星 2000–2017 年数据：全球绿叶面积净增 5%，中国、印度合计贡献全球净增量的三分之一以上；增量主要来自农业集约化耕作与大规模植树工程，而非森林自然恢复。",
    },
  ],
  locked: true,
  role: "第一手数据来源，证实了「卫星观测到变绿」这件事本身是真的，但没有说这等于「更可持续」——变绿主要来自农业集约化与植树造林，论文本身并未涉及碳排放。",
  tier: "一手论文",
  takeaway:
    "NASA 与 Nature Sustainability 指出：卫星数据确认地球在变绿，中国是最大贡献者之一，但主要机制是农业集约化与人工造林，不是整体生态系统改善——论文本身不支持「中国让地球更可持续」这个更大的结论，也没有讨论碳排放。",
  timeSpentS: 610,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
  anchors: [],
};

const SOURCES: MaterialSource[] = [blogArticle, nasaSummary];

// Task 8/11 (read-together redesign): SourceDossier is list-mode ONLY now —
// the in-place article view (open/close, span highlighting, annotate-
// prepare, the locate-into-article machinery) was retired in Task 11 once
// SIFT/CRAAP moved entirely into the reading room. Coverage for reading a
// source, its highlighted anchors, the select-mode locate flow, and
// reading-time logging now lives in ReadingRoom.test.tsx; this file only
// covers the source-LIST surface (chips, 添加信源, 检索日志).
describe("SourceDossier", () => {
  it("lists sources with locked count and opens the ReadingRoom on a row click", () => {
    const onOpenReading = vi.fn();
    render(<SourceDossier sources={SOURCES} onOpenReading={onOpenReading} />);

    expect(screen.getByText(/信源档案/)).toBeInTheDocument();
    expect(screen.getByText(/已收集 2 篇/)).toBeInTheDocument();
    expect(screen.getByText(/已锁定 1\/2/)).toBeInTheDocument();

    // Task 8 (read-together redesign): a source-list row click opens the
    // focused ReadingRoom surface — the container owns that navigation.
    // Task 11: this is now the ONLY thing a row click does; there is no
    // in-place fallback view any more.
    openSourceByTitle(blogArticle.title);
    expect(onOpenReading).toHaveBeenCalledWith(blogArticle.id);
    // The list stays on screen — SourceDossier itself never navigates.
    expect(screen.getByTestId("dossier-source-list")).toBeInTheDocument();
  });

  it("a row click is a no-op (never throws) when onOpenReading is not supplied", () => {
    render(<SourceDossier sources={SOURCES} />);
    expect(() => openSourceByTitle(blogArticle.title)).not.toThrow();
    expect(screen.getByTestId("dossier-source-list")).toBeInTheDocument();
  });

  it("does not render a credibility verdict chip — MaterialSource has no verdict field", () => {
    render(<SourceDossier sources={SOURCES} />);
    expect(screen.queryByText("可信")).not.toBeInTheDocument();
    expect(screen.queryByText("存疑")).not.toBeInTheDocument();
  });

  it("renders the design's state-derived chips, never an invented verdict", () => {
    render(<SourceDossier sources={[blogArticle, nasaSummary]} />);
    expect(screen.getByText("待评估")).toBeInTheDocument();
    expect(screen.getByText("✓ 已锁定")).toBeInTheDocument();
    expect(screen.queryByText("可信")).not.toBeInTheDocument();
    expect(screen.queryByText("存疑")).not.toBeInTheDocument();
  });

  it("shows the design's fallback when 作用与风险 is unwritten", () => {
    render(<SourceDossier sources={[{ ...blogArticle, role: "" }]} />);
    expect(screen.getByText(/尚未写「作用与风险」/)).toBeInTheDocument();
  });

  // The chip is a statement about the SOURCE's state (evaluated + no lateral
  // check yet), never about which card happens to be open on it. `locked`
  // only flips once a CRAAP evaluation actually finished (a real
  // `evaluated-as` edge minted server-side, Slice 6b) — so these five cases
  // pin down the full 2x2(+) that matters: locked×lateralRead, plus the two
  // exclusions the server's own summon rule applies.

  // NOTE: nasaSummary is deliberately excluded from these cases — its own
  // fixture state is locked:true/lateralRead:false, so under the (correct)
  // derivation it legitimately carries the chip itself. Scoping each render
  // to the one source under test keeps the assertion about that source's
  // state, not about fixture noise from an unrelated row.

  it("shows the 需横向阅读 chip once a source is locked (CRAAP finished) but not yet laterally read", () => {
    const lockedNoLateral: MaterialSource = { ...blogArticle, locked: true, lateralRead: false };
    // Deliberately no live anchors/cards at all — the chip must not depend
    // on any card being open.
    render(<SourceDossier sources={[lockedNoLateral]} />);

    const dossier = screen.getByTestId("dossier-source-list");
    expect(within(dossier).getByText(/需横向阅读/)).toBeInTheDocument();
  });

  // Regression test for the CRITICAL bug found in the Task 11 review: the
  // old derivation was "a live card has anchors on this material" — but
  // CRAAP's own anchors carry `material_id` too, and CRAAP auto-surfaces on
  // every freshly-added source, which starts `locked: false`. So the old
  // guard told the student "需横向阅读" while she was mid-way through an
  // ordinary, unrelated vertical CRAAP check. This must show nothing.
  it("does not show 需横向阅读 while CRAAP is merely open (source not yet locked)", () => {
    // blogArticle.locked is false — CRAAP is open on it, not finished.
    render(<SourceDossier sources={[blogArticle]} />);

    const dossier = screen.getByTestId("dossier-source-list");
    expect(within(dossier).queryByText(/需横向阅读/)).not.toBeInTheDocument();
  });

  it("does not show 需横向阅读 once a locked source has already been laterally read (debt paid)", () => {
    const paidBlog: MaterialSource = { ...blogArticle, locked: true, lateralRead: true };
    render(<SourceDossier sources={[paidBlog]} />);

    const dossier = screen.getByTestId("dossier-source-list");
    expect(within(dossier).queryByText(/需横向阅读/)).not.toBeInTheDocument();
  });

  // Fix-wave finding [4]: FIX-A's server-side summon rule excludes a lateral
  // instrument (a material some OTHER cross_check --cites--> ) from ever
  // being proposed for its own SIFT card — the treadmill guard. The chip
  // must match that rule exactly, or it promises a lateral-read workflow the
  // summon logic will never actually offer. `locked && !lateralRead` alone
  // is not enough once a source can also be `isLateralInstrument`.
  it("does not show 需横向阅读 on a lateral instrument, even though it is locked and not itself laterally read", () => {
    const laterallyCheckedInstrument: MaterialSource = {
      ...blogArticle,
      locked: true,
      lateralRead: false,
      isLateralInstrument: true,
    };
    render(<SourceDossier sources={[laterallyCheckedInstrument]} />);

    const dossier = screen.getByTestId("dossier-source-list");
    expect(within(dossier).queryByText(/需横向阅读/)).not.toBeInTheDocument();
  });

  // FIX 3 (whole-branch review): classifier.go's siftSurfaced suppression
  // treats a SKIPPED SIFT card_instance the same as any in-flight/terminal
  // one and never re-proposes SIFT on that material again — so once she has
  // skipped it, `locked && !lateralRead` alone would keep the chip claiming
  // "正在核对 · 需横向阅读" (an ACTIVE process) forever, demanding a workflow
  // the coach will never actually offer again. This is the RED this fix
  // wave's own review flagged as the "unclearable nag with no honest
  // resolution" defect class — revert the `!source.siftSkipped` clause in
  // SourceDossier.tsx's needsLateralRead and this fails (the chip renders).
  it("does not show 需横向阅读 once SIFT has been explicitly skipped on this source", () => {
    const skippedLateral: MaterialSource = {
      ...blogArticle,
      locked: true,
      lateralRead: false,
      siftSkipped: true,
    };
    render(<SourceDossier sources={[skippedLateral]} />);

    const dossier = screen.getByTestId("dossier-source-list");
    expect(within(dossier).queryByText(/需横向阅读/)).not.toBeInTheDocument();
  });

  it("marks a laterally-read entry 已横向核查 in the 检索日志 ledger", () => {
    const laterallyReadBlog: MaterialSource = { ...blogArticle, lateralRead: true };
    render(<SourceDossier sources={[laterallyReadBlog, nasaSummary]} />);

    expect(screen.getByText("已横向核查")).toBeInTheDocument();
  });

  it("does not render the 已横向核查 mark for an entry that hasn't been laterally read", () => {
    render(<SourceDossier sources={SOURCES} />);
    expect(screen.queryByText("已横向核查")).not.toBeInTheDocument();
  });

  it("does not render the 添加信源 form at all when no onAddSource handler is supplied", () => {
    // A control that cannot do anything (no handler to actually add a
    // source) must not be shown — no live-looking form that silently no-ops.
    render(<SourceDossier sources={SOURCES} />);

    expect(screen.queryByText("添加信源")).not.toBeInTheDocument();
    // The rest of the list still renders fine without it.
    expect(screen.getByText(/检索日志/)).toBeInTheDocument();
  });

  it("mounts 添加信源 and 检索日志 in list mode", () => {
    render(<SourceDossier sources={SOURCES} onAddSource={async () => {}} />);

    expect(screen.getByText("添加信源")).toBeInTheDocument();
    expect(screen.getByText(/检索日志/)).toBeInTheDocument();
  });
});
