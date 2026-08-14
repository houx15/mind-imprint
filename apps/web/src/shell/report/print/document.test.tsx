import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { DUALAXIS_MODEL, DepthDims } from "@mind-imprint/contracts";
import { MOCK_EVALUATION_REPORT as R } from "../EvaluationReport/__fixtures__/mock";
import { EvaluationReportPrint } from "./EvaluationReportPrint";
import { AUTONOMY_RAMP, DEPTH_RAMP } from "../EvaluationReport/tokens";

afterEach(cleanup);

// jsdom normalizes an inline hex `background` to `rgb(r, g, b)` on read (not a
// lowercased hex string) — convert the ramp's hex to the same form to compare.
function hexToRgb(hex: string): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgb(${(n >> 16) & 255}, ${(n >> 8) & 255}, ${n & 255})`;
}

describe("EvaluationReportPrint — front matter", () => {
  it("renders the shared axiom and the 综述 counters", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText(DUALAXIS_MODEL.axiom)).toBeInTheDocument();
    expect(screen.getByText("关于这份报告")).toBeInTheDocument();
    expect(screen.getByText("综述 · 学生画像")).toBeInTheDocument();
    expect(screen.getByText(String(R.basics.counters.aiTurns))).toBeInTheDocument();
  });
});

describe("EvaluationReportPrint — timeline & materials", () => {
  it("renders section intros and material rows", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("过程时间线")).toBeInTheDocument();
    expect(screen.getByText("材料清单")).toBeInTheDocument();
    if (R.materials[0]) {
      expect(screen.getByText(new RegExp(R.materials[0].source))).toBeInTheDocument();
    }
  });
});

describe("EvaluationReportPrint — 认知深度 D", () => {
  it("shows all four L1–L4 anchors as reference, marks no current rung, prints no level number", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("认知深度 D · 想得有多深")).toBeInTheDocument();
    // No "you-are-here" marker exists anywhere by construction.
    expect(container.querySelector("[data-current], [data-current-rung]")).toBeNull();
    const d0 = R.depth[0]!;
    // Color chip carries the position, using the ramp for the (never-printed) level.
    const chip = container.querySelector(`[data-color-chip="${d0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(hexToRgb(DEPTH_RAMP[d0.level - 1]!));
    // All four rubric anchors for D1 render.
    const model = DepthDims().find((x) => x.id === d0.id)!;
    for (const rung of ["L1", "L2", "L3", "L4"] as const) {
      expect(screen.getByText(model.anchors[rung])).toBeInTheDocument();
    }
  });
});

describe("EvaluationReportPrint — 智识自主 A", () => {
  it("shows the band scale as reference, marks no cell, prints no band number", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("智识自主 A · 是否自己驱动认知")).toBeInTheDocument();
    const a0 = R.autonomy[0]!;
    const chip = container.querySelector(`[data-color-chip="${a0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(hexToRgb(AUTONOMY_RAMP[a0.band]!));
    expect(container.querySelector("[data-current], [data-current-band]")).toBeNull();
  });
});
