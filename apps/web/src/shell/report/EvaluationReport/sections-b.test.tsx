import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AxisPanel } from "./AxisPanel";
import { Materials } from "./Materials";
import { MOCK_EVALUATION_REPORT } from "./__fixtures__/mock";

describe("AxisPanel (depth)", () => {
  it("renders all 6 dimension cards", () => {
    render(<AxisPanel axis="depth" dims={MOCK_EVALUATION_REPORT.depth} />);

    for (const id of ["D1", "D2", "D3", "D4", "D5", "D6"]) {
      expect(screen.getByTestId(`axis-dim-${id}`)).toBeInTheDocument();
    }
  });

  it("shows D1's quote and its boundary line", () => {
    render(<AxisPanel axis="depth" dims={MOCK_EVALUATION_REPORT.depth} />);

    const d1 = MOCK_EVALUATION_REPORT.depth.find((d) => d.id === "D1")!;
    const boundaryEvidence = d1.evidence.find((e) => e.boundary)!;

    expect(screen.getByText(d1.evidence[0]!.quote)).toBeInTheDocument();
    expect(screen.getByText(`边界：${boundaryEvidence.boundary}`)).toBeInTheDocument();
  });

  it("never renders the level as a standalone digit badge, only as the color bar", () => {
    const { container } = render(<AxisPanel axis="depth" dims={MOCK_EVALUATION_REPORT.depth} />);

    // D1's level = 3 -> DEPTH_RAMP[2]. No element should render the bare
    // digit "3" as its own text node (the level is color-only).
    const bareDigits = screen.queryAllByText("3", { exact: true });
    expect(bareDigits).toHaveLength(0);

    // The color IS present, carried by the bar element instead of a digit.
    const bar = container.querySelector('[data-dim-bar="D1"]') as HTMLElement | null;
    expect(bar).not.toBeNull();
    expect(bar!.style.background).toBeTruthy();
  });
});

describe("AxisPanel (autonomy)", () => {
  it("renders all 6 dimension cards", () => {
    render(<AxisPanel axis="autonomy" dims={MOCK_EVALUATION_REPORT.autonomy} />);

    for (const id of ["A1", "A2", "A3", "A4", "A5", "A6"]) {
      expect(screen.getByTestId(`axis-dim-${id}`)).toBeInTheDocument();
    }
  });
});

describe("Materials", () => {
  it("shows a cannotSupport string and a finalStatus chip", () => {
    render(<Materials materials={MOCK_EVALUATION_REPORT.materials} />);

    const first = MOCK_EVALUATION_REPORT.materials[0]!;
    expect(screen.getByText(first.cannotSupport)).toBeInTheDocument();
    expect(screen.getAllByText(first.finalStatus).length).toBeGreaterThan(0);
  });
});
