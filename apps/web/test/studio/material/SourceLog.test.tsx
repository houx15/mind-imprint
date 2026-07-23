import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MaterialSource } from "@mind-imprint/contracts";
import { SourceLog } from "@/studio/material/SourceLog";

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
  timeSpentS: 610,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
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

  it("shows the accumulated reading time, rounded to the nearest minute", () => {
    render(<SourceLog sources={[loggedSource]} />);
    // 610s = 10.17min, rounds to 10.
    expect(screen.getByText("停留 10m")).toBeInTheDocument();
  });

  it("renders nothing for a source with 0 seconds logged, rather than 停留 0m", () => {
    render(<SourceLog sources={[{ ...loggedSource, timeSpentS: 0 }]} />);
    expect(screen.queryByText(/停留/)).not.toBeInTheDocument();
  });

  it("marks an entry 已横向核查 once its cross_check mint flips lateral_read", () => {
    render(<SourceLog sources={[{ ...loggedSource, lateralRead: true }]} />);
    expect(screen.getByText("已横向核查")).toBeInTheDocument();
  });

  it("does not mark an entry that hasn't been laterally read yet", () => {
    render(<SourceLog sources={[loggedSource]} />);
    expect(screen.queryByText("已横向核查")).not.toBeInTheDocument();
  });
});
