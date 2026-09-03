import { describe, expect, it } from "vitest";
import { PROJECT_KINDS } from "./projects";
import { keepDelta, keepLaps, keepMetricsFor, lookbackTodo, type LookbackPrompt } from "./lookback";

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

// 🚨 迭代的意思是重复。一张勾一次就完的清单不是迭代——圈数是这件工具真正要她
// 看见的东西，所以「走完一圈」的判定不能含糊。
describe("keepLaps", () => {
  const e = (stage: string) =>
    ({ id: stage + Math.random(), kind: "stat", body: "x", stage,
       sessionId: null, createdAt: "2026-09-01T00:00:00Z" }) as never;

  it("counts a lap each time she finishes 产品迭代", () => {
    expect(keepLaps([e("ship"), e("observe"), e("interpret"), e("change")])).toBe(1);
    expect(keepLaps([e("change"), e("ship"), e("change")])).toBe(2);
  });

  // 还没改过一次东西，就还没转完一圈——记了一堆数据不算迭代。
  it("is zero until she has actually changed something", () => {
    expect(keepLaps([e("ship"), e("observe"), e("interpret")])).toBe(0);
    expect(keepLaps([])).toBe(0);
  });
});

// 🚨 一个数字本身不说明任何事：「23 个人用了」是多还是少？只有和上一次比才有
// 意思。「数据驱动」落到一个中学生手里，实际内容就是这一件：看变化，不看数值。
describe("keepDelta", () => {
  const e = (value: number | null, prev: number | null) =>
    ({ id: "x", kind: "stat", body: "b", stage: "observe", metric: "使用人数",
       value, prev, unit: "人", expect: "", verdict: "",
       sessionId: null, createdAt: "2026-09-01T00:00:00Z" }) as never;

  it("gives the change, not the number", () => {
    expect(keepDelta(e(23, 11))).toBe(12);
    expect(keepDelta(e(8, 20))).toBe(-12);
  });

  // 🚨 第一次记某个指标时没有上一次可比。返回 null，界面照实说「首次记录」——
  // 拿 0 当上一次，会把第一次记录说成一次巨大的增长。
  it("is null on the first record instead of pretending prev was zero", () => {
    expect(keepDelta(e(23, null))).toBeNull();
    expect(keepDelta(e(null, 11))).toBeNull();
  });
});

// 🚨 「完成」在复盘问题还没生成出来的那两三分钟里是亮的。
//
// lookbackTodo 返回空串的含义是「齐了，可以收工」，而它对空列表也返回空串——
// 于是印记还在读项目、写问题的时候，她就能把一次一道题都没有的复盘交掉，
// 回灌带给印记的是 answered: 0。整件工具的最后一步，恰恰在它还没有内容的
// 时候敞着。线上实测那次生成花了 2 分 44 秒。
describe("lookbackTodo", () => {
  const q = (answer: string): LookbackPrompt => ({
    id: answer || "empty",
    section: "what",
    prompt: "当时你为什么这么定？",
    answer,
    evidence: "",
    stance: "",
    ordinal: 0,
  });

  it("还没生成出来时拦住完成", () => {
    expect(lookbackTodo([])).not.toBe("");
  });

  it("生成出来了、一条没写，还是要拦", () => {
    expect(lookbackTodo([q(""), q("")])).toBe("至少写下一条");
  });

  it("写了一条就算复盘过——不要求答满", () => {
    expect(lookbackTodo([q("我当时想岔了"), q("")])).toBe("");
  });
});
