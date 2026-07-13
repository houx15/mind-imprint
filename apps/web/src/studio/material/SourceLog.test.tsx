import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { SourceLog } from "./SourceLog";

// The visible ledger proving the student did the finding (RL-2) — every
// source they actually opened and logged shows up here; nothing is invented
// for a source that hasn't been logged yet.

const loggedSource: MaterialSource = {
  id: "src-nasa-nature-sustainability",
  title: "Chen et al. (2019), Nature Sustainability",
  sourceUrl: "https://doi.org/10.1038/s41893-019-0220-7",
  kind: "paper",
  origin: "fetched",
  blocks: [],
  locked: true,
  role: "第一手数据来源。",
  tier: "一手论文",
  takeaway: "卫星数据确认地球在变绿，但论文本身不涉及碳排放。",
  anchors: [],
};

describe("SourceLog", () => {
  it("renders the ledger", () => {
    render(<SourceLog sources={[loggedSource]} />);
    expect(screen.getByText("检索日志 · 已记录 1 条")).toBeInTheDocument();
    expect(screen.getByText(loggedSource.takeaway)).toBeInTheDocument();
  });

  it("omits sources with no log entry rather than inventing one", () => {
    render(<SourceLog sources={[{ ...loggedSource, takeaway: "", tier: "" }]} />);
    expect(screen.getByText("检索日志 · 已记录 0 条")).toBeInTheDocument();
    expect(screen.queryByText(loggedSource.title)).not.toBeInTheDocument();
  });

  it("links the title to sourceUrl when present, opening in a new tab safely", () => {
    render(<SourceLog sources={[loggedSource]} />);
    const link = screen.getByRole("link", { name: loggedSource.title });
    expect(link).toHaveAttribute("href", loggedSource.sourceUrl);
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("shows the tier chip", () => {
    render(<SourceLog sources={[loggedSource]} />);
    expect(screen.getByText(loggedSource.tier)).toBeInTheDocument();
  });

  it("shows the ledger's framing copy", () => {
    render(<SourceLog sources={[loggedSource]} />);
    expect(screen.getByText("每一条你打开过的来源都在这里——引用只能从这里来。")).toBeInTheDocument();
  });
});
