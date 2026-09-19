import { describe, expect, it } from "vitest";

import { scoreEnergy } from "./scenes/Cards";
import {
  ARCHIVE_EVIDENCE,
  ARCHIVE_OPTIONS,
  DECK_CARDS,
  ENERGY_DOMAINS,
  ENERGY_STAGES,
  GUIDES,
  TALENT_CARDS,
  TALENT_LANES,
  TALENT_PICK_COUNT,
} from "./content";

/**
 * 这个文件只测**读代码看不出对错**的那几条。
 *
 * 没有「标题渲染了 / 类名存在 / 列表有 N 项」那类断言 —— 它们写起来和维护起来
 * 都很贵，每次正常改版都会碎，而且抓不到真正会坏的东西
 * （AGENTS.md §测试只写逻辑测试；memory: test-logic-not-endless-frontend）。
 *
 * 这里每一条都对应一个**会静悄悄坏掉**的情况：一道没有正确答案的题会让她永远
 * 点不过去；一个和后端对不上的助手代号会让整趟作答存不进去。
 */

describe("档案确认题", () => {
  // 零个正确项 = 她永远点不过那一屏；两个 = 那道题没有意义。
  it("恰好有一个正确答案", () => {
    expect(ARCHIVE_OPTIONS.filter((o) => o.correct)).toHaveLength(1);
  });

  it("每个选项都给了一句回应，包括错的那几个", () => {
    for (const o of ARCHIVE_OPTIONS) {
      expect(o.reply.length, `选项 ${o.key} 没有回应`).toBeGreaterThan(0);
    }
  });

  it("三条证据的 id 互不相同", () => {
    const ids = ARCHIVE_EVIDENCE.map((e) => e.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe("AI 底牌", () => {
  // 一个选项没有回应，她点下去就什么都不会发生 —— 而那一屏看起来是活的。
  it("每张牌的每个选项都有一句回应", () => {
    for (const c of DECK_CARDS) {
      for (const o of c.experiment.options) {
        expect(
          c.experiment.reply[o.key],
          `牌 ${c.id} 的选项 ${o.key} 没有回应`,
        ).toBeTruthy();
      }
    }
  });

  it("三张牌的 id 互不相同，且都有正面文案", () => {
    const ids = DECK_CARDS.map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
    for (const c of DECK_CARDS) {
      expect(c.face.headline).toBeTruthy();
      expect(c.face.body).toBeTruthy();
    }
  });
});

describe("印记助手", () => {
  // 🚨 代号进库，而且 Go 侧按它选 prompt 语气段落和音频文件前缀。
  // 这里多一个或者改一个字，那一趟作答会被服务端以 invalid_navigator 拒掉。
  it("代号就是后端认的那三个", () => {
    expect(GUIDES.map((g) => g.id).sort()).toEqual(["KIRO", "NOVA", "SAGE"]);
  });
});

describe("能量卡牌", () => {
  it("每张卡指向的方向都在方向表里", () => {
    for (const stage of ENERGY_STAGES) {
      for (const c of stage.cards) {
        expect(ENERGY_DOMAINS[c.domain], `卡 ${c.id} 指向未知方向 ${c.domain}`).toBeTruthy();
      }
    }
  });

  it("卡的 id 全局唯一", () => {
    const ids = ENERGY_STAGES.flatMap((s) => s.cards.map((c) => c.id));
    expect(new Set(ids).size).toBe(ids.length);
  });

  // 一张都不选是合法的：这一屏没有必答要求。那时 focus 为空串，
  // 终端的第一问回到默认问法，而不是拼出一句「你的能量方向是 undefined」。
  it("一张都没选时给一个空方向，而不是一句坏掉的话", () => {
    const p = scoreEnergy({});
    expect(p.domains).toEqual([]);
    expect(p.focus).toBe("");
  });

  it("按选中次数排序，同分时顺序确定", () => {
    const p = scoreEnergy({
      approach: ["a-build", "a-story"],
      immerse: ["i-build"],
    });
    expect(p.domains?.[0]?.id).toBe("build"); // 两次
    expect(p.domains?.[1]?.id).toBe("story"); // 一次
    // 同一份输入算两次，结果必须一样 —— 否则两次调用的 prompt 不一样。
    expect(scoreEnergy({ approach: ["a-build", "a-story"], immerse: ["i-build"] })).toEqual(p);
  });

  it("忽略不存在的卡 id，而不是崩掉", () => {
    expect(() => scoreEnergy({ approach: ["不存在的卡"] })).not.toThrow();
    expect(scoreEnergy({ approach: ["不存在的卡"] }).domains).toEqual([]);
  });
});

describe("天赋卡牌", () => {
  // 要选 5 张而只有 4 张卡，她会卡在那一屏，按钮永远是灰的。
  it("卡的数量够选满", () => {
    expect(TALENT_CARDS.length).toBeGreaterThanOrEqual(TALENT_PICK_COUNT);
  });

  it("卡的 id 唯一", () => {
    const ids = TALENT_CARDS.map((c) => c.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  // 🚨 三堆的 key 进 jsonb，Go 侧 talentPiles 按这三个名字读。改一个，
  // 报告里「怎么靠近」那一块就整块空掉，而且不会报错。
  it("三堆的 key 就是后端读的那三个", () => {
    expect(TALENT_LANES.map((l) => l.key)).toEqual(["energy", "learned", "latent"]);
  });
});
