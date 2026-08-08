import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { SubagentHint } from "@/studio/ai/SubagentHint";

/**
 * SubagentHint (Task 7) — the single-line status for a HIDDEN subagent
 * (reading-list search, plan generation, compaction, question-relation
 * proposals): a spinner while it runs, a check once it's done. Never a chat.
 */
describe("SubagentHint", () => {
  it("renders its text", () => {
    render(<SubagentHint text="subagent 正在检索来源…" />);
    expect(screen.getByText("subagent 正在检索来源…")).toBeInTheDocument();
  });

  it("shows a loading (spinning) icon by default, not the done/check style", () => {
    const { container } = render(<SubagentHint text="subagent 正在梳理问题关系…" />);
    expect(container.querySelector(".animate-spin")).not.toBeNull();
    expect(container.querySelector(".text-mk-success")).toBeNull();
  });

  it("shows a done/check style (no spinner) when done", () => {
    const { container } = render(<SubagentHint text="subagent 已整理研究计划" done />);
    expect(screen.getByText("subagent 已整理研究计划")).toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).toBeNull();
    expect(container.querySelector(".text-mk-success")).not.toBeNull();
  });
});
