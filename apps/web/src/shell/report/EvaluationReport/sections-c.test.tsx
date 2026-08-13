import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PromptLens } from "./PromptLens";
import { Risks } from "./Risks";
import { ToolUsage } from "./ToolUsage";
import { MOCK_EVALUATION_REPORT } from "./__fixtures__/mock";
import { RISK_LABEL } from "./tokens";

describe("PromptLens", () => {
  it("shows a prompt's relatedDomains and never renders a good/bad grade", () => {
    render(<PromptLens promptLens={MOCK_EVALUATION_REPORT.promptLens} />);

    const first = MOCK_EVALUATION_REPORT.promptLens.prompts[0]!;
    expect(screen.getByText(first.relatedDomains.join(" · "))).toBeInTheDocument();

    for (const banned of ["good", "needs-improvement", "值得称赞", "可以更好"]) {
      expect(screen.queryByText(banned)).not.toBeInTheDocument();
    }
  });

  it("shows 需要留意 only for the prompt(s) flagged attention: true", () => {
    render(<PromptLens promptLens={MOCK_EVALUATION_REPORT.promptLens} />);

    const attentionPrompts = MOCK_EVALUATION_REPORT.promptLens.prompts.filter((p) => p.attention);
    const nonAttentionPrompts = MOCK_EVALUATION_REPORT.promptLens.prompts.filter((p) => !p.attention);
    expect(attentionPrompts.length).toBeGreaterThan(0);
    expect(nonAttentionPrompts.length).toBeGreaterThan(0);

    expect(screen.getAllByText("需要留意")).toHaveLength(attentionPrompts.length);
  });
});

describe("ToolUsage", () => {
  it("renders each tool's name, stage, purpose and summary", () => {
    render(<ToolUsage toolUsage={MOCK_EVALUATION_REPORT.toolUsage} />);

    const first = MOCK_EVALUATION_REPORT.toolUsage[0]!;
    expect(screen.getByText(first.name)).toBeInTheDocument();
    expect(screen.getByText(first.stage)).toBeInTheDocument();
    expect(screen.getByText(first.purpose)).toBeInTheDocument();
    expect(screen.getByText(first.summary)).toBeInTheDocument();
  });
});

describe("Risks", () => {
  it("shows a risk's 中文 RISK_LABEL and its 处理方式 suggestion", () => {
    render(<Risks risks={MOCK_EVALUATION_REPORT.risks} />);

    const first = MOCK_EVALUATION_REPORT.risks[0]!;
    expect(screen.getAllByText(RISK_LABEL[first.type]).length).toBeGreaterThan(0);
    expect(screen.getByText(first.behaviour)).toBeInTheDocument();
    expect(screen.getAllByText("处理方式：").length).toBeGreaterThan(0);
    expect(screen.getByText(first.suggestion)).toBeInTheDocument();
  });
});
