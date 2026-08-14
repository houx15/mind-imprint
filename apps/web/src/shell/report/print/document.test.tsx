import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { DUALAXIS_MODEL } from "@mind-imprint/contracts";
import { MOCK_EVALUATION_REPORT as R } from "../EvaluationReport/__fixtures__/mock";
import { EvaluationReportPrint } from "./EvaluationReportPrint";

afterEach(cleanup);

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
