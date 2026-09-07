import { describe, expect, it } from "vitest";
import type { InterestKeyword, InterestTree } from "../api/interest";
import {
  bornAtFor,
  growthStops,
  hashString,
  placeOnBranch,
  stopCount,
  toTreeKeywords,
} from "./liveTree";

// liveTree.test.ts —— 位置算法的守卫。
//
// 这是这一层唯一「读代码看不出对错」的东西：位置漂了，那张图就从「这是你的
// 模型」退化成装饰，而且没有任何断言会失败，只有人眼能看出来。所以这里测的是
// 不变量（确定性、不叠、有序），不是像素。

const DAY = 86_400_000;

function kw(over: Partial<InterestKeyword>): InterestKeyword {
  return {
    id: "k1",
    interestId: "i1",
    textZh: "词",
    textEn: "w",
    field: "formal",
    strength: 3,
    note: "",
    firstSeenAt: new Date(0).toISOString(),
    sources: [],
    disciplines: [],
    ...over,
  };
}

function tree(keywords: InterestKeyword[]): InterestTree {
  return { fields: [], keywords };
}

describe("hashString", () => {
  it("是确定的——同一个 id 每次都得到同一个数", () => {
    expect(hashString("a-uuid")).toBe(hashString("a-uuid"));
  });

  it("把相邻的 uuid 分开——否则同枝的词会挤在一起", () => {
    expect(hashString("00000000-0000-0000-0000-000000000001")).not.toBe(
      hashString("00000000-0000-0000-0000-000000000002"),
    );
  });
});

describe("placeOnBranch", () => {
  it("一个词时不贴树干也不吊在梢尖", () => {
    const [p] = placeOnBranch([{ id: "a" }]);
    expect(p!.t).toBeGreaterThan(0.3);
    expect(p!.t).toBeLessThan(0.9);
  });

  it("从里到外铺开，顺序就是入参顺序", () => {
    const got = placeOnBranch([{ id: "a" }, { id: "b" }, { id: "c" }, { id: "d" }]);
    const ts = got.map((p) => p.t);
    expect(ts).toEqual([...ts].sort((x, y) => x - y));
    expect(new Set(ts).size).toBe(4); // 没有两个落在同一处
  });

  it("左右交替——相邻两个词绝不挤在同一侧", () => {
    const got = placeOnBranch([{ id: "a" }, { id: "b" }, { id: "c" }]);
    expect(Math.sign(got[0]!.spread)).not.toBe(Math.sign(got[1]!.spread));
    expect(Math.sign(got[1]!.spread)).not.toBe(Math.sign(got[2]!.spread));
  });

  it("是确定的——同样的输入两次得到同样的位置", () => {
    const input = [{ id: "a" }, { id: "b" }, { id: "c" }];
    expect(placeOnBranch(input)).toEqual(placeOnBranch(input));
  });

  it("偏移幅度不会小到让标签压在枝上", () => {
    for (const p of placeOnBranch([{ id: "a" }, { id: "b" }, { id: "c" }, { id: "d" }])) {
      expect(Math.abs(p.spread)).toBeGreaterThan(30);
    }
  });
});

describe("bornAtFor", () => {
  const earliest = Date.parse("2026-01-01T00:00:00Z");
  const now = earliest + 300 * DAY;

  it("最早的那个词落在起点", () => {
    expect(bornAtFor(new Date(earliest).toISOString(), earliest, now)).toBe(0);
  });

  it("今天长出来的词落在现在", () => {
    expect(bornAtFor(new Date(now).toISOString(), earliest, now)).toBe(3);
  });

  it("中间的词按跨度分档，而不是按绝对月份", () => {
    const mid = new Date(earliest + 150 * DAY).toISOString();
    expect(bornAtFor(mid, earliest, now)).toBe(1);
  });

  // 🚨 一个昨天才开始的学生，不该被展示一段五个月的成长故事。
  it("跨度只有一天时，所有词都在起点——不把一天切成四段假精度", () => {
    const t0 = Date.parse("2026-09-01T09:00:00Z");
    const t1 = t0 + 3 * 3600_000;
    expect(bornAtFor(new Date(t0).toISOString(), t0, t1)).toBe(0);
  });

  it("时间戳坏掉时退到现在，而不是 NaN", () => {
    expect(bornAtFor("不是一个时间", earliest, now)).toBe(3);
  });
});

describe("toTreeKeywords", () => {
  it("空树给空数组，不抛", () => {
    expect(toTreeKeywords(tree([]))).toEqual([]);
  });

  it("老的靠树干，新的在梢头", () => {
    const t0 = Date.parse("2026-01-01T00:00:00Z");
    const got = toTreeKeywords(
      tree([
        kw({ id: "new", firstSeenAt: new Date(t0 + 200 * DAY).toISOString() }),
        kw({ id: "old", firstSeenAt: new Date(t0).toISOString() }),
      ]),
      t0 + 300 * DAY,
    );
    const old = got.find((k) => k.id === "old")!;
    const fresh = got.find((k) => k.id === "new")!;
    expect(old.at.t).toBeLessThan(fresh.at.t);
  });

  it("按枝分开铺——两根枝各自从里往外，互不影响", () => {
    const got = toTreeKeywords(
      tree([
        kw({ id: "f1", field: "formal" }),
        kw({ id: "f2", field: "formal" }),
        kw({ id: "a1", field: "arts" }),
      ]),
      Date.now(),
    );
    const arts = got.filter((k) => k.field === "arts");
    expect(arts).toHaveLength(1);
    // 独苗那根枝用的是「一个词」的位置，不是「第一个词」的位置。
    expect(arts[0]!.at.t).toBeGreaterThan(0.5);
  });

  // SQL 不保证行序稳定，所以同毫秒的两个词必须有一个确定的次序，
  // 否则这棵树会在两次刷新之间自己换个样子。
  it("同一时刻的两个词按 id 定序，和数组顺序无关", () => {
    const same = new Date("2026-05-05T00:00:00Z").toISOString();
    const a = toTreeKeywords(tree([kw({ id: "bbb", firstSeenAt: same }), kw({ id: "aaa", firstSeenAt: same })]));
    const b = toTreeKeywords(tree([kw({ id: "aaa", firstSeenAt: same }), kw({ id: "bbb", firstSeenAt: same })]));
    expect(a.map((k) => [k.id, k.at.t])).toEqual(b.map((k) => [k.id, k.at.t]));
  });

  it("强度被夹在 1..5——界面上的 STR n/5 不能撒谎", () => {
    const got = toTreeKeywords(tree([kw({ id: "a", strength: 99 }), kw({ id: "b", strength: 0 })]));
    expect(got.map((k) => k.strength).sort()).toEqual([1, 5]);
  });

  it("带上她的原话，因为抽屉里要显示它", () => {
    const got = toTreeKeywords(
      tree([
        kw({
          id: "a",
          sources: [
            {
              kind: "reading",
              refId: "r1",
              label: "一篇阅读",
              evidence: "我读到面积那一段才反应过来。",
              happenedAt: "2026-08-29T10:00:00Z",
            },
          ],
        }),
      ]),
    );
    expect(got[0]!.sources[0]!.evidence).toBe("我读到面积那一段才反应过来。");
    expect(got[0]!.sources[0]!.date).toBe("2026-08-29");
  });
});

describe("成长回放那条轴", () => {
  const now = Date.parse("2026-09-07T12:00:00Z");

  /** 一棵最早的词出现在 `daysAgo` 天前的树。 */
  function aged(daysAgo: number) {
    return toTreeKeywords(
      tree([kw({ firstSeenAt: new Date(now - daysAgo * DAY).toISOString() })]),
      now,
    );
  }

  // 🚨 这一条是这个 describe 存在的理由。上一版永远是四格，标签写死成
  // 「起点 · 半年前 · 近两个月 · 现在」—— 一个上周才开始的学生会看到一条写着
  // 「半年前」的轴，而那六个月不存在。截图上它看着完全正常。
  it("轴的长度跟着她的树活了多久走", () => {
    expect(stopCount(3 * DAY)).toBe(1);
    expect(stopCount(20 * DAY)).toBe(1);
    expect(stopCount(30 * DAY)).toBe(2);
    expect(stopCount(120 * DAY)).toBe(3);
    expect(stopCount(400 * DAY)).toBe(4);
  });

  it("刚开始的学生只有一格，界面据此不画这条轴", () => {
    expect(growthStops(aged(3), now)).toHaveLength(1);
    expect(growthStops(aged(3), now)[0]!.label).toBe("现在");
  });

  it("待久了轴就长出来，最多四格", () => {
    expect(growthStops(aged(30), now)).toHaveLength(2);
    expect(growthStops(aged(120), now)).toHaveLength(3);
    expect(growthStops(aged(400), now)).toHaveLength(4);
    expect(growthStops(aged(4000), now)).toHaveLength(4);
  });

  it("两头永远是起点和现在", () => {
    const stops = growthStops(aged(400), now);
    expect(stops[0]!.label).toBe("起点");
    expect(stops[stops.length - 1]!.label).toBe("现在");
  });

  // 中间那几格写的是**离今天多久**，由真实时间算出来。写死的「半年前」正是
  // 这次要改掉的东西，所以这里钉住：它必须随跨度变。
  it("中间那几格说的是真的时长，不是写死的词", () => {
    const half = growthStops(aged(400), now)[1]!.label;
    const longer = growthStops(aged(1200), now)[1]!.label;
    expect(half).toMatch(/前$/);
    expect(longer).toMatch(/前$/);
    expect(half).not.toBe(longer);
  });

  it("每一格都挂着它自己那一天，不是一句编出来的情节", () => {
    for (const st of growthStops(aged(400), now).slice(0, -1)) {
      expect(st.sub).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    }
  });

  it("一个词都没有时不画轴", () => {
    expect(growthStops([], now)).toHaveLength(1);
  });

  // bornAt 的上界必须跟着刻度数走：轴只有两格时，一个 bornAt=3 的词永远显示
  // 不出来 —— 树上少一个词，而没有任何地方会报错。
  it("每个词的刻度下标都在轴里面", () => {
    for (const days of [3, 30, 120, 400, 4000]) {
      const list = toTreeKeywords(
        tree(
          [0, 0.3, 0.6, 0.95, 1].map((r, i) =>
            kw({ id: `k${i}`, firstSeenAt: new Date(now - days * DAY * (1 - r)).toISOString() }),
          ),
        ),
        now,
      );
      const stops = growthStops(list, now);
      for (const k of list) {
        expect(k.bornAt, `${days} 天的树上「${k.text}」落在第 ${k.bornAt} 格`).toBeLessThan(
          stops.length,
        );
        expect(k.bornAt).toBeGreaterThanOrEqual(0);
      }
    }
  });
});
