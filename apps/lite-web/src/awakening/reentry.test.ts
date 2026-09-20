import { describe, expect, it } from "vitest";

import { planReentry } from "./reentry";

describe("planReentry", () => {
  it("第一次进来的人不进入口那一屏", () => {
    expect(planReentry({ attemptNo: 1, stage: "boot", finishedAt: "" })).toEqual({
      hub: false,
      resume: "terminal",
    });
  });

  it("做到一半回来：进入口，「继续」回到她停下的那一屏", () => {
    expect(planReentry({ attemptNo: 1, stage: "lens", finishedAt: "" })).toEqual({
      hub: true,
      resume: "lens",
    });
  });

  it("做到一半回来，停在第一屏之后的剧情里，也算回来的人", () => {
    expect(planReentry({ attemptNo: 1, stage: "world", finishedAt: "" })).toEqual({
      hub: true,
      resume: "world",
    });
  });

  it("重做：新开的一趟停在 boot，但「继续」直接进探询，不重看剧情", () => {
    expect(planReentry({ attemptNo: 2, stage: "boot", finishedAt: "" })).toEqual({
      hub: true,
      resume: "terminal",
    });
  });

  it("已经走完的那一趟不进入口 —— 房间显示的是它的报告", () => {
    expect(
      planReentry({ attemptNo: 2, stage: "report", finishedAt: "2026-09-20T02:00:00Z" }),
    ).toEqual({ hub: false, resume: "report" });
  });
});
