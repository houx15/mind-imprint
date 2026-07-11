import { describe, it, expect } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { OnboardingView } from "./OnboardingView";
import { STUDIO_FIXTURE } from "../fixtures";

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
