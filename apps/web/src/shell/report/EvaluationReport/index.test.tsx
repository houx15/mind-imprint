import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";
import { EvaluationReport } from "@mind-imprint/contracts";
import { MOCK_EVALUATION_REPORT } from "./__fixtures__/mock";
import { EvaluationReportView } from "./index";

// jsdom doesn't implement IntersectionObserver; Ruler.tsx guards its absence,
// but stub it here so the real observe()/disconnect() wiring exercises too.
class IntersectionObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).IntersectionObserver = IntersectionObserverStub;

describe("MOCK_EVALUATION_REPORT fixture", () => {
  it("parses against the EvaluationReport contract without throwing", () => {
    expect(() => EvaluationReport.parse(MOCK_EVALUATION_REPORT)).not.toThrow();
  });
});

describe("EvaluationReportView", () => {
  beforeEach(() => {
    render(<EvaluationReportView report={MOCK_EVALUATION_REPORT} />);
  });

  it("renders the report title", () => {
    expect(screen.getByText("中国是否让地球变得更可持续？")).toBeInTheDocument();
  });

  it("renders all 9 ruler labels", () => {
    const labels = ["基本信息", "综述", "过程时间线", "材料清单", "认知深度 D", "智识自主 A", "提问透镜", "工具卡与子代理", "风险提示"];
    for (const label of labels) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });

  it("shows a deep value in the header section (AI turns counter)", () => {
    expect(screen.getByText(/612 次 AI 对话轮次/)).toBeInTheDocument();
  });

  it("shows a deep value in the abstract section", () => {
    expect(screen.getByText(/共 9 条来源/)).toBeInTheDocument();
  });

  it("shows a deep value in the timeline section (event summary)", () => {
    expect(screen.getByText(/从微信公众号标题进入/)).toBeInTheDocument();
  });

  it("shows a deep value in the materials section (cannotSupport)", () => {
    expect(screen.getByText(/不能证明 NASA 原意、中国单独贡献或整体可持续。/)).toBeInTheDocument();
  });

  it("shows a deep value in the depth axis section (a D1 quote)", () => {
    expect(screen.getByText("公众号说的地球变绿是真的吗？中国到底是不是主角？")).toBeInTheDocument();
  });

  it("shows a deep value in the autonomy axis section (an A1 quote)", () => {
    expect(screen.getByText("我不是让 AI 给我 NASA 链接，而是自己用公众号里的线索搜。")).toBeInTheDocument();
  });

  it("shows a deep value in the prompt lens section", () => {
    expect(screen.getByText("你能不能帮我总结视频。")).toBeInTheDocument();
  });

  it("shows a deep value in the tool usage section", () => {
    expect(screen.getByText("SIFT 溯源")).toBeInTheDocument();
  });

  it("shows a deep value in the risks section", () => {
    expect(screen.getByText("MEE 因缺少具体 URL 和打开理由被移出正文。")).toBeInTheDocument();
  });
});
