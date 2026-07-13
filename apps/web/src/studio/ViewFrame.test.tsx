import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
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
