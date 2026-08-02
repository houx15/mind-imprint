import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { LogEntry } from "@mind-imprint/contracts";

// #5 (studio-batch5-followup2) — the activity log should read as a timeline
// of real progress. "打开来源…进入阅读室" (opened a source) and "新增计划任务"
// (routine board bookkeeping) are noise; a finished reading, a settled
// proposal, a generated plan, a settled outline, a finished review and the
// student's own manual notes are milestones. This is display-only curation:
// the component still receives the FULL unfiltered `log` array (mirroring
// what PlanBlock passes in from `getLog`) — only what's rendered is curated.
import { ActivityLogView, isMilestoneLogEntry } from "@/workspace/blocks/PlanBlock";

const entries: LogEntry[] = [
  { id: "l1", date: "07-02", text: "开题四问初次落定", source: "auto" },
  { id: "l2", date: "07-03", text: "打开来源《卫星图看中国变绿》进入阅读室", source: "auto" },
  { id: "l3", date: "07-05", text: "归纳了《Chen et al. (2019)》", source: "auto" },
  { id: "l4", date: "07-06", text: "新增计划任务：找一篇质疑视角", source: "auto" },
  { id: "l5", date: "07-08", text: "印记根据开题生成了项目计划", source: "auto" },
  { id: "l6", date: "07-10", text: "写作提纲初次落定", source: "auto" },
  { id: "l7", date: "07-12", text: "完成回顾", source: "auto" },
  { id: "l8", date: "07-09", text: "撞上反例：中国碳排放全球第一", source: "me" },
];

describe("isMilestoneLogEntry", () => {
  it("drops source-open and plan-task-add noise", () => {
    expect(isMilestoneLogEntry(entries[1]!)).toBe(false); // 打开来源
    expect(isMilestoneLogEntry(entries[3]!)).toBe(false); // 新增计划任务
  });

  it("keeps milestone auto entries", () => {
    expect(isMilestoneLogEntry(entries[0]!)).toBe(true); // 开题四问初次落定
    expect(isMilestoneLogEntry(entries[2]!)).toBe(true); // 归纳了《…》
    expect(isMilestoneLogEntry(entries[4]!)).toBe(true); // 印记根据开题生成了项目计划
    expect(isMilestoneLogEntry(entries[5]!)).toBe(true); // 写作提纲初次落定
    expect(isMilestoneLogEntry(entries[6]!)).toBe(true); // 完成回顾
  });

  it("always keeps the student's own notes regardless of text", () => {
    expect(isMilestoneLogEntry({ id: "x", date: "07-01", text: "打开来源随手写的笔记", source: "me" })).toBe(true);
  });
});

describe("ActivityLogView", () => {
  it("renders milestone entries and hides opening/viewing + plan-task noise", () => {
    render(<ActivityLogView log={entries} onAdd={vi.fn()} />);

    // Milestones show.
    expect(screen.getByText("开题四问初次落定")).toBeInTheDocument();
    expect(screen.getByText("归纳了《Chen et al. (2019)》")).toBeInTheDocument();
    expect(screen.getByText("印记根据开题生成了项目计划")).toBeInTheDocument();
    expect(screen.getByText("写作提纲初次落定")).toBeInTheDocument();
    expect(screen.getByText("完成回顾")).toBeInTheDocument();
    expect(screen.getByText("撞上反例：中国碳排放全球第一")).toBeInTheDocument();

    // Noise is hidden from the rendered timeline.
    expect(screen.queryByText("打开来源《卫星图看中国变绿》进入阅读室")).not.toBeInTheDocument();
    expect(screen.queryByText("新增计划任务：找一篇质疑视角")).not.toBeInTheDocument();
  });

  it("still shows the empty state copy when every entry is filtered out", () => {
    const onlyNoise: LogEntry[] = [
      { id: "n1", date: "07-01", text: "打开来源《X》进入阅读室", source: "auto" },
    ];
    render(<ActivityLogView log={onlyNoise} onAdd={vi.fn()} />);
    expect(screen.getByText("还没有记录")).toBeInTheDocument();
  });
});
