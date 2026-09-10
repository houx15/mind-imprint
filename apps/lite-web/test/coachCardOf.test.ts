import { describe, expect, it } from "vitest";
import { coachCardOf, type LiteMessage } from "@lite/api/readingRoom";

/**
 * 服务端发来的那张卡片，客户端认不认。
 *
 * 🚨 这个文件存在的理由是一个真实的、花了我大半天才找对地方的缺陷。
 *
 * 2026-09-10 给带读加了两块板（label_roles / word_bank）。服务端那一侧全做完
 * 并且**真的在发**：`atom_message.payload` 里逐字躺着一张完整的 label_roles，
 * 校验器有测试、prompt 写好了、渲染组件写好了。而 `coachCardOf` 里一行写死的
 * 白名单只认原来那三种，于是每一块板都在到达客户端的那一刻被静默丢掉。
 *
 * 表现出来是：印记 一遍遍说「现在给你一张卡片」，她屏幕上什么都没有。
 * 我为此在**服务端**追出并修好了四个「静默丢弃点」，而真正丢掉它的一直是这里。
 * 服务端日志因此干干净净 —— 什么都没被丢掉，是根本没被认出来。
 *
 * 所以这里逐个类型过一遍。加一种卡片形状而忘了改白名单，这条会红。
 */

function msg(card: unknown): LiteMessage {
  return {
    seq: 1,
    role: "ai",
    content: "现在给你一张卡片。",
    createdAt: "2026-09-10T00:00:00Z",
    payload: { card } as LiteMessage["payload"],
  };
}

const SPAN = { blockId: "b2", quote: "The agency said it had delivered 40 trucks." };

describe("coachCardOf 认得服务端会发的每一种卡片", () => {
  it("五种类型都认得，一种都不能少", () => {
    const cards: { type: string; extra: Record<string, unknown> }[] = [
      { type: "choose_span", extra: { options: [SPAN] } },
      { type: "pick_in_article", extra: {} },
      { type: "short_text", extra: {} },
      { type: "label_roles", extra: { options: [SPAN], labels: ["主张", "证据"] } },
      { type: "word_bank", extra: { words: [{ blockId: "b2", term: "delivered" }] } },
    ];
    for (const c of cards) {
      const got = coachCardOf(msg({ type: c.type, prompt: "问一句话？", ...c.extra }));
      expect(got, `${c.type} 被客户端丢掉了 —— 服务端发了，她却看不到`).not.toBeNull();
      expect(got!.type).toBe(c.type);
    }
  });

  it("标注板要把服务端填的格子原样带过来", () => {
    const got = coachCardOf(
      msg({ type: "label_roles", prompt: "这几句各自在干什么？", options: [SPAN], labels: ["主张", "证据", "限制"] }),
    );
    expect(got?.labels).toEqual(["主张", "证据", "限制"]);
    expect(got?.options).toHaveLength(1);
  });

  it("生词板要把词带过来", () => {
    const got = coachCardOf(
      msg({
        type: "word_bank",
        prompt: "这几个词你认识吗？",
        words: [{ blockId: "b1", term: "scrambling" }, { blockId: "b2", term: "delivered" }],
      }),
    );
    expect(got?.words?.map((w) => w.term)).toEqual(["scrambling", "delivered"]);
  });

  it("摆不了的板不算板 —— 半张卡片不许当成真卡片渲染", () => {
    // 这一条和 choose_span 的老规矩是同一个契约：渲染出来是一个没法回答的问题、
    // 也没有退路。板「空了」的判据不同而已：标注板没有句子，生词板没有词。
    expect(coachCardOf(msg({ type: "label_roles", prompt: "拖一拖", options: [] }))).toBeNull();
    expect(coachCardOf(msg({ type: "word_bank", prompt: "分一分", words: [] }))).toBeNull();
    expect(coachCardOf(msg({ type: "choose_span", prompt: "选一句", options: [] }))).toBeNull();
  });

  it("不认识的类型仍然丢掉", () => {
    expect(coachCardOf(msg({ type: "mind_map", prompt: "画一画" }))).toBeNull();
    expect(coachCardOf(msg({ prompt: "没有类型" }))).toBeNull();
    expect(coachCardOf(msg({ type: "short_text" }))).toBeNull(); // 没有问题
    expect(coachCardOf(msg(null))).toBeNull();
  });
});
