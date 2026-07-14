import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CARD_REGISTRY } from "@mind-imprint/contracts";
import type { MaterialSource } from "@mind-imprint/contracts";
import type { LiveCard } from "./CoachRail";
import { ViewFrame } from "./ViewFrame";
import { STUDIO_FIXTURE } from "./fixtures";

// Both sources' block b1 deliberately open with the identical quote so the
// per-material filter is actually exercised: if `SourceDossier`'s
// `material_id` filter were ever removed, the live anchor (targeting only
// blogSource) would incorrectly split nasaSource's block too, since both
// share block id "b1" and the same leading substring.
const blogSource: MaterialSource = {
  id: "src-blog-china-greening",
  title: "《卫星图看中国变绿》",
  sourceUrl: "https://mp.weixin.qq.com/s/china-greening-satellite",
  kind: "article",
  origin: "fetched",
  blocks: [{ id: "b1", text: "过去二十年里发生了一件事：地球正在变绿。" }],
  locked: false,
  role: "",
  tier: "",
  takeaway: "",
  timeSpentS: 0,
  lateralRead: false,
  isLateralInstrument: false,
  anchors: [],
};
const nasaSource: MaterialSource = {
  id: "src-nasa-nature-sustainability",
  title: "Chen et al. (2019), Nature Sustainability",
  sourceUrl: "https://doi.org/10.1038/s41893-019-0220-7",
  kind: "paper",
  origin: "fetched",
  blocks: [{ id: "b1", text: "过去二十年这句话在论文里也出现，但语境完全不同。" }],
  locked: true,
  role: "",
  tier: "",
  takeaway: "",
  timeSpentS: 0,
  lateralRead: false,
  isLateralInstrument: false,
  anchors: [],
};
const craapSpec = CARD_REGISTRY["craap"]!;
const liveCard: LiveCard = {
  cardInstanceId: "ci1",
  cardId: "craap",
  spec: craapSpec,
  status: "active",
  anchors: [
    {
      id: "a1",
      material_id: blogSource.id,
      block_id: "b1",
      start: 0,
      end: 5,
      quote: "过去二十年",
      dimension: "authority",
      author: "ai",
      question: "原始出处是谁？",
      answer: "",
    },
  ],
};

describe("ViewFrame (station rail = view switcher)", () => {
  it("S4 active renders the 结构 view with its role cards + the station header", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S4" }} />);
    expect(screen.getByText("论证构建")).toBeInTheDocument();
    expect(screen.getByText(STUDIO_FIXTURE.views.structure[0]!.role)).toBeInTheDocument();
  });
  it("switching activeStation to S3 renders the 素材 SourceDossier", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S3" }} />);
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();
  });
  it("S0 active renders the onboarding recognition", () => {
    render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: "S0" }} />);
    expect(screen.getByText(/说清这份任务在考什么/)).toBeInTheDocument();
  });
  it("S1/S2 render onboarding shells, NOT their four-view association", () => {
    // S1.view is 结构 and S2.view is 素材 in the fixture, but both are
    // onboarding stations — they must render the onboarding shell, not the
    // 结构/素材 views.
    for (const code of ["S1", "S2"] as const) {
      const { unmount } = render(<ViewFrame state={{ ...STUDIO_FIXTURE, activeStation: code }} />);
      expect(screen.getByText(/后续切片接入/)).toBeInTheDocument(); // OnboardingView S1/S2 shell copy
      expect(screen.queryByText(/信源档案/)).not.toBeInTheDocument();
      unmount();
    }
  });

  it("highlights the live card's anchors only in the material they target", () => {
    const state = {
      ...STUDIO_FIXTURE,
      activeStation: "S3" as const,
      views: { ...STUDIO_FIXTURE.views, material: [blogSource, nasaSource] },
    };
    render(<ViewFrame state={state} card={liveCard} />);
    fireEvent.click(screen.getByText(blogSource.title));
    // The anchor's exact quote is its own <mark> run only when the span
    // actually matched — Annotate leaves an unmatched block as one plain
    // run, so getByText's exact-match semantics only succeed here if the
    // live anchor was actually threaded down and split the block.
    expect(screen.getByText("过去二十年").tagName).toBe("MARK");
  });

  it("does not leak one material's anchors into another", () => {
    const state = {
      ...STUDIO_FIXTURE,
      activeStation: "S3" as const,
      views: { ...STUDIO_FIXTURE.views, material: [blogSource, nasaSource] },
    };
    render(<ViewFrame state={state} card={liveCard} />);
    fireEvent.click(screen.getByText(nasaSource.title));
    // nasaSource's block b1 shares the same leading substring + block id
    // "b1" as blogSource's — if the material_id filter were ever dropped,
    // this anchor (targeting only blogSource) would split nasaSource's
    // block too, and "过去二十年" would appear as its own element.
    expect(screen.queryByText("过去二十年")).not.toBeInTheDocument();
  });
});

describe("ViewFrame (Task 11 + fix-wave [3]/[5]): render off the card's primitive, never its id", () => {
  const siftSpec = CARD_REGISTRY["sift"]!;

  function stateWithMaterials() {
    return {
      ...STUDIO_FIXTURE,
      activeStation: "S3" as const,
      views: { ...STUDIO_FIXTURE.views, material: [blogSource, nasaSource] },
    };
  }

  // The REAL server payload shape (FIX-A): `materialId` set to the checked
  // material, anchors scoped to it only — the "find" (lateral) dimension is
  // dropped at surface time because no lateral source exists yet
  // (studioturn.go's dropDimension). A test that hand-builds a symmetric
  // two-material anchor set (the old fixture here) never would have caught
  // finding [3]/[5] — the server never sends that shape.
  function compareCard(overrides: Partial<LiveCard> = {}): LiveCard {
    return {
      cardInstanceId: "ci2",
      cardId: "sift",
      spec: siftSpec,
      status: "active",
      materialId: blogSource.id,
      anchors: [
        { id: "a-stop", material_id: blogSource.id, block_id: "", start: 0, end: 0, quote: "", dimension: "stop", author: "ai", question: "你的第一反应是什么？", answer: "" },
      ],
      ...overrides,
    };
  }

  it("renders Compare's two panes for an active compare-primitive card, not SourceDossier's source list", () => {
    render(<ViewFrame state={stateWithMaterials()} card={compareCard()} />);
    expect(screen.getAllByTestId("compare-pane").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("dossier-source-list")).not.toBeInTheDocument();
  });

  it("keeps rendering SourceDossier for an active annotate-primitive card (today's path, unchanged)", () => {
    render(<ViewFrame state={stateWithMaterials()} card={liveCard} />);
    expect(screen.getByText(/信源档案/)).toBeInTheDocument();
    expect(screen.queryByTestId("compare-pane")).not.toBeInTheDocument();
  });

  // Finding [3]: the left pane used to derive its material from anchor
  // content, which comes up empty whenever anchor generation degrades (a
  // real, documented failure mode — studioturn.go's surfaceAnchors "no
  // anchors rather than failing the whole turn"). It must show the checked
  // source's actual text purely from `card.materialId`.
  it("the left pane renders the checked source's article text, keyed by card.materialId — works even with zero anchors", () => {
    render(<ViewFrame state={stateWithMaterials()} card={compareCard({ anchors: [] })} />);
    expect(screen.getByText(blogSource.blocks[0]!.text)).toBeInTheDocument();
  });

  it("invites adding the lateral source when none has been picked yet, reusing 6b's onAdd", () => {
    render(<ViewFrame state={stateWithMaterials()} card={compareCard()} material={{ onAdd: async () => {} }} />);
    expect(screen.getByText("去找一个独立的来源")).toBeInTheDocument();
  });

  // Finding [3]: the student's lateral-source choice used to live only in
  // StudioCompareCard's private state — nothing else in the Studio could
  // see it. Lifted, it must reach this pane the moment she picks it, before
  // any anchor ever targets that material (which only happens after a full
  // submit — see buildCompareState's doc comment).
  it("the right pane fills with the LIFTED lateral pick the instant it is set, before any anchor exists for it", () => {
    render(<ViewFrame state={stateWithMaterials()} card={compareCard()} lateralMaterialId={nasaSource.id} />);
    expect(screen.getByText(nasaSource.blocks[0]!.text)).toBeInTheDocument();
    expect(screen.getAllByTestId("compare-pane")).toHaveLength(2);
    expect(screen.queryByText("去找一个独立的来源")).not.toBeInTheDocument();
  });

  // Finding [3]: Compare used to fully REPLACE the dossier — the source
  // list, 检索日志 ledger, and chip all vanished while a compare card was
  // active, so she could neither read the source under review's siblings
  // nor see her other sources. A one-click toggle keeps both reachable.
  it("keeps the full dossier (source list) reachable behind a link while a compare card is active, and returns to Compare", async () => {
    const user = userEvent.setup();
    render(<ViewFrame state={stateWithMaterials()} card={compareCard()} />);

    await user.click(screen.getByRole("button", { name: /查看信源档案/ }));
    const dossier = await screen.findByTestId("dossier-source-list");
    expect(within(dossier).getByText(blogSource.title)).toBeInTheDocument();
    expect(within(dossier).getByText(nasaSource.title)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /返回横向核查/ }));
    expect(await screen.findAllByTestId("compare-pane")).not.toHaveLength(0);
    expect(screen.queryByTestId("dossier-source-list")).not.toBeInTheDocument();
  });
});
