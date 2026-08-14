import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { DUALAXIS_MODEL, DepthDims } from "@mind-imprint/contracts";
import { MOCK_EVALUATION_REPORT as R } from "../EvaluationReport/__fixtures__/mock";
import { EvaluationReportPrint } from "./EvaluationReportPrint";
import { autonomyTier, depthTier } from "../EvaluationReport/tokens";

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
  it("shows position by semantic color + word only — no level number, no L1–L4 ladder", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("认知深度 D · 想得有多深")).toBeInTheDocument();
    // No "you-are-here" marker exists anywhere by construction.
    expect(container.querySelector("[data-current], [data-current-rung]")).toBeNull();
    const d0 = R.depth[0]!;
    const tier = depthTier(d0.level);
    // The accent bar carries the position via the SEMANTIC tier color.
    const chip = container.querySelector(`[data-color-chip="${d0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(hexToRgb(tier.bar));
    // The plain verdict word is shown; the L1–L4 anchor ladder is NOT (hidden).
    expect(screen.getAllByText(tier.word).length).toBeGreaterThan(0);
    const model = DepthDims().find((x) => x.id === d0.id)!;
    expect(screen.queryByText(model.anchors.L1)).toBeNull();
    expect(screen.queryByText(model.anchors.L4)).toBeNull();
    // The static definition (means) IS shown.
    expect(screen.getByText(new RegExp(model.means.slice(0, 8)))).toBeInTheDocument();
  });
});

describe("EvaluationReportPrint — 智识自主 A", () => {
  it("shows position by semantic color + word only — no band number, no 0–5 strip", () => {
    const { container } = render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("智识自主 A · 是否自己驱动认知")).toBeInTheDocument();
    const a0 = R.autonomy[0]!;
    const tier = autonomyTier(a0.band);
    const chip = container.querySelector(`[data-color-chip="${a0.id}"]`) as HTMLElement | null;
    expect(chip).not.toBeNull();
    expect(chip!.style.background).toContain(hexToRgb(tier.bar));
    expect(screen.getAllByText(tier.word).length).toBeGreaterThan(0);
    expect(container.querySelector("[data-current], [data-current-band]")).toBeNull();
  });
});

describe("EvaluationReportPrint — lens, tools, risks", () => {
  it("renders lens + tool sections", () => {
    render(<EvaluationReportPrint report={R} />);
    expect(screen.getByText("提问透镜")).toBeInTheDocument();
    expect(screen.getByText("工具卡与子代理")).toBeInTheDocument();
  });

  it("shows the positive statement when risks are empty", () => {
    render(<EvaluationReportPrint report={{ ...R, risks: [] }} />);
    expect(screen.getByText("本次评估未发现需要提示的风险行为。")).toBeInTheDocument();
  });

  it("shows a risk table when risks are present", () => {
    const withRisk = {
      ...R,
      risks: [{ type: "missing-source" as const, behaviour: "引用了未溯源的网页", suggestion: "补一手出处" }],
    };
    render(<EvaluationReportPrint report={withRisk} />);
    expect(screen.getByText("引用了未溯源的网页")).toBeInTheDocument();
  });
});
