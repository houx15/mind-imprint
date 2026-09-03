import { describe, expect, it } from "vitest";
import { PROJECT_KINDS } from "./projects";
import { keepMetricsFor } from "./lookback";

// 🚨 「长期迭代」请她带回来的数据，必须是她这个项目量得出来的。
//
// 2026-09-02 线上实测走的是一个走廊上的项目——她要改的是课间十分钟，界面却
// 摆着「停留时长 / 点击率 / 留存率」三个写死的芯片。一个量不出来的指标不只是
// 没用：它在告诉她这一步跟她的项目无关，而这一步正是整个长期迭代的入口。
//
// 产品负责人 2026-09-03：「in-scene projects may need survey?」
describe("keepMetricsFor", () => {
  it("never asks a project happening in a corridor for a click-through rate", () => {
    for (const kind of ["田野调查", "活动策划", "investigation"]) {
      const names = keepMetricsFor(kind).map((m) => m.name);
      expect(names).not.toContain("点击率");
      expect(names).not.toContain("停留时长");
      expect(names).not.toContain("留存率");
    }
  });

  it("keeps the analytics three for things that live on a screen", () => {
    for (const kind of ["网站搭建", "产品原型", "website"]) {
      const names = keepMetricsFor(kind).map((m) => m.name);
      expect(names).toContain("点击率");
      expect(names).toContain("留存率");
    }
  });

  // 问卷、访谈、人数对任何项目都成立，所以每一类都该有它们——否则某一类项目
  // 会连一个能用的取数方式都拿不到。
  it("gives every project a way to gather data that always works", () => {
    for (const kind of [...PROJECT_KINDS, "", "  ", "investigation", "没见过的类别"]) {
      const names = keepMetricsFor(kind).map((m) => m.name);
      expect(names).toContain("问卷调查");
      expect(names).toContain("访谈记录");
      expect(names).toContain("使用人数");
    }
  });

  // 类别还没定（刚建的项目 kind 是空的）不该是错，也不该退回成一屏网站指标。
  it("falls back to the universal ones before she has picked a kind", () => {
    expect(keepMetricsFor("").map((m) => m.name)).not.toContain("点击率");
    expect(keepMetricsFor("").length).toBeGreaterThan(0);
  });

  // 每一个都要说得清它是什么——她是中学生，甩三个词等于没说。
  it("explains every metric it offers", () => {
    for (const kind of PROJECT_KINDS) {
      for (const m of keepMetricsFor(kind)) {
        expect(m.what.length).toBeGreaterThan(8);
      }
    }
  });

  it("never offers the same metric twice in one list", () => {
    for (const kind of PROJECT_KINDS) {
      const names = keepMetricsFor(kind).map((m) => m.name);
      expect(new Set(names).size).toBe(names.length);
    }
  });
});
