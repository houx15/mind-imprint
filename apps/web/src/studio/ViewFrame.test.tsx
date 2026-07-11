import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ViewFrame } from "./ViewFrame";
import { STUDIO_FIXTURE } from "./fixtures";

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
});
