import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { OnboardingView } from "./OnboardingView";
import { STUDIO_FIXTURE } from "../fixtures";
import type { OnboardingFx, Station } from "../state";

const s0 = STUDIO_FIXTURE.stations.find((s) => s.code === "S0")!;

describe("OnboardingView (S0 任务解码)", () => {
  it("renders the restate card, rubric rows, and the plan tracker", () => {
    render(<OnboardingView station={s0} data={STUDIO_FIXTURE.views.onboarding} />);
    expect(screen.getByText(/说清这份任务在考什么/)).toBeInTheDocument();
    // the card-2 heading specifically (the restate subtitle also says 评分表)
    expect(screen.getByText("评分表 · 翻成人话")).toBeInTheDocument();
    const firstPlain = STUDIO_FIXTURE.views.onboarding.rubricRows[0]!.plain;
    expect(screen.getByText(firstPlain)).toBeInTheDocument();
  });

  it("clicking a rubric row toggles its 待加强 marker", () => {
    render(<OnboardingView station={s0} data={STUDIO_FIXTURE.views.onboarding} />);
    const firstPlain = STUDIO_FIXTURE.views.onboarding.rubricRows[0]!.plain;
    fireEvent.click(screen.getByText(firstPlain));
    expect(screen.getAllByText("待加强").length).toBeGreaterThan(0);
  });
});

// Task 8: persisting the restate + weak-picks, and hydrating both back from
// the projection on revisit. Uses a hand-built OnboardingFx (rather than
// STUDIO_FIXTURE, whose assignmentText/studentRestate are deliberately
// empty) so these assertions are independent of fixture data drift.
const station: Station = { code: "S0", name: "任务解码", view: "评估", state: "current" };

const priorRestate = "这道题在问中国这些年的环境政策，是不是真的让整个地球都变得更可持续了。";

function makeData(overrides: Partial<OnboardingFx> = {}): OnboardingFx {
  return {
    restatePrompt: "用你自己的话说说这道题在问什么。",
    rubricRows: [
      { official: "Analysis of different perspectives", plain: "能从不同视角分析，不只罗列观点", weak: true },
      { official: "Use & evaluation of evidence / sources", plain: "用可信来源，并说清它可不可信", weak: false },
    ],
    planSteps: ["立题", "找素材", "评估来源", "搭论证", "成稿", "反思归档"],
    assignmentText: "「中国在多大程度上让世界变得更具环境可持续性？」（0457 全球社会）",
    studentRestate: "",
    studentWeakPicks: [],
    ...overrides,
  };
}

describe("OnboardingView · S0 restate + weak-picks persistence", () => {
  it("renders the pasted assignment text above the restate card", () => {
    const data = makeData();
    render(<OnboardingView station={station} data={data} />);
    expect(screen.getByText(data.assignmentText)).toBeInTheDocument();
  });

  it("hydrates the restate textarea from data.studentRestate", () => {
    const data = makeData({ studentRestate: priorRestate });
    render(<OnboardingView station={station} data={data} />);
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    expect(textarea.value).toBe(priorRestate);
  });

  it("disables submit under 15 runes and enables it at/over 15", () => {
    const data = makeData({ studentRestate: "" });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OnboardingView station={station} data={data} onSubmit={onSubmit} />);
    const textarea = screen.getByRole("textbox") as HTMLTextAreaElement;
    const button = screen.getByRole("button", { name: "记下我的理解" });
    expect(button).toBeDisabled();

    fireEvent.change(textarea, { target: { value: "短于十五个字的复述" } }); // < 15 runes
    expect(button).toBeDisabled();

    fireEvent.change(textarea, { target: { value: priorRestate } }); // >= 15 runes
    expect(button).toBeEnabled();
  });

  it("calls onSubmit with the restate text and selected weak picks", async () => {
    const data = makeData({ studentRestate: priorRestate, studentWeakPicks: [0] });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<OnboardingView station={station} data={data} onSubmit={onSubmit} />);
    const button = screen.getByRole("button", { name: "记下我的理解" });
    expect(button).toBeEnabled();
    await act(async () => {
      fireEvent.click(button);
    });
    expect(onSubmit).toHaveBeenCalledWith({ restate: priorRestate, weakPicks: [0] });
  });
});
